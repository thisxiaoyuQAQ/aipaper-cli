package search

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/references"
	domainsearch "github.com/thisxiaoyuQAQ/aipaper-cli/internal/search"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
	materialstui "github.com/thisxiaoyuQAQ/aipaper-cli/internal/tui/materials"
)

type Status string

const (
	StatusSearching  Status = "searching"
	StatusComplete   Status = "complete"
	StatusAllFailed  Status = "all_failed"
	StatusDisabled   Status = "disabled"
	StatusError      Status = "error"
)

type Action string

const (
	ActionNone     Action = ""
	ActionContinue Action = "continue"
	ActionRetry    Action = "retry"
	ActionSkip     Action = "skip"
	ActionBack     Action = "back"
	ActionCancel   Action = "cancel"
)

type SearchFunc func(ctx context.Context, s store.Store, opts domainsearch.Options) (domainsearch.Result, error)

type Options struct {
	WorkDir         string
	Store           store.Store
	Requirements    contracts.Requirements
	MaterialsResult materialstui.ScanResult
	Search          SearchFunc
}

type Model struct {
	workDir         string
	store           store.Store
	requirements    contracts.Requirements
	materialsResult materialstui.ScanResult
	search          SearchFunc

	status         Status
	searchErrors   []domainsearch.ProviderError
	searchOutputs  []string
	finalCandidates []contracts.ReferenceCandidate
	materialCount  int
	searchCount    int

	action Action
	err    error
}

type SearchFinishedMsg struct {
	Result domainsearch.Result
	Err    error
}

func NewModel(opts Options) Model {
	workDir := opts.WorkDir
	if workDir == "" {
		workDir = "."
	}
	s := opts.Store
	if s.Root() == "" {
		s = store.New(workDir)
	}
	searchFn := opts.Search
	if searchFn == nil {
		searchFn = domainsearch.Run
	}
	return Model{
		workDir:         workDir,
		store:           s,
		requirements:    opts.Requirements,
		materialsResult: opts.MaterialsResult,
		search:          searchFn,
		status:          StatusSearching,
		materialCount:   len(opts.MaterialsResult.Candidates),
	}
}

func (m Model) Init() tea.Cmd {
	return m.searchCmd()
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SearchFinishedMsg:
		m.applySearchFinished(msg)
		return m, nil
	default:
		return m, nil
	}
}

func (m Model) UpdateKey(key string) (Model, tea.Cmd) {
	m.action = ActionNone
	key = normalizeKey(key)
	if key == "q" || key == "ctrl+c" || key == "esc" {
		m.action = ActionCancel
		return m, nil
	}
	if m.status == StatusSearching {
		return m, nil
	}
	switch key {
	case "b":
		m.action = ActionBack
	case "r":
		if m.status == StatusAllFailed || m.status == StatusError {
			m.startSearch()
			return m, m.searchCmd()
		}
	case "s":
		if m.status == StatusAllFailed {
			m.action = ActionSkip
		}
	case "enter":
		switch m.status {
		case StatusComplete, StatusDisabled:
			m.action = ActionContinue
		case StatusAllFailed, StatusError:
			m.startSearch()
			return m, m.searchCmd()
		}
	}
	return m, nil
}

func (m Model) Status() Status {
	return m.status
}

func (m Model) Action() Action {
	return m.action
}

func (m Model) Err() error {
	return m.err
}

func (m Model) SearchErrors() []domainsearch.ProviderError {
	return append([]domainsearch.ProviderError(nil), m.searchErrors...)
}

func (m Model) MaterialCount() int {
	return m.materialCount
}

func (m Model) SearchCount() int {
	return m.searchCount
}

func (m Model) FinalCount() int {
	return len(m.finalCandidates)
}

func (m Model) Result() []contracts.ReferenceCandidate {
	return append([]contracts.ReferenceCandidate(nil), m.finalCandidates...)
}

func (m *Model) startSearch() {
	m.status = StatusSearching
	m.searchErrors = nil
	m.searchOutputs = nil
	m.finalCandidates = nil
	m.searchCount = 0
	m.err = nil
}

func (m *Model) applySearchFinished(msg SearchFinishedMsg) {
	m.action = ActionNone
	if msg.Err != nil {
		m.status = StatusError
		m.err = msg.Err
		return
	}
	m.err = nil

	materialCandidates := m.materialsResult.Candidates
	searchCandidates := msg.Result.Candidates.Items
	m.searchErrors = append([]domainsearch.ProviderError(nil), msg.Result.Errors...)
	m.searchCount = len(searchCandidates)

	merged := append(append([]contracts.ReferenceCandidate(nil), materialCandidates...), searchCandidates...)
	deduped := references.DedupeCandidates(merged)
	m.finalCandidates = references.AssignCandidateIDs(deduped, 1)

	outputs, err := references.WriteCandidates(m.store, m.finalCandidates)
	if err != nil {
		m.status = StatusError
		m.err = fmt.Errorf("write candidates failed: %w", err)
		return
	}
	m.searchOutputs = outputs

	if !m.requirements.AllowOnlineSearch {
		m.status = StatusDisabled
		return
	}
	if len(searchCandidates) == 0 && len(m.searchErrors) > 0 {
		m.status = StatusAllFailed
		return
	}
	m.status = StatusComplete
}

func (m Model) searchCmd() tea.Cmd {
	s := m.store
	requirements := m.requirements
	searchFn := m.search
	return func() tea.Msg {
		ctx := context.Background()
		result, err := searchFn(ctx, s, domainsearch.Options{
			Requirements: requirements,
			Limit:        10,
		})
		return SearchFinishedMsg{Result: result, Err: err}
	}
}

func normalizeKey(key string) string {
	if key == " " {
		return "space"
	}
	return strings.ToLower(strings.TrimSpace(key))
}
