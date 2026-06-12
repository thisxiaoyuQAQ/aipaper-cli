// Package main is a throwaway real-provider before/after harness for module 31
// layer three. The product TUI->AgentRuntime wiring is missing (module 22 gap),
// so this harness drives the real LLM directly through the product contracts:
//
//	before: legacy flow - writer prompt without evidence protocol, claims have
//	        no evidence_ids, artifacts.WriteDraftBundle, no quality artifacts.
//	after:  enhanced flow - real architect plan, writer prompt with evidence
//	        protocol, agent.WriteGuardedDraftBundle, claim extraction, real
//	        verifier verdicts, quality gate, quality-report export.
//
// Editor scoring stays mock-passed in BOTH flows so the comparison isolates
// the evidence protocol variable. The API key is injected via env only.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/voocel/agentcore"

	aipagent "github.com/thisxiaoyuQAQ/aipaper-cli/internal/agent"
	runtimeapp "github.com/thisxiaoyuQAQ/aipaper-cli/internal/app"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/artifacts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/config"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	finalexport "github.com/thisxiaoyuQAQ/aipaper-cli/internal/export"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/materials"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/quality"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/references"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

var chat agentcore.ChatModel

const (
	chapter01 = "ch01"
	chapter02 = "ch02"
)

type writerOut struct {
	DraftMarkdown string `json:"draft_markdown"`
	Claims        []struct {
		ID            string   `json:"id"`
		Text          string   `json:"text"`
		Importance    string   `json:"importance"`
		ReferenceKeys []string `json:"reference_keys"`
		EvidenceIDs   []string `json:"evidence_ids,omitempty"`
		Confidence    float64  `json:"confidence"`
	} `json:"claims"`
	WriterNotes string `json:"writer_notes"`
}

type planOut struct {
	Sections []struct {
		ChapterID                string   `json:"chapter_id"`
		Questions                []string `json:"questions"`
		RequiredEvidenceIDs      []string `json:"required_evidence_ids"`
		Boundaries               []string `json:"boundaries"`
		ForbiddenGeneralizations []string `json:"forbidden_generalizations"`
	} `json:"sections"`
}

type verdictOut struct {
	Verdicts []struct {
		ClaimID      string `json:"claim_id"`
		Support      string `json:"support"`
		RiskLevel    string `json:"risk_level,omitempty"`
		VerifierNote string `json:"verifier_note"`
	} `json:"verdicts"`
}

func main() {
	if err := run(); err != nil {
		fmt.Println("FATAL:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Config{
		Provider: "custom",
		Model:    os.Getenv("SMOKE_MODEL"),
		Providers: map[string]config.ProviderConfig{
			"custom": {Type: "openai", APIKey: "env:SMOKE_API_KEY", BaseURL: os.Getenv("SMOKE_BASE_URL")},
		},
		Roles: map[string]config.RoleConfig{aipagent.RoleCoordinator: {Provider: "custom", Model: os.Getenv("SMOKE_MODEL")}},
	}
	roleCfg, err := runtimeapp.ResolveRoleRuntime(cfg, aipagent.RoleCoordinator)
	if err != nil {
		return err
	}
	chat, err = runtimeapp.NewChatModel(roleCfg)
	if err != nil {
		return err
	}

	root := filepath.Join("agent-output", "real-before-after")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}

	fmt.Println("=== BEFORE (legacy flow, no quality artifacts) ===")
	if err := runFlow(filepath.Join(root, "run-before"), false); err != nil {
		return fmt.Errorf("before flow: %w", err)
	}
	fmt.Println("=== AFTER (enhanced flow, full evidence chain) ===")
	if err := runFlow(filepath.Join(root, "run-after"), true); err != nil {
		return fmt.Errorf("after flow: %w", err)
	}
	return nil
}

