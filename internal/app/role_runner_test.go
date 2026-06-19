package app

// Unit tests for the module-22 bugfix role runners (B1/B2/B3): a scripted mock
// ChatModel drives the full writer -> claim extraction -> verify -> review ->
// commit chain over the quality-mini fixtures without any network access.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/voocel/agentcore"

	aipagent "github.com/thisxiaoyuQAQ/aipaper-cli/internal/agent"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/artifacts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/checkpoint"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/config"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/quality"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/references"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

// scriptedChatModel returns canned replies in order. Generate and
// GenerateStream share the same script.
type scriptedChatModel struct {
	replies []string
	calls   int
	prompts []string
}

func (m *scriptedChatModel) next(messages []agentcore.Message) (string, error) {
	if m.calls >= len(m.replies) {
		return "", fmt.Errorf("scripted model exhausted after %d calls", m.calls)
	}
	if len(messages) > 0 {
		m.prompts = append(m.prompts, messages[len(messages)-1].TextContent())
	}
	reply := m.replies[m.calls]
	m.calls++
	return reply, nil
}

func (m *scriptedChatModel) Generate(_ context.Context, messages []agentcore.Message, _ []agentcore.ToolSpec, _ ...agentcore.CallOption) (*agentcore.LLMResponse, error) {
	reply, err := m.next(messages)
	if err != nil {
		return nil, err
	}
	msg := assistantReply(reply)
	return &agentcore.LLMResponse{Message: msg}, nil
}

func (m *scriptedChatModel) GenerateStream(_ context.Context, messages []agentcore.Message, _ []agentcore.ToolSpec, _ ...agentcore.CallOption) (<-chan agentcore.StreamEvent, error) {
	reply, err := m.next(messages)
	if err != nil {
		return nil, err
	}
	ch := make(chan agentcore.StreamEvent, 8)
	go func() {
		defer close(ch)
		// Split the reply into a few deltas to exercise the streamer.
		for i := 0; i < len(reply); i += 40 {
			end := i + 40
			if end > len(reply) {
				end = len(reply)
			}
			ch <- agentcore.StreamEvent{Type: agentcore.StreamEventTextDelta, Delta: reply[i:end]}
		}
		final := assistantReply(reply)
		ch <- agentcore.StreamEvent{Type: agentcore.StreamEventDone, Message: final}
	}()
	return ch, nil
}

func (m *scriptedChatModel) SupportsTools() bool { return false }

func assistantReply(text string) agentcore.Message {
	return agentcore.Message{
		Role:      agentcore.RoleAssistant,
		Content:   []agentcore.ContentBlock{agentcore.TextBlock(text)},
		Usage:     &agentcore.Usage{Input: 100, Output: 50, TotalTokens: 150},
		Timestamp: time.Now(),
	}
}

func runnerTestConfig() config.Config {
	return config.Config{
		Provider: "mock",
		Model:    "mock-model",
		Providers: map[string]config.ProviderConfig{
			"mock": {Type: "openai", APIKey: "test-key"},
		},
	}
}

