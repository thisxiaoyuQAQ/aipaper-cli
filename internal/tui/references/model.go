package referencestui

import (
	"sort"
	"strings"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/i18n"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/references"
)

type SortMode int

const (
	SortOriginal SortMode = iota
	SortRelevance
	SortYear
	SortTitle
)

type Options struct {
	I18N i18n.T
}

type Model struct {
	candidates []contracts.ReferenceCandidate
	cursor     int
	selected   map[string]bool
	rejected   map[string]bool
	filter     string
	searching  bool
	sortMode   SortMode
	done       bool
	canceled   bool
	err        error
	i18n       i18n.T
}

func NewModel(candidates contracts.ReferenceCandidates, opts ...Options) Model {
	tr := i18n.New("")
	if len(opts) > 0 && !opts[0].I18N.IsZero() {
		tr = opts[0].I18N
	}
	items := append([]contracts.ReferenceCandidate(nil), candidates.Items...)
	return Model{
		candidates: items,
		selected:   map[string]bool{},
		rejected:   map[string]bool{},
		i18n:       tr,
	}
}

func (m Model) UpdateKey(key string) Model {
	key = normalizeKey(key)
	if m.done || m.canceled {
		return m
	}
	if m.searching {
		return m.updateSearch(key)
	}
	switch key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.visible())-1 {
			m.cursor++
		}
	case "space":
		m.toggleSelected()
	case "/":
		m.searching = true
		m.filter = ""
		m.cursor = 0
	case "s":
		m.sortMode = (m.sortMode + 1) % 4
		m.cursor = clampCursor(m.cursor, len(m.visible()))
	case "a":
		m.selectHighRelevance(0.75)
	case "r":
		m.toggleRejected()
	case "enter":
		if len(m.selected) == 0 {
			m.err = references.ConfirmError{Code: references.CodeNoneConfirmed, Message: m.i18n.Text(i18n.ReferencesErrNone)}
			return m
		}
		m.done = true
		m.err = nil
	case "q", "ctrl+c", "esc":
		m.canceled = true
	}
	return m
}

func (m Model) Decision(now time.Time) references.ConfirmationDecision {
	return references.ConfirmationDecision{
		ConfirmedIDs: sortedIDs(m.selected),
		RejectedIDs:  sortedIDs(m.rejected),
		ConfirmedAt:  now,
	}
}

func (m Model) Done() bool {
	return m.done
}

func (m Model) Canceled() bool {
	return m.canceled
}

func (m Model) Err() error {
	return m.err
}

func (m Model) SelectedIDs() []string {
	return sortedIDs(m.selected)
}

func (m Model) RejectedIDs() []string {
	return sortedIDs(m.rejected)
}

func (m Model) CursorID() string {
	visible := m.visible()
	if len(visible) == 0 || m.cursor < 0 || m.cursor >= len(visible) {
		return ""
	}
	return visible[m.cursor].ID
}

func (m Model) VisibleCandidates() []contracts.ReferenceCandidate {
	return m.visible()
}

func (m Model) SortMode() SortMode {
	return m.sortMode
}

func (m Model) Filter() string {
	return m.filter
}

func (m Model) Searching() bool {
	return m.searching
}

func (m Model) updateSearch(key string) Model {
	switch key {
	case "enter", "esc":
		m.searching = false
	case "backspace":
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
			m.cursor = clampCursor(m.cursor, len(m.visible()))
		}
	default:
		if len([]rune(key)) == 1 {
			m.filter += key
			m.cursor = 0
		}
	}
	return m
}

func (m *Model) toggleSelected() {
	candidate := m.current()
	if candidate.ID == "" {
		return
	}
	if m.selected[candidate.ID] {
		delete(m.selected, candidate.ID)
		return
	}
	m.selected[candidate.ID] = true
	delete(m.rejected, candidate.ID)
}

func (m *Model) toggleRejected() {
	candidate := m.current()
	if candidate.ID == "" {
		return
	}
	if m.rejected[candidate.ID] {
		delete(m.rejected, candidate.ID)
		return
	}
	m.rejected[candidate.ID] = true
	delete(m.selected, candidate.ID)
}

func (m *Model) selectHighRelevance(threshold float64) {
	for _, candidate := range m.candidates {
		if candidate.RelevanceScore >= threshold {
			m.selected[candidate.ID] = true
			delete(m.rejected, candidate.ID)
		}
	}
}

func (m Model) current() contracts.ReferenceCandidate {
	visible := m.visible()
	if len(visible) == 0 || m.cursor < 0 || m.cursor >= len(visible) {
		return contracts.ReferenceCandidate{}
	}
	return visible[m.cursor]
}

func (m Model) visible() []contracts.ReferenceCandidate {
	var out []contracts.ReferenceCandidate
	filter := strings.ToLower(strings.TrimSpace(m.filter))
	for _, candidate := range m.candidates {
		if filter == "" || candidateMatches(candidate, filter) {
			out = append(out, candidate)
		}
	}
	sortCandidates(out, m.sortMode)
	return out
}

func sortCandidates(candidates []contracts.ReferenceCandidate, mode SortMode) {
	switch mode {
	case SortRelevance:
		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].RelevanceScore == candidates[j].RelevanceScore {
				return candidates[i].ID < candidates[j].ID
			}
			return candidates[i].RelevanceScore > candidates[j].RelevanceScore
		})
	case SortYear:
		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].Year == candidates[j].Year {
				return candidates[i].ID < candidates[j].ID
			}
			return candidates[i].Year > candidates[j].Year
		})
	case SortTitle:
		sort.SliceStable(candidates, func(i, j int) bool {
			return strings.ToLower(candidates[i].Title) < strings.ToLower(candidates[j].Title)
		})
	}
}

func candidateMatches(candidate contracts.ReferenceCandidate, filter string) bool {
	fields := []string{
		candidate.ID,
		candidate.Title,
		strings.Join(candidate.Authors, " "),
		candidate.Source,
		candidate.DOI,
		candidate.URL,
		candidate.Abstract,
		candidate.Venue,
		candidate.RelevanceReason,
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), filter) {
			return true
		}
	}
	return false
}

func sortedIDs(values map[string]bool) []string {
	ids := make([]string, 0, len(values))
	for id, ok := range values {
		if ok {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func normalizeKey(key string) string {
	if key == " " {
		return "space"
	}
	return strings.ToLower(strings.TrimSpace(key))
}

func clampCursor(cursor, length int) int {
	if length <= 0 {
		return 0
	}
	if cursor >= length {
		return length - 1
	}
	if cursor < 0 {
		return 0
	}
	return cursor
}