func runFlow(workDir string, enhanced bool) error {
	if err := os.RemoveAll(workDir); err != nil {
		return err
	}
	if _, err := runtimeapp.Bootstrap(workDir, config.Config{Provider: "custom", Model: os.Getenv("SMOKE_MODEL")}); err != nil {
		return err
	}
	s := store.New(workDir)

	mode := ""
	if enhanced {
		mode = quality.ModeEnhanced
	}
	req := contracts.Requirements{
		Topic:             "Behavioral and digital treatment of insomnia",
		ResearchQuestions: []string{"How durable are CBT-I effects across delivery modes?"},
		Scope:             "short mini review, two chapters",
		Language:          "en",
		CitationStyle:     "APA",
		QualityMode:       mode,
		TargetWords:       300,
		MaterialDir:       "fixtures/quality-mini/materials",
		AllowOnlineSearch: false,
	}
	if _, err := store.WriteJSON(s.RequirementsPath(), req, store.Overwrite); err != nil {
		return err
	}

	matResult, err := materials.ProcessDir(req.MaterialDir, s)
	if err != nil {
		return err
	}
	candidates := contracts.ReferenceCandidates{
		Items: references.AssignCandidateIDs(references.DedupeCandidates(matResult.Candidates), 1),
	}
	if _, err := store.WriteJSON(s.Path("references", "candidates.json"), candidates, store.Overwrite); err != nil {
		return err
	}
	var confirmIDs []string
	for _, c := range candidates.Items {
		if strings.Contains(c.DOI, "wearables-sleep") {
			continue // stays unconfirmed, same as the mock comparison
		}
		confirmIDs = append(confirmIDs, c.ID)
	}
	confirmation, err := references.ConfirmCandidates(s, candidates, references.ConfirmationDecision{
		ConfirmedIDs: confirmIDs, ConfirmedAt: time.Now().UTC(),
	})
	if err != nil {
		return err
	}
	refLines := make([]string, 0, len(confirmation.Confirmed.Items))
	keys := make([]string, 0, len(confirmation.Confirmed.Items))
	var chenKey, garciaKey string
	for _, ref := range confirmation.Confirmed.Items {
		refLines = append(refLines, fmt.Sprintf("- key=%s | %s (%d): %s", ref.Key, ref.Title, ref.Year, ref.Abstract))
		keys = append(keys, ref.Key)
		if ref.DOI == "10.1000/cbti-outcomes" {
			chenKey = ref.Key
		}
		if ref.DOI == "10.1000/digital-sleep" {
			garciaKey = ref.Key
		}
	}
	if chenKey == "" || garciaKey == "" {
		return fmt.Errorf("confirmed keys missing: %v", keys)
	}

	outline := map[string]any{
		"title_suggestion": "Insomnia Treatment Mini Review",
		"chapters": []map[string]any{
			{"chapter_id": chapter01, "title": "Therapist Guided CBT-I"},
			{"chapter_id": chapter02, "title": "Digital Delivery of Sleep Therapy"},
		},
	}
	if _, err := store.WriteJSON(s.Path("outline", "outline.json"), outline, store.Overwrite); err != nil {
		return err
	}

	materialText := readMaterials(s)

	evidenceDesc := ""
	if enhanced {
		snippetMat := ""
		for _, item := range matResult.Manifest.Items {
			if strings.Contains(item.Path, "digital_sleep") && item.Status == "parsed" {
				snippetMat = item.ID
			}
		}
		table := quality.EvidenceTable{
			GeneratedAt: time.Now().UTC(),
			Items: []quality.Evidence{
				{ID: "ev_001", ReferenceKey: chenKey, Depth: quality.DepthAbstract,
					Topics: []string{"cbt-i"}, KeyFindings: []string{"improved sleep onset latency in a randomized trial of 120 adults; effect persisted at six month follow up; sample excluded comorbid depression"},
					Confidence: "medium"},
				{ID: "ev_002", ReferenceKey: garciaKey, MaterialID: snippetMat, Depth: quality.DepthSnippet,
					Topics: []string{"digital sleep"}, KeyFindings: []string{"app delivered programs retain partial efficacy; effect sizes vary widely; most trials are short"},
					Excerpt:    "Reported effect sizes vary widely across studies, and most trials are short, so durability claims need stronger evidence.",
					Confidence: "medium"},
			},
		}
		if _, err := quality.SaveEvidenceTable(s, table); err != nil {
			return err
		}
		evidenceDesc = "Evidence table (the ONLY evidence ids you may bind):\n" +
			"- ev_001 (reference " + chenKey + ", depth=abstract): improved sleep onset latency, RCT of 120 adults, six month persistence, excluded comorbid depression\n" +
			"- ev_002 (reference " + garciaKey + ", depth=snippet): app delivered programs retain partial efficacy; effect sizes vary widely; most trials short\n"

		if err := realArchitectPlan(s, evidenceDesc); err != nil {
			return err
		}
	}

	chapters := []struct{ id, title string }{
		{chapter01, "Therapist Guided CBT-I"},
		{chapter02, "Digital Delivery of Sleep Therapy"},
	}
	for _, ch := range chapters {
		if err := realWriterChapter(s, ch.id, ch.title, materialText, refLines, evidenceDesc, enhanced); err != nil {
			return err
		}
		if enhanced {
			if _, _, err := quality.ExtractChapterClaimGraph(s, ch.id, 1, false, time.Now().UTC()); err != nil {
				return err
			}
		}
	}

	if enhanced {
		if err := realVerifier(s); err != nil {
			return err
		}
	}

	for _, ch := range chapters {
		review := contracts.Review{
			ChapterID: ch.id, DraftVersion: 1,
			Scores: contracts.ReviewScores{Overall: 86, CitationConsistency: 94, StructureLogic: 86, Coverage: 86, Readability: 86},
			Passed: true, UnsupportedClaims: []string{}, RequiredFixes: []string{}, OptionalFixes: []string{},
		}
		if _, err := artifacts.WriteReview(s, review, "mock-passed in both flows; comparison isolates the evidence protocol"); err != nil {
			return err
		}
		if _, err := artifacts.CommitAccepted(s, ch.id, 1, review); err != nil {
			return err
		}
	}

	input, err := finalexport.LoadInput(s)
	if err != nil {
		return err
	}
	result, err := finalexport.ExportFinal(s, input, finalexport.Options{Now: time.Now().UTC()})
	if err != nil {
		return err
	}
	fmt.Printf("flow=%s outputs=%d issues=%v metadata=%v\n", workDir, len(result.Outputs), result.Issues, result.Metadata)
	if input.Quality.Available {
		supported, unsupported := 0, 0
		for _, node := range input.Quality.ClaimGraph.Claims {
			switch node.Support {
			case quality.SupportSupported:
				supported++
			case quality.SupportUnsupported:
				unsupported++
			}
		}
		fmt.Printf("claims=%d supported=%d unsupported=%d gate=%s\n",
			len(input.Quality.ClaimGraph.Claims), supported, unsupported, input.Quality.GateOutcome.Conclusion)
	}
	return nil
}