// setupRunnerStore prepares a store with requirements, outline, confirmed
// references, and an evidence table — the state right before write_chapter.
func setupRunnerStore(t *testing.T) (store.Store, string, string) {
	t.Helper()
	workDir := t.TempDir()
	if _, err := Bootstrap(workDir, config.Config{Provider: "mock", Model: "mock-model"}); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	s := store.New(workDir)
	req := contracts.Requirements{
		Topic:             "Behavioral treatment of insomnia",
		ResearchQuestions: []string{"How durable are CBT-I effects?"},
		Scope:             "mini review",
		Language:          "en",
		CitationStyle:     "APA",
		QualityMode:       quality.ModeEnhanced,
		TargetWords:       300,
		MaterialDir:       "unused",
		AllowOnlineSearch: false,
	}
	if _, err := store.WriteJSON(s.RequirementsPath(), req, store.Overwrite); err != nil {
		t.Fatalf("write requirements: %v", err)
	}

	candidates := contracts.ReferenceCandidates{Items: []contracts.ReferenceCandidate{
		{ID: "cand_001", Title: "CBT-I Outcomes", Authors: []string{"Chen, Li"}, Year: 2024,
			Source: "material", DOI: "10.1000/cbti-outcomes", Abstract: "RCT of CBT-I.", Status: "pending"},
		{ID: "cand_002", Title: "Digital Sleep Therapy", Authors: []string{"Garcia, Ana"}, Year: 2025,
			Source: "material", DOI: "10.1000/digital-sleep", Abstract: "App delivered programs.", Status: "pending"},
	}}
	if _, err := store.WriteJSON(s.Path("references", "candidates.json"), candidates, store.Overwrite); err != nil {
		t.Fatalf("write candidates: %v", err)
	}
	confirmation, err := references.ConfirmCandidates(s, candidates, references.ConfirmationDecision{
		ConfirmedIDs: []string{"cand_001", "cand_002"},
		ConfirmedAt:  time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("ConfirmCandidates() error = %v", err)
	}
	chenKey := confirmation.Confirmed.Items[0].Key
	garciaKey := confirmation.Confirmed.Items[1].Key

	outline := map[string]any{
		"title_suggestion": "Insomnia Treatment Mini Review",
		"chapters": []map[string]any{
			{"chapter_id": "ch01", "title": "Therapist Guided CBT-I"},
		},
	}
	if _, err := store.WriteJSON(s.Path("outline", "outline.json"), outline, store.Overwrite); err != nil {
		t.Fatalf("write outline: %v", err)
	}

	table := quality.EvidenceTable{
		GeneratedAt: time.Now().UTC(),
		Items: []quality.Evidence{
			{ID: "ev_001", ReferenceKey: chenKey, Depth: quality.DepthAbstract,
				Topics: []string{"cbt-i"}, KeyFindings: []string{"improved sleep onset latency"}, Confidence: "medium"},
		},
	}
	if _, err := quality.SaveEvidenceTable(s, table); err != nil {
		t.Fatalf("SaveEvidenceTable() error = %v", err)
	}
	plan := quality.SectionQualityPlan{
		GeneratedAt: time.Now().UTC(),
		Sections: []quality.SectionPlan{{
			ChapterID:           "ch01",
			Questions:           []string{"How effective is CBT-I?"},
			RequiredEvidenceIDs: []string{"ev_001"},
		}},
	}
	if _, err := quality.SaveSectionQualityPlan(s, plan); err != nil {
		t.Fatalf("SaveSectionQualityPlan() error = %v", err)
	}
	// Seed a checkpoint so role runners continue step numbering from it.
	if err := checkpoint.Record(s, checkpoint.Checkpoint{
		Step: 1, Phase: "section_quality_plan", CreatedAt: time.Now().UTC(),
		Outputs: []checkpoint.OutputArtifact{}, NextExpected: "write_chapter",
	}, contracts.Progress{Phase: "section_quality_plan", Status: "running", UpdatedAt: time.Now().UTC(),
		CompletedChapters: []string{}, PendingChapters: []string{"ch01"}}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	return s, chenKey, garciaKey
}

func writerReply(chenKey string) string {
	out := map[string]any{
		"draft_markdown": "## Therapist Guided CBT-I\n\nCBT-I improved sleep onset latency in a randomized trial.",
		"claims": []map[string]any{{
			"id": "ch01_claim_001", "text": "CBT-I improves sleep onset latency.",
			"importance": "high", "reference_keys": []string{chenKey},
			"evidence_ids": []string{"ev_001"}, "confidence": 0.8,
		}},
		"citation_mappings": []map[string]any{{
			"paragraph_id": "ch01_p001", "claim_ids": []string{"ch01_claim_001"},
			"reference_keys": []string{chenKey},
		}},
		"writer_notes": "evidence depth is abstract; conclusion kept moderate",
	}
	data, _ := json.Marshal(out)
	return string(data)
}

func TestWriterRunnerWritesGuardedBundleAndCheckpoint(t *testing.T) {
	s, chenKey, _ := setupRunnerStore(t)
	model := &scriptedChatModel{replies: []string{writerReply(chenKey)}}
	var events []contracts.RunEvent
	writer, err := NewWriterRunner(RoleRunnerOptions{
		Config: runnerTestConfig(), Store: s, Model: model,
		EventSink: func(ev contracts.RunEvent) { events = append(events, ev) },
	})
	if err != nil {
		t.Fatalf("NewWriterRunner() error = %v", err)
	}

	input, err := aipagent.BuildWriterChapterInput(s, json.RawMessage(`{"chapter_id":"ch01"}`))
	if err != nil {
		t.Fatalf("BuildWriterChapterInput() error = %v", err)
	}
	args, _ := json.Marshal(input)
	result, err := writer.RunWriter(context.Background(), args)
	if err != nil {
		t.Fatalf("RunWriter() error = %v", err)
	}
	resultMap := result.(map[string]any)
	if resultMap["draft_version"] != 1 || resultMap["guarded"] != true {
		t.Fatalf("result = %#v", resultMap)
	}

	// Draft bundle persisted
	if _, err := os.Stat(s.Path("drafts", "ch01", "draft-v1.md")); err != nil {
		t.Fatalf("draft not written: %v", err)
	}
	var claims contracts.ClaimsFile
	if err := store.ReadJSON(s.Path("drafts", "ch01", "claims-v1.json"), &claims); err != nil {
		t.Fatalf("read claims: %v", err)
	}
	if len(claims.Claims) != 1 || claims.Claims[0].EvidenceIDs[0] != "ev_001" {
		t.Fatalf("claims = %#v", claims)
	}

	// Checkpoint recorded with artifacts and progress updated
	validation := checkpoint.ValidateLatest(s)
	if !validation.OK || validation.Phase != "write_chapter" {
		t.Fatalf("validation = %#v", validation)
	}

	// Streaming deltas and usage were emitted
	kinds := map[string]int{}
	for _, ev := range events {
		kinds[ev.Kind]++
	}
	if kinds["content_delta"] == 0 || kinds["usage_update"] == 0 || kinds["chapter_status"] == 0 || kinds["checkpoint_saved"] == 0 {
		t.Fatalf("event kinds = %#v", kinds)
	}
	// The prompt carried the quality plan and the evidence protocol.
	prompt := model.prompts[0]
	for _, want := range []string{"ev_001", "evidence_ids", "Paper quality writing policy", "writer_notes"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("writer prompt missing %q: %s", want, prompt)
		}
	}
}

func TestWriterRunnerGuardBlocksClaimWithoutEvidence(t *testing.T) {
	s, chenKey, _ := setupRunnerStore(t)
	bad := map[string]any{
		"draft_markdown": "## Therapist Guided CBT-I\n\nText.",
		"claims": []map[string]any{{
			"id": "ch01_claim_001", "text": "CBT-I cures insomnia permanently.",
			"importance": "high", "reference_keys": []string{chenKey}, "confidence": 0.9,
		}},
		"writer_notes": "",
	}
	badJSON, _ := json.Marshal(bad)
	model := &scriptedChatModel{replies: []string{string(badJSON)}}
	writer, err := NewWriterRunner(RoleRunnerOptions{Config: runnerTestConfig(), Store: s, Model: model})
	if err != nil {
		t.Fatalf("NewWriterRunner() error = %v", err)
	}
	_, err = writer.RunWriter(context.Background(), json.RawMessage(`{"chapter_id":"ch01"}`))
	if err == nil || !strings.Contains(err.Error(), aipagent.CodeClaimMissingEvidence) {
		t.Fatalf("RunWriter() error = %v, want %s", err, aipagent.CodeClaimMissingEvidence)
	}
	if _, statErr := os.Stat(s.Path("drafts", "ch01", "draft-v1.md")); !os.IsNotExist(statErr) {
		t.Fatalf("guard failure must block the draft write, stat = %v", statErr)
	}
}

func TestEditorRunnerVerifyReviewCommitChain(t *testing.T) {
	s, chenKey, _ := setupRunnerStore(t)

	// Chapter draft via the writer runner
	writerModel := &scriptedChatModel{replies: []string{writerReply(chenKey)}}
	writer, err := NewWriterRunner(RoleRunnerOptions{Config: runnerTestConfig(), Store: s, Model: writerModel})
	if err != nil {
		t.Fatalf("NewWriterRunner() error = %v", err)
	}
	if _, err := writer.RunWriter(context.Background(), json.RawMessage(`{"chapter_id":"ch01"}`)); err != nil {
		t.Fatalf("RunWriter() error = %v", err)
	}
	// Deterministic claim extraction (existing module 26 step)
	if _, _, err := quality.ExtractChapterClaimGraph(s, "ch01", 1, false, time.Now().UTC()); err != nil {
		t.Fatalf("ExtractChapterClaimGraph() error = %v", err)
	}

	graph, err := quality.LoadClaimGraph(s)
	if err != nil || len(graph.Claims) != 1 {
		t.Fatalf("claim graph = %#v err = %v", graph, err)
	}
	claimID := graph.Claims[0].ID

	verdictReply, _ := json.Marshal(map[string]any{
		"verdicts": []map[string]any{{
			"claim_id": claimID, "support": "supported", "risk_level": "low",
			"verifier_note": "abstract-level RCT evidence matches the moderate claim",
		}},
	})
	reviewReply, _ := json.Marshal(map[string]any{
		"scores": map[string]int{
			"overall": 88, "citation_consistency": 95, "structure_logic": 86, "coverage": 85, "readability": 90,
		},
		"summary":              "Solid chapter grounded in confirmed evidence.",
		"unsupported_claims":   []string{},
		"required_fixes":       []string{},
		"optional_fixes":       []string{"tighten the opening sentence"},
		"rewrite_instructions": []any{},
	})

	editorModel := &scriptedChatModel{replies: []string{string(verdictReply), string(reviewReply)}}
	var events []contracts.RunEvent
	editor, err := NewEditorRunner(RoleRunnerOptions{
		Config: runnerTestConfig(), Store: s, Model: editorModel,
		EventSink: func(ev contracts.RunEvent) { events = append(events, ev) },
	})
	if err != nil {
		t.Fatalf("NewEditorRunner() error = %v", err)
	}

	// verify
	verifyResult, err := editor.RunEditor(context.Background(), json.RawMessage(`{"task":"verify","chapter_id":"ch01"}`))
	if err != nil {
		t.Fatalf("RunEditor(verify) error = %v", err)
	}
	verifyPrompt := editorModel.prompts[0]
	for _, want := range []string{"Paper Quality verifier policy", "Claim support rubric", "same object, relation, and scope", "claim_type"} {
		if !strings.Contains(verifyPrompt, want) {
			t.Fatalf("verify prompt missing %q: %s", want, verifyPrompt)
		}
	}
	if verifyResult.(map[string]any)["verdicts"] != 1 {
		t.Fatalf("verify result = %#v", verifyResult)
	}
	verification, err := quality.LoadVerificationResult(s)
	if err != nil || len(verification.Verdicts) != 1 {
		t.Fatalf("verification = %#v err = %v", verification, err)
	}

	// review
	reviewResult, err := editor.RunEditor(context.Background(), json.RawMessage(`{"task":"review","chapter_id":"ch01"}`))
	if err != nil {
		t.Fatalf("RunEditor(review) error = %v", err)
	}
	reviewPrompt := editorModel.prompts[1]
	for _, want := range []string{"Paper Quality editor policy", "Reviewer rubric", "Rewrite rubric", "Human review trigger"} {
		if !strings.Contains(reviewPrompt, want) {
			t.Fatalf("review prompt missing %q: %s", want, reviewPrompt)
		}
	}
	reviewMap := reviewResult.(map[string]any)
	review := reviewMap["review"].(contracts.Review)
	if !review.Passed || review.Scores.Overall != 88 {
		t.Fatalf("review = %#v", review)
	}

	// commit
	commitResult, err := editor.RunEditor(context.Background(), json.RawMessage(`{"task":"commit","chapter_id":"ch01"}`))
	if err != nil {
		t.Fatalf("RunEditor(commit) error = %v", err)
	}
	commitMap := commitResult.(map[string]any)
	if commitMap["writing_completed"] != true {
		t.Fatalf("commit result = %#v", commitMap)
	}
	if _, err := os.Stat(s.Path("drafts", "ch01", "accepted.md")); err != nil {
		t.Fatalf("accepted.md not written: %v", err)
	}

	var finalProgress contracts.Progress
	if err := store.ReadJSON(s.ProgressPath(), &finalProgress); err != nil {
		t.Fatalf("read progress: %v", err)
	}
	if finalProgress.Status != "completed" || len(finalProgress.PendingChapters) != 0 || finalProgress.CompletedChapters[0] != "ch01" {
		t.Fatalf("progress = %#v", finalProgress)
	}
}

func TestArchitectRunnerOutlineEvidencePlanChain(t *testing.T) {
	workDir := t.TempDir()
	if _, err := Bootstrap(workDir, config.Config{Provider: "mock", Model: "mock-model"}); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	s := store.New(workDir)
	req := contracts.Requirements{
		Topic: "Behavioral treatment of insomnia", Scope: "mini review",
		Language: "en", CitationStyle: "APA", TargetWords: 300, MaterialDir: "unused",
	}
	if _, err := store.WriteJSON(s.RequirementsPath(), req, store.Overwrite); err != nil {
		t.Fatalf("write requirements: %v", err)
	}
	candidates := contracts.ReferenceCandidates{Items: []contracts.ReferenceCandidate{
		{ID: "cand_001", Title: "CBT-I Outcomes", Authors: []string{"Chen, Li"}, Year: 2024,
			Source: "material", DOI: "10.1000/cbti-outcomes", Abstract: "RCT of CBT-I.", Status: "pending"},
	}}
	if _, err := store.WriteJSON(s.Path("references", "candidates.json"), candidates, store.Overwrite); err != nil {
		t.Fatalf("write candidates: %v", err)
	}
	confirmation, err := references.ConfirmCandidates(s, candidates, references.ConfirmationDecision{
		ConfirmedIDs: []string{"cand_001"}, ConfirmedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("ConfirmCandidates() error = %v", err)
	}
	chenKey := confirmation.Confirmed.Items[0].Key
	if err := checkpoint.Record(s, checkpoint.Checkpoint{
		Step: 1, Phase: "confirm_references", CreatedAt: time.Now().UTC(),
		Outputs: []checkpoint.OutputArtifact{}, NextExpected: "create_outline",
	}, contracts.Progress{Phase: "confirm_references", Status: "running", UpdatedAt: time.Now().UTC(),
		CompletedChapters: []string{}, PendingChapters: []string{}}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	outlineReply, _ := json.Marshal(map[string]any{
		"title_suggestion": "Insomnia Treatment Mini Review",
		"chapters":         []map[string]any{{"title": "Therapist Guided CBT-I"}},
	})
	evidenceReply, _ := json.Marshal(map[string]any{
		"items": []map[string]any{{
			"reference_key": chenKey, "depth": "abstract",
			"topics": []string{"cbt-i"}, "key_findings": []string{"improved sleep onset latency"},
			"confidence": "medium",
		}},
	})
	planReply, _ := json.Marshal(map[string]any{
		"sections": []map[string]any{{
			"chapter_id": "ch01", "questions": []string{"How effective is CBT-I?"},
			"required_evidence_ids": []string{"ev_001"},
			"boundaries":            []string{"abstract-level evidence cannot support absolute conclusions"},
		}},
	})

	model := &scriptedChatModel{replies: []string{string(outlineReply), string(evidenceReply), string(planReply)}}
	architect, err := NewArchitectRunner(RoleRunnerOptions{Config: runnerTestConfig(), Store: s, Model: model})
	if err != nil {
		t.Fatalf("NewArchitectRunner() error = %v", err)
	}

	if _, err := architect.RunArchitect(context.Background(), json.RawMessage(`{"task":"outline"}`)); err != nil {
		t.Fatalf("RunArchitect(outline) error = %v", err)
	}
	for _, want := range []string{"Paper Quality narrative policy", "Narrative contract", "chapter needs one clear role"} {
		if !strings.Contains(model.prompts[0], want) {
			t.Fatalf("outline prompt missing %q: %s", want, model.prompts[0])
		}
	}
	var outline struct {
		TitleSuggestion string `json:"title_suggestion"`
		Chapters        []struct {
			ChapterID string `json:"chapter_id"`
			Title     string `json:"title"`
		} `json:"chapters"`
	}
	if err := store.ReadJSON(s.Path("outline", "outline.json"), &outline); err != nil {
		t.Fatalf("read outline: %v", err)
	}
	if len(outline.Chapters) != 1 || outline.Chapters[0].ChapterID != "ch01" {
		t.Fatalf("outline = %#v", outline)
	}

	if _, err := architect.RunArchitect(context.Background(), json.RawMessage(`{"task":"evidence_extraction"}`)); err != nil {
		t.Fatalf("RunArchitect(evidence_extraction) error = %v", err)
	}
	for _, want := range []string{"Paper Quality evidence-depth policy", "Evidence depth rubric", "Depth honesty"} {
		if !strings.Contains(model.prompts[1], want) {
			t.Fatalf("evidence prompt missing %q: %s", want, model.prompts[1])
		}
	}
	table, err := quality.LoadEvidenceTable(s)
	if err != nil || len(table.Items) != 1 || table.Items[0].ID != "ev_001" {
		t.Fatalf("evidence table = %#v err = %v", table, err)
	}

	if _, err := architect.RunArchitect(context.Background(), json.RawMessage(`{"task":"section_quality_plan"}`)); err != nil {
		t.Fatalf("RunArchitect(section_quality_plan) error = %v", err)
	}
	for _, want := range []string{"Paper Quality section-plan policy", "SectionPlan rubric", "forbidden_generalizations"} {
		if !strings.Contains(model.prompts[2], want) {
			t.Fatalf("section plan prompt missing %q: %s", want, model.prompts[2])
		}
	}
	plan, err := quality.LoadSectionQualityPlan(s)
	if err != nil || len(plan.Sections) != 1 || plan.Sections[0].ChapterID != "ch01" {
		t.Fatalf("plan = %#v err = %v", plan, err)
	}

	validation := checkpoint.ValidateLatest(s)
	if !validation.OK || validation.Phase != aipagent.StepSectionQualityPlan || validation.Next != "write_chapter" {
		t.Fatalf("validation = %#v", validation)
	}
}

func TestJSONStringStreamerExtractsDraftAcrossDeltas(t *testing.T) {
	streamer := newJSONStringStreamer("draft_markdown")
	full := `{"draft_markdown":"## Title\n\nBody text.","claims":[]}`
	var collected strings.Builder
	for i := 0; i < len(full); i += 7 {
		end := i + 7
		if end > len(full) {
			end = len(full)
		}
		collected.WriteString(streamer.feed(full[i:end]))
	}
	if got := collected.String(); got != "## Title\n\nBody text." {
		t.Fatalf("streamed draft = %q", got)
	}
}

// writerReplyTwoClaims produces a chapter with two distinct claims, both bound
// to ev_001, so the verifier can be scripted to omit one of them.
func writerReplyTwoClaims(chenKey string) string {
	out := map[string]any{
		"draft_markdown": "## Therapist Guided CBT-I\n\nCBT-I improved sleep onset latency. It also reduced wake after sleep onset.",
		"claims": []map[string]any{
			{"id": "ch01_claim_001", "text": "CBT-I improves sleep onset latency.",
				"importance": "high", "reference_keys": []string{chenKey}, "evidence_ids": []string{"ev_001"}, "confidence": 0.8},
			{"id": "ch01_claim_002", "text": "CBT-I reduces wake after sleep onset.",
				"importance": "medium", "reference_keys": []string{chenKey}, "evidence_ids": []string{"ev_001"}, "confidence": 0.7},
		},
		"citation_mappings": []map[string]any{{
			"paragraph_id": "ch01_p001", "claim_ids": []string{"ch01_claim_001", "ch01_claim_002"},
			"reference_keys": []string{chenKey},
		}},
		"writer_notes": "two evidence-grounded claims at abstract depth",
	}
	data, _ := json.Marshal(out)
	return string(data)
}

// TestEditorRunnerVerifierMarksOmittedClaimsUnsupported locks in the runVerify
// behavior change: when the model omits a verdict for a listed claim, the host
// injects an unsupported/high-risk verdict for it instead of silently dropping
// the claim.
func TestEditorRunnerVerifierMarksOmittedClaimsUnsupported(t *testing.T) {
	s, chenKey, _ := setupRunnerStore(t)

	writerModel := &scriptedChatModel{replies: []string{writerReplyTwoClaims(chenKey)}}
	writer, err := NewWriterRunner(RoleRunnerOptions{Config: runnerTestConfig(), Store: s, Model: writerModel})
	if err != nil {
		t.Fatalf("NewWriterRunner() error = %v", err)
	}
	if _, err := writer.RunWriter(context.Background(), json.RawMessage(`{"chapter_id":"ch01"}`)); err != nil {
		t.Fatalf("RunWriter() error = %v", err)
	}
	if _, _, err := quality.ExtractChapterClaimGraph(s, "ch01", 1, false, time.Now().UTC()); err != nil {
		t.Fatalf("ExtractChapterClaimGraph() error = %v", err)
	}
	graph, err := quality.LoadClaimGraph(s)
	if err != nil || len(graph.Claims) != 2 {
		t.Fatalf("claim graph = %#v err = %v", graph, err)
	}
	firstClaim := graph.Claims[0].ID
	secondClaim := graph.Claims[1].ID

	// The model returns a verdict for only the first claim, omitting the second.
	omittedReply, _ := json.Marshal(map[string]any{
		"verdicts": []map[string]any{{
			"claim_id": firstClaim, "support": "supported", "risk_level": "low",
			"verifier_note": "abstract evidence matches the moderate claim",
		}},
	})
	editorModel := &scriptedChatModel{replies: []string{string(omittedReply)}}
	editor, err := NewEditorRunner(RoleRunnerOptions{Config: runnerTestConfig(), Store: s, Model: editorModel})
	if err != nil {
		t.Fatalf("NewEditorRunner() error = %v", err)
	}
	verifyResult, err := editor.RunEditor(context.Background(), json.RawMessage(`{"task":"verify","chapter_id":"ch01"}`))
	if err != nil {
		t.Fatalf("RunEditor(verify) error = %v", err)
	}
	if verifyResult.(map[string]any)["verdicts"] != 2 {
		t.Fatalf("expected 2 verdicts (1 model + 1 host-injected), got %#v", verifyResult)
	}
	verification, err := quality.LoadVerificationResult(s)
	if err != nil || len(verification.Verdicts) != 2 {
		t.Fatalf("verification = %#v err = %v", verification, err)
	}
	var omitted *quality.ClaimVerdict
	for i := range verification.Verdicts {
		if verification.Verdicts[i].ClaimID == secondClaim {
			omitted = &verification.Verdicts[i]
		}
	}
	if omitted == nil {
		t.Fatalf("omitted claim %s has no verdict: %#v", secondClaim, verification.Verdicts)
	}
	if omitted.Support != quality.SupportUnsupported || omitted.RiskLevel != quality.RiskHigh {
		t.Fatalf("omitted claim verdict = %#v, want unsupported/high", omitted)
	}
	if !strings.Contains(omitted.VerifierNote, "omitted") {
		t.Fatalf("omitted claim note should explain the host injection: %q", omitted.VerifierNote)
	}
}

// TestEditorRunnerQualityGateOverridesPassingReview locks in the
// applyQualityConclusion behavior change: when the Editor's own review passes
// (scores >= thresholds, no required fixes, no forced rewrite instruction) but
// the claim-level quality gate concludes needs_revision, the review gate is
// overridden to revision_required and the chapter is NOT auto-committed.
//
// A partially_supported claim in strict mode is the clean trigger: the review
// guard does NOT force a rewrite instruction for partially_supported claims
// (only unsupported/overstated/needs_rewrite), so the review can still pass,
// while strict mode escalates partially_supported to needs_revision.
func TestEditorRunnerQualityGateOverridesPassingReview(t *testing.T) {
	s, chenKey, _ := setupRunnerStore(t)

	// Escalate to strict mode so a partially_supported claim becomes
	// needs_revision at the quality gate.
	var req contracts.Requirements
	if err := store.ReadJSON(s.RequirementsPath(), &req); err != nil {
		t.Fatalf("read requirements: %v", err)
	}
	req.QualityMode = quality.ModeStrict
	if _, err := store.WriteJSON(s.RequirementsPath(), req, store.Overwrite); err != nil {
		t.Fatalf("write requirements: %v", err)
	}

	writerModel := &scriptedChatModel{replies: []string{writerReply(chenKey)}}
	writer, err := NewWriterRunner(RoleRunnerOptions{Config: runnerTestConfig(), Store: s, Model: writerModel})
	if err != nil {
		t.Fatalf("NewWriterRunner() error = %v", err)
	}
	if _, err := writer.RunWriter(context.Background(), json.RawMessage(`{"chapter_id":"ch01"}`)); err != nil {
		t.Fatalf("RunWriter() error = %v", err)
	}
	if _, _, err := quality.ExtractChapterClaimGraph(s, "ch01", 1, false, time.Now().UTC()); err != nil {
		t.Fatalf("ExtractChapterClaimGraph() error = %v", err)
	}
	graph, err := quality.LoadClaimGraph(s)
	if err != nil || len(graph.Claims) != 1 {
		t.Fatalf("claim graph = %#v err = %v", graph, err)
	}
	claimID := graph.Claims[0].ID

	// Verifier marks the claim partially_supported (medium risk): this does NOT
	// force a rewrite instruction, so the review below can still pass on its own.
	verdictReply, _ := json.Marshal(map[string]any{
		"verdicts": []map[string]any{{
			"claim_id": claimID, "support": "partially_supported", "risk_level": "medium",
			"verifier_note": "abstract evidence only partially supports the claim",
		}},
	})
	// The Editor's own review passes: scores above thresholds, no unsupported
	// claims, no required rewrite instructions. Without applyQualityConclusion
	// this chapter would be accepted.
	reviewReply, _ := json.Marshal(map[string]any{
		"scores": map[string]int{
			"overall": 88, "citation_consistency": 95, "structure_logic": 86, "coverage": 85, "readability": 90,
		},
		"summary":              "Well-written chapter.",
		"unsupported_claims":   []string{},
		"required_fixes":       []string{},
		"optional_fixes":       []string{"tighten the opening"},
		"rewrite_instructions": []any{},
	})
	editorModel := &scriptedChatModel{replies: []string{string(verdictReply), string(reviewReply)}}
	editor, err := NewEditorRunner(RoleRunnerOptions{Config: runnerTestConfig(), Store: s, Model: editorModel})
	if err != nil {
		t.Fatalf("NewEditorRunner() error = %v", err)
	}
	if _, err := editor.RunEditor(context.Background(), json.RawMessage(`{"task":"verify","chapter_id":"ch01"}`)); err != nil {
		t.Fatalf("RunEditor(verify) error = %v", err)
	}
	reviewResult, err := editor.RunEditor(context.Background(), json.RawMessage(`{"task":"review","chapter_id":"ch01"}`))
	if err != nil {
		t.Fatalf("RunEditor(review) error = %v", err)
	}
	result := reviewResult.(map[string]any)
	if result["quality_conclusion"] != quality.GateNeedsRevision {
		t.Fatalf("quality_conclusion = %#v, want %q", result["quality_conclusion"], quality.GateNeedsRevision)
	}
	gate, ok := result["gate"].(artifacts.GateResult)
	if !ok {
		t.Fatalf("gate = %#v", result["gate"])
	}
	if gate.Passed || gate.Status != artifacts.StatusRevisionRequired {
		t.Fatalf("gate = %#v, want Passed=false Status=%q (quality gate must override the passing review)", gate, artifacts.StatusRevisionRequired)
	}
	if result["auto_committed"] == true {
		t.Fatalf("chapter must not be auto-committed when the quality gate overrides the review")
	}
	if _, statErr := os.Stat(s.Path("drafts", "ch01", "accepted.md")); !os.IsNotExist(statErr) {
		t.Fatalf("accepted.md must not exist when quality gate overrides the review, stat = %v", statErr)
	}
}

// TestWriterRunnerGuardsChapterWithPlanButNoEvidence locks in the writer_runner
// guarded-condition change: a chapter that has a section quality plan and an
// existing evidence table, but no resolvable evidence for this chapter, now
// runs through the evidence guard (guarded=true) instead of degrading to
// compatibility mode. The guard then blocks a claim that has no evidence to
// bind, preventing unsupported writing.
func TestWriterRunnerGuardsChapterWithPlanButNoEvidence(t *testing.T) {
	s, chenKey, _ := setupRunnerStore(t)

	// Rewrite the section plan so ch01 has no required evidence: the evidence
	// table still exists, but BuildWriterChapterInput resolves an empty Evidence
	// list for this chapter.
	plan, err := quality.LoadSectionQualityPlan(s)
	if err != nil {
		t.Fatalf("LoadSectionQualityPlan() error = %v", err)
	}
	plan.Sections[0].RequiredEvidenceIDs = nil
	if _, err := quality.SaveSectionQualityPlan(s, plan); err != nil {
		t.Fatalf("SaveSectionQualityPlan() error = %v", err)
	}

	// The writer attempts a claim (with a confirmed reference) but has no
	// evidence id to bind because none was resolved for the chapter.
	noEvidenceReply, _ := json.Marshal(map[string]any{
		"draft_markdown": "## Introduction\n\nThis review surveys CBT-I.",
		"claims": []map[string]any{{
			"id": "ch01_claim_001", "text": "CBT-I is a widely studied intervention.",
			"importance": "high", "reference_keys": []string{chenKey}, "confidence": 0.6,
		}},
		"citation_mappings": []map[string]any{{
			"paragraph_id": "ch01_p001", "claim_ids": []string{"ch01_claim_001"},
			"reference_keys": []string{chenKey},
		}},
		"writer_notes": "",
	})
	model := &scriptedChatModel{replies: []string{string(noEvidenceReply)}}
	writer, err := NewWriterRunner(RoleRunnerOptions{Config: runnerTestConfig(), Store: s, Model: model})
	if err != nil {
		t.Fatalf("NewWriterRunner() error = %v", err)
	}
	_, err = writer.RunWriter(context.Background(), json.RawMessage(`{"chapter_id":"ch01"}`))
	if err == nil || !strings.Contains(err.Error(), aipagent.CodeClaimMissingEvidence) {
		t.Fatalf("RunWriter() error = %v, want guard block %s (evidence guard must be active for plan+table chapters even without chapter evidence)", err, aipagent.CodeClaimMissingEvidence)
	}
	if _, statErr := os.Stat(s.Path("drafts", "ch01", "draft-v1.md")); !os.IsNotExist(statErr) {
		t.Fatalf("guard failure must block the draft write, stat = %v", statErr)
	}
}
