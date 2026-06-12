package app

// Module-22 bugfix (BUG-20260613-01, plan B) boundary additions: shared LLM
// role-runner infrastructure. Each role runner (writer/editor/architect)
// resolves its own ChatModel through the existing ResolveRoleRuntime +
// NewChatModel chain, emits host-side run events (streaming deltas, usage,
// chapter status, checkpoint markers) through a RunnerEventSink, and records
// step checkpoints through the existing checkpoint mechanism. No decision
// rules live here: runners only produce content and persist it through the
// guarded write paths.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/voocel/agentcore"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/checkpoint"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/config"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

// RunnerEventSink receives host-side run events emitted by role runners
// (content deltas, usage updates, chapter status, checkpoint markers). The
// kinds match the TUI bridge in internal/tui/writing/events.go.
type RunnerEventSink func(contracts.RunEvent)

// Run event kinds emitted by role runners. They are bridged 1:1 by
// writing.BridgeRunEvent.
const (
	runEventContentDelta    = "content_delta"
	runEventUsageUpdate     = "usage_update"
	runEventChapterStatus   = "chapter_status"
	runEventCheckpointSaved = "checkpoint_saved"
	runEventRoleLog         = "role_log"
)

const roleCallTimeout = 10 * time.Minute

// RoleRunnerOptions configures one role runner. Model overrides the
// config-resolved ChatModel for tests; EventSink and Now are optional.
type RoleRunnerOptions struct {
	Config    config.Config
	Store     store.Store
	EventSink RunnerEventSink
	Now       func() time.Time
	Model     agentcore.ChatModel
}

// roleChat wraps one role's ChatModel with event emission and the JSON
// ask/parse/retry protocol proven by tools/real-before-after.
type roleChat struct {
	role     string
	provider string
	model    string
	chat     agentcore.ChatModel
	store    store.Store
	emit     RunnerEventSink
	now      func() time.Time
}

func newRoleChat(role string, opts RoleRunnerOptions) (*roleChat, error) {
	rc := &roleChat{role: role, store: opts.Store, emit: opts.EventSink, now: opts.Now}
	if rc.now == nil {
		rc.now = func() time.Time { return time.Now().UTC() }
	}
	if rc.emit == nil {
		rc.emit = func(contracts.RunEvent) {}
	}
	roleCfg, err := ResolveRoleRuntime(opts.Config, role)
	if err != nil {
		return nil, err
	}
	rc.provider = roleCfg.Provider
	rc.model = roleCfg.Model
	rc.chat = opts.Model
	if rc.chat == nil {
		chat, err := NewChatModel(roleCfg)
		if err != nil {
			return nil, err
		}
		rc.chat = chat
	}
	return rc, nil
}

func (rc *roleChat) event(kind, chapterID, message string, fields map[string]any) {
	if fields == nil {
		fields = map[string]any{}
	}
	fields["agent"] = rc.role
	if chapterID != "" {
		fields["chapter_id"] = chapterID
	}
	rc.emit(contracts.RunEvent{At: rc.now(), Kind: kind, Message: message, Fields: fields})
}

func (rc *roleChat) chapterStatus(chapterID, status string, extra map[string]any) {
	fields := map[string]any{"status": status}
	for k, v := range extra {
		fields[k] = v
	}
	rc.event(runEventChapterStatus, chapterID, fmt.Sprintf("chapter %s: %s", chapterID, status), fields)
}

func (rc *roleChat) emitUsage(usage *agentcore.Usage) {
	if usage == nil {
		return
	}
	fields := map[string]any{
		"input_tokens":  int64(usage.Input),
		"output_tokens": int64(usage.Output),
		"model":         rc.model,
	}
	if usage.TotalTokens > 0 {
		fields["context_tokens"] = int64(usage.TotalTokens)
	}
	if usage.Cost != nil {
		fields["cost_usd"] = usage.Cost.Total
	}
	rc.event(runEventUsageUpdate, "", "", fields)
}

// askJSON sends the prompt, parses the JSON reply into out, and retries once
// with the parse error appended (same protocol as tools/real-before-after).
// When streamChapterID is non-empty the call streams and emits the
// draft_markdown JSON string value as content_delta events for that chapter.
func (rc *roleChat) askJSON(ctx context.Context, streamChapterID, prompt string, out any) error {
	ctx, cancel := context.WithTimeout(ctx, roleCallTimeout)
	defer cancel()
	ask := prompt
	for attempt := 1; attempt <= 2; attempt++ {
		text, err := rc.generate(ctx, streamChapterID, ask)
		if err != nil {
			return err
		}
		cleaned := extractJSONObject(text)
		if err := strictJSONInto(cleaned, out); err == nil {
			return nil
		} else if attempt == 2 {
			return fmt.Errorf("%s: parse model JSON: %w; raw=%q", rc.role, err, truncate(cleaned, 400))
		}
		ask = prompt + "\n\nYour previous reply was not valid JSON. Reply with ONLY the JSON object."
	}
	return nil
}