func realArchitectPlan(s store.Store, evidenceDesc string) error {
	prompt := `You are the Architect of an academic mini review titled "Insomnia Treatment Mini Review" with chapters ch01 "Therapist Guided CBT-I" and ch02 "Digital Delivery of Sleep Therapy".
` + evidenceDesc + `
Produce a section quality plan. Return ONLY JSON:
{"sections":[{"chapter_id":"ch01","questions":["..."],"required_evidence_ids":["ev_001"],"boundaries":["..."],"forbidden_generalizations":["..."]},{"chapter_id":"ch02",...}]}
Rules: chapter_id must be ch01/ch02; required_evidence_ids only from ev_001/ev_002; 1-2 questions per chapter; boundaries state what the evidence cannot support.`
	var out planOut
	if err := askJSON(prompt, &out); err != nil {
		return err
	}
	plan := quality.SectionQualityPlan{GeneratedAt: time.Now().UTC()}
	for _, sec := range out.Sections {
		plan.Sections = append(plan.Sections, quality.SectionPlan{
			ChapterID: sec.ChapterID, Questions: sec.Questions,
			RequiredEvidenceIDs: sec.RequiredEvidenceIDs, Boundaries: sec.Boundaries,
			ForbiddenGeneralizations: sec.ForbiddenGeneralizations,
		})
	}
	_, err := quality.SaveSectionQualityPlan(s, plan)
	return err
}

func realWriterChapter(s store.Store, chapterID, title, materialText string, refLines []string, evidenceDesc string, enhanced bool) error {
	common := fmt.Sprintf(`You are the Writer of an academic mini review. Write chapter %s "%s" (120-160 words, English, APA style citations by reference key).
Source materials:
%s
Confirmed references (cite ONLY these keys):
%s
`, chapterID, title, materialText, strings.Join(refLines, "\n"))

	var protocol string
	if enhanced {
		plan, err := quality.LoadSectionQualityPlan(s)
		planDesc := ""
		if err == nil {
			for _, sec := range plan.Sections {
				if sec.ChapterID == chapterID {
					b, _ := json.Marshal(sec)
					planDesc = "Chapter quality plan (follow it): " + string(b) + "\n"
				}
			}
		}
		protocol = planDesc + evidenceDesc + `Evidence protocol (hard rules): every claim MUST include "evidence_ids" with at least one id from the evidence table above; only cite confirmed reference keys; if the evidence is too shallow for a strong conclusion, soften the claim and note the gap in writer_notes; never fabricate sources or evidence.
Return ONLY JSON: {"draft_markdown":"## ` + title + `\n\n...","claims":[{"id":"` + chapterID + `_claim_001","text":"...","importance":"high|medium","reference_keys":["..."],"evidence_ids":["ev_..."],"confidence":0.0}],"writer_notes":"..."}`
	} else {
		protocol = `Return ONLY JSON: {"draft_markdown":"## ` + title + `\n\n...","claims":[{"id":"` + chapterID + `_claim_001","text":"...","importance":"high|medium","reference_keys":["..."],"confidence":0.0}],"writer_notes":"..."}`
	}

	var out writerOut
	if err := askJSON(common+protocol, &out); err != nil {
		return err
	}
	bundle := artifacts.DraftBundle{
		ChapterID: chapterID, Version: 1,
		DraftMarkdown: out.DraftMarkdown,
		Claims:        contracts.ClaimsFile{ChapterID: chapterID, DraftVersion: 1},
		CitationMap:   contracts.CitationMap{ChapterID: chapterID, DraftVersion: 1},
		WriterNotes:   out.WriterNotes,
	}
	var claimIDs []string
	keySet := map[string]bool{}
	for _, c := range out.Claims {
		bundle.Claims.Claims = append(bundle.Claims.Claims, contracts.Claim{
			ID: c.ID, Text: c.Text, Importance: c.Importance,
			ReferenceKeys: c.ReferenceKeys, EvidenceIDs: c.EvidenceIDs, Confidence: c.Confidence,
		})
		claimIDs = append(claimIDs, c.ID)
		for _, k := range c.ReferenceKeys {
			keySet[k] = true
		}
	}
	var mappedKeys []string
	for k := range keySet {
		mappedKeys = append(mappedKeys, k)
	}
	bundle.CitationMap.Mappings = []contracts.CitationMapping{{
		ParagraphID: chapterID + "_p001", ClaimIDs: claimIDs, ReferenceKeys: mappedKeys,
	}}

	if enhanced {
		if _, err := aipagent.WriteGuardedDraftBundle(s, bundle); err != nil {
			fmt.Printf("  [%s] writer guard blocked: %v (recording, no retry)\n", chapterID, err)
			return err
		}
		fmt.Printf("  [%s] guarded write ok, claims=%d\n", chapterID, len(bundle.Claims.Claims))
		return nil
	}
	if _, err := artifacts.WriteDraftBundle(s, bundle); err != nil {
		return err
	}
	fmt.Printf("  [%s] legacy write ok, claims=%d\n", chapterID, len(bundle.Claims.Claims))
	return nil
}