func (rc *roleChat) generate(ctx context.Context, streamChapterID, ask string) (string, error) {
	messages := []agentcore.Message{agentcore.UserMsg(ask)}
	if streamChapterID == "" {
		resp, err := rc.chat.Generate(ctx, messages, nil)
		if err != nil {
			return "", err
		}
		rc.emitUsage(resp.Message.Usage)
		return resp.Message.TextContent(), nil
	}

	events, err := rc.chat.GenerateStream(ctx, messages, nil)
	if err != nil {
		return "", err
	}
	var raw strings.Builder
	var final *agentcore.Message
	streamer := newJSONStringStreamer("draft_markdown")
	for ev := range events {
		switch ev.Type {
		case agentcore.StreamEventTextDelta:
			raw.WriteString(ev.Delta)
			if display := streamer.feed(ev.Delta); display != "" {
				rc.event(runEventContentDelta, streamChapterID, "", map[string]any{"delta": display})
			}
		case agentcore.StreamEventDone:
			msg := ev.Message
			final = &msg
		case agentcore.StreamEventError:
			if ev.Err != nil {
				return "", ev.Err
			}
		}
	}
	if final != nil {
		rc.emitUsage(final.Usage)
		if text := final.TextContent(); strings.TrimSpace(text) != "" {
			return text, nil
		}
	}
	return raw.String(), nil
}

// jsonStringStreamer incrementally extracts one top-level JSON string value
// (by key) from a streamed JSON document so the TUI can render the chapter
// draft while the model is still writing. Display-only: final parsing always
// happens on the complete reply.
type jsonStringStreamer struct {
	key     string
	buf     strings.Builder // searched prefix until the value starts
	inValue bool
	done    bool
	escape  strings.Builder // pending escape sequence (may split across deltas)
}

func newJSONStringStreamer(key string) *jsonStringStreamer {
	return &jsonStringStreamer{key: key}
}

func (j *jsonStringStreamer) feed(delta string) string {
	if j.done {
		return ""
	}
	if !j.inValue {
		j.buf.WriteString(delta)
		marker := regexp.MustCompile(`"` + regexp.QuoteMeta(j.key) + `"\s*:\s*"`)
		loc := marker.FindStringIndex(j.buf.String())
		if loc == nil {
			return ""
		}
		j.inValue = true
		rest := j.buf.String()[loc[1]:]
		j.buf.Reset()
		return j.consumeValue(rest)
	}
	return j.consumeValue(delta)
}

func (j *jsonStringStreamer) consumeValue(chunk string) string {
	var out strings.Builder
	for i := 0; i < len(chunk); i++ {
		ch := chunk[i]
		if j.escape.Len() > 0 {
			j.escape.WriteByte(ch)
			seq := j.escape.String()
			decoded, complete := decodeJSONEscape(seq)
			if complete {
				out.WriteString(decoded)
				j.escape.Reset()
			} else if len(seq) > 6 { // malformed; flush as-is
				out.WriteString(seq)
				j.escape.Reset()
			}
			continue
		}
		switch ch {
		case '\\':
			j.escape.WriteByte(ch)
		case '"':
			j.done = true
			return out.String()
		default:
			out.WriteByte(ch)
		}
	}
	return out.String()
}

func decodeJSONEscape(seq string) (string, bool) {
	if len(seq) < 2 {
		return "", false
	}
	switch seq[1] {
	case 'n':
		return "\n", true
	case 't':
		return "\t", true
	case 'r':
		return "", true
	case '"':
		return `"`, true
	case '\\':
		return `\`, true
	case '/':
		return "/", true
	case 'b', 'f':
		return "", true
	case 'u':
		if len(seq) < 6 {
			return "", false
		}
		code, err := strconv.ParseUint(seq[2:6], 16, 32)
		if err != nil {
			return seq, true
		}
		return string(rune(code)), true
	default:
		return seq, true
	}
}

func extractJSONObject(text string) string {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	if i := strings.Index(text, "{"); i > 0 {
		text = text[i:]
	}
	if i := strings.LastIndex(text, "}"); i >= 0 {
		text = text[:i+1]
	}
	return strings.TrimSpace(text)
}

func strictJSONInto(text string, out any) error {
	return json.Unmarshal([]byte(text), out)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// recordStepCheckpoint persists artifacts-then-checkpoint per the existing
// checkpoint rule: outputs are already on disk when Record runs. Step numbers
// continue from the latest checkpoint. The progress mutation runs on the
// current progress.json content.
func (rc *roleChat) recordStepCheckpoint(phase, next string, outputs []checkpoint.OutputArtifact, mutate func(*contracts.Progress)) error {
	latest, _, err := checkpoint.LoadLatest(rc.store)
	if err != nil {
		return fmt.Errorf("load latest checkpoint: %w", err)
	}
	step := latest.Step + 1
	var progress contracts.Progress
	if err := store.ReadJSON(rc.store.ProgressPath(), &progress); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read progress.json: %w", err)
	}
	now := rc.now()
	progress.Phase = phase
	progress.Status = "running"
	progress.UpdatedAt = now
	if progress.CompletedChapters == nil {
		progress.CompletedChapters = []string{}
	}
	if progress.PendingChapters == nil {
		progress.PendingChapters = []string{}
	}
	if mutate != nil {
		mutate(&progress)
	}
	cp := checkpoint.Checkpoint{
		Step:         step,
		Phase:        phase,
		CreatedAt:    now,
		Outputs:      outputs,
		NextExpected: next,
	}
	if err := checkpoint.Record(rc.store, cp, progress); err != nil {
		return err
	}
	rc.event(runEventCheckpointSaved, "", fmt.Sprintf("checkpoint %d (%s) saved", step, phase), map[string]any{
		"step": step, "phase": phase, "next_expected": next,
	})
	return nil
}

// ---------------------------------------------------------------------------
// Shared fact loaders (host-side, read-only)
// ---------------------------------------------------------------------------

type outlineChapter struct {
	ChapterID string `json:"chapter_id"`
	Title     string `json:"title"`
}

type outlineDoc struct {
	TitleSuggestion string           `json:"title_suggestion"`
	Chapters        []outlineChapter `json:"chapters"`
}

func loadOutlineDoc(s store.Store) (outlineDoc, error) {
	var outline outlineDoc
	if err := store.ReadJSON(s.Path("outline", "outline.json"), &outline); err != nil {
		return outlineDoc{}, fmt.Errorf("read outline/outline.json: %w", err)
	}
	return outline, nil
}

func loadRequirementsDoc(s store.Store) (contracts.Requirements, error) {
	var req contracts.Requirements
	if err := store.ReadJSON(s.RequirementsPath(), &req); err != nil {
		return contracts.Requirements{}, fmt.Errorf("read requirements.json: %w", err)
	}
	return req, nil
}

func loadConfirmedRefs(s store.Store) (contracts.ConfirmedReferences, error) {
	var confirmed contracts.ConfirmedReferences
	err := store.ReadJSON(s.Path("references", "confirmed.json"), &confirmed)
	if errors.Is(err, os.ErrNotExist) {
		return contracts.ConfirmedReferences{Items: []contracts.ConfirmedReference{}}, nil
	}
	if err != nil {
		return contracts.ConfirmedReferences{}, err
	}
	return confirmed, nil
}

func confirmedRefLines(confirmed contracts.ConfirmedReferences) []string {
	lines := make([]string, 0, len(confirmed.Items))
	for _, ref := range confirmed.Items {
		line := fmt.Sprintf("- key=%s | %s (%d)", ref.Key, ref.Title, ref.Year)
		if len(ref.SourceMaterialIDs) > 0 {
			line += " | materials: " + strings.Join(ref.SourceMaterialIDs, ",")
		}
		if ref.Abstract != "" {
			line += ": " + truncate(ref.Abstract, 600)
		}
		lines = append(lines, line)
	}
	return lines
}

const (
	maxMaterialChars      = 4000
	maxTotalMaterialChars = 24000
)

// loadMaterialTexts returns "material_id" -> extracted text (trimmed), reading
// the manifest written by materials.ProcessDir. Missing manifest degrades to
// an empty map so legacy runs keep working.
func loadMaterialTexts(s store.Store) map[string]string {
	var manifest contracts.MaterialManifest
	if err := store.ReadJSON(s.Path("materials", "manifest.json"), &manifest); err != nil {
		return map[string]string{}
	}
	out := map[string]string{}
	total := 0
	for _, item := range manifest.Items {
		if item.Status != "parsed" || item.OutputText == "" || total >= maxTotalMaterialChars {
			continue
		}
		data, err := os.ReadFile(s.Path(item.OutputText))
		if err != nil {
			continue
		}
		text := truncate(strings.TrimSpace(string(data)), maxMaterialChars)
		out[item.ID] = text
		total += len(text)
	}
	return out
}

func materialDigest(texts map[string]string) string {
	if len(texts) == 0 {
		return "(no parsed source materials)"
	}
	ids := make([]string, 0, len(texts))
	for id := range texts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var b strings.Builder
	for _, id := range ids {
		fmt.Fprintf(&b, "[material %s]\n%s\n\n", id, texts[id])
	}
	return b.String()
}

var draftVersionPattern = regexp.MustCompile(`^draft-v(\d+)\.md$`)

// latestDraftVersion scans drafts/<chapter>/ for the highest draft-vN.md.
// Returns 0 when the chapter has no drafts yet.
func latestDraftVersion(s store.Store, chapterID string) (int, error) {
	entries, err := os.ReadDir(s.Path("drafts", chapterID))
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	latest := 0
	for _, entry := range entries {
		match := draftVersionPattern.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		if v, err := strconv.Atoi(match[1]); err == nil && v > latest {
			latest = v
		}
	}
	return latest, nil
}