func realVerifier(s store.Store) error {
	graph, err := quality.LoadClaimGraph(s)
	if err != nil {
		return err
	}
	table, err := quality.LoadEvidenceTable(s)
	if err != nil {
		return err
	}
	evidenceByID := map[string]quality.Evidence{}
	for _, ev := range table.Items {
		evidenceByID[ev.ID] = ev
	}
	var lines []string
	for _, node := range graph.Claims {
		var evDesc []string
		for _, id := range node.EvidenceIDs {
			ev := evidenceByID[id]
			evDesc = append(evDesc, fmt.Sprintf("%s(depth=%s: %s)", id, ev.Depth, strings.Join(ev.KeyFindings, "; ")))
		}
		lines = append(lines, fmt.Sprintf("- claim_id=%s chapter=%s text=%q evidence: %s",
			node.ID, node.ChapterID, node.Text, strings.Join(evDesc, " | ")))
	}
	prompt := `You are the Verifier of an academic review. For EACH claim below, judge whether its bound evidence actually supports it.
` + strings.Join(lines, "\n") + `
Rules: support must be one of supported|partially_supported|unsupported|overstated (uncertainty counts as unsupported); risk_level high for strong/absolute conclusions, else medium/low; note why in verifier_note.
Return ONLY JSON: {"verdicts":[{"claim_id":"claim_001","support":"...","risk_level":"...","verifier_note":"..."}]} with one verdict per claim, claim_id exactly as listed.`
	var out verdictOut
	if err := askJSON(prompt, &out); err != nil {
		return err
	}
	verdicts := make([]quality.ClaimVerdict, 0, len(out.Verdicts))
	for _, v := range out.Verdicts {
		verdicts = append(verdicts, quality.ClaimVerdict{
			ClaimID: v.ClaimID, Support: v.Support, RiskLevel: v.RiskLevel, VerifierNote: v.VerifierNote,
		})
	}
	_, _, _, err = quality.SaveVerificationResult(s, verdicts, time.Now().UTC())
	if err != nil {
		return err
	}
	fmt.Printf("  verifier verdicts=%d\n", len(verdicts))
	return nil
}

func readMaterials(s store.Store) string {
	entries, err := os.ReadDir(s.Path("materials", "extracted"))
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, entry := range entries {
		data, err := os.ReadFile(s.Path("materials", "extracted", entry.Name()))
		if err == nil {
			b.WriteString(string(data))
			b.WriteString("\n")
		}
	}
	return b.String()
}

// askJSON sends the prompt, parses the JSON reply into out, and retries once
// with the parse error appended.
func askJSON(prompt string, out any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	ask := prompt
	for attempt := 1; attempt <= 2; attempt++ {
		resp, err := chat.Generate(ctx, []agentcore.Message{agentcore.UserMsg(ask)}, nil)
		if err != nil {
			return err
		}
		text := strings.TrimSpace(resp.Message.TextContent())
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSuffix(text, "```")
		if i := strings.Index(text, "{"); i > 0 {
			text = text[i:]
		}
		if i := strings.LastIndex(text, "}"); i >= 0 {
			text = text[:i+1]
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(text)), out); err == nil {
			return nil
		} else if attempt == 2 {
			return fmt.Errorf("parse model JSON: %w; raw=%q", err, text)
		} else {
			ask = prompt + "\n\nYour previous reply was not valid JSON. Reply with ONLY the JSON object."
		}
	}
	return nil
}
