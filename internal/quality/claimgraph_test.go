package quality

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

func setupClaimGraphStore(t *testing.T) store.Store {
	t.Helper()
	s := store.NewAt(t.TempDir())
	confirmed := contracts.ConfirmedReferences{Items: []contracts.ConfirmedReference{
		{
			Key:               "doe2024deep",
			Title:             "Deep Survey",
			Authors:           []string{"Doe, J."},
			Year:              2024,
			SourceMaterialIDs: []string{},
			ConfirmedAt:       time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
		},
		{
			Key:               "lee2023graph",
			Title:             "Graph Methods",
			Authors:           []string{"Lee, K."},
			Year:              2023,
			SourceMaterialIDs: []string{},
			ConfirmedAt:       time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
		},
	}}
	if _, err := store.WriteJSON(s.Path("references", "confirmed.json"), confirmed, store.Overwrite); err != nil {
		t.Fatalf("write confirmed.json: %v", err)
	}
	outline := map[string]any{
		"title_suggestion": "A Survey",
		"chapters": []map[string]any{
			{"chapter_id": "ch01", "title": "Intro"},
			{"chapter_id": "ch02", "title": "Methods"},
		},
	}
	if _, err := store.WriteJSON(s.Path("outline", "outline.json"), outline, store.Overwrite); err != nil {
		t.Fatalf("write outline.json: %v", err)
	}
	table := EvidenceTable{
		GeneratedAt: time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC),
		Items: []Evidence{
			{ID: "ev_001", ReferenceKey: "doe2024deep", Depth: DepthAbstract, Topics: []string{"t"}, KeyFindings: []string{"f"}, Confidence: ConfidenceHigh},
			{ID: "ev_002", ReferenceKey: "lee2023graph", Depth: DepthAbstract, Topics: []string{"t"}, KeyFindings: []string{"f"}, Confidence: ConfidenceMedium},
		},
	}
	if _, err := SaveEvidenceTable(s, table); err != nil {
		t.Fatalf("save evidence table: %v", err)
	}
	return s
}

func validClaimGraph() ClaimGraph {
	return ClaimGraph{
		UpdatedAt: time.Date(2026, 6, 10, 13, 0, 0, 0, time.UTC),
		Claims: []ClaimNode{
			{
				ID:            "claim_001",
				Text:          "Deep models improve survey coverage.",
				ChapterID:     "ch01",
				ReferenceKeys: []string{"doe2024deep"},
				EvidenceIDs:   []string{"ev_001"},
			},
		},
	}
}

func writeChapterClaimArtifacts(t *testing.T, s store.Store, chapterID string, version int, claims contracts.ClaimsFile, citationMap contracts.CitationMap) {
	t.Helper()
	claimsPath := s.Path("drafts", chapterID, fmt.Sprintf("claims-v%d.json", version))
	if _, err := store.WriteJSON(claimsPath, claims, store.Overwrite); err != nil {
		t.Fatalf("write claims: %v", err)
	}
	citationPath := s.Path("drafts", chapterID, fmt.Sprintf("citation-map-v%d.json", version))
	if _, err := store.WriteJSON(citationPath, citationMap, store.Overwrite); err != nil {
		t.Fatalf("write citation map: %v", err)
	}
}

func chapterClaims(chapterID string, version int, claims ...contracts.Claim) contracts.ClaimsFile {
	return contracts.ClaimsFile{ChapterID: chapterID, DraftVersion: version, Claims: claims}
}

func chapterCitationMap(chapterID string, version int, mappings ...contracts.CitationMapping) contracts.CitationMap {
	return contracts.CitationMap{ChapterID: chapterID, DraftVersion: version, Mappings: mappings}
}

func TestSaveAndLoadClaimGraphRoundTrip(t *testing.T) {
	s := setupClaimGraphStore(t)
	graph := validClaimGraph()
	outputs, err := SaveClaimGraph(s, graph)
	if err != nil {
		t.Fatalf("SaveClaimGraph() error = %v", err)
	}
	want := []string{ClaimGraphJSONRel, ClaimGraphMarkdownRel}
	for i, rel := range want {
		if outputs[i] != rel {
			t.Fatalf("outputs = %v, want %v", outputs, want)
		}
	}
	for _, path := range []string{ClaimGraphJSONPath(s), ClaimGraphMarkdownPath(s)} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("output %s missing: %v", path, err)
		}
	}
	loaded, err := LoadClaimGraph(s)
	if err != nil {
		t.Fatalf("LoadClaimGraph() error = %v", err)
	}
	if len(loaded.Claims) != 1 || loaded.Claims[0].ID != "claim_001" {
		t.Fatalf("loaded graph = %#v", loaded)
	}
}

func TestLoadClaimGraphRejectsUnknownFields(t *testing.T) {
	s := setupClaimGraphStore(t)
	raw := `{"updated_at":"2026-06-10T13:00:00Z","claims":[],"unexpected":true}`
	if _, err := store.WriteFile(ClaimGraphJSONPath(s), []byte(raw), store.Overwrite); err != nil {
		t.Fatalf("write raw graph: %v", err)
	}
	_, err := LoadClaimGraph(s)
	assertQualityError(t, err, CodeClaimGraphIOFailed)
}

func TestValidateClaimGraphFailures(t *testing.T) {
	s := setupClaimGraphStore(t)
	cases := []struct {
		name     string
		mutate   func(*ClaimGraph)
		wantCode string
	}{
		{"missing updated_at", func(g *ClaimGraph) { g.UpdatedAt = time.Time{} }, CodeClaimGraphInvalid},
		{"bad id style", func(g *ClaimGraph) { g.Claims[0].ID = "c1" }, CodeClaimGraphInvalid},
		{"duplicate id", func(g *ClaimGraph) {
			g.Claims = append(g.Claims, g.Claims[0])
		}, CodeClaimGraphDuplicateID},
		{"empty text", func(g *ClaimGraph) { g.Claims[0].Text = "" }, CodeClaimGraphInvalid},
		{"empty chapter", func(g *ClaimGraph) { g.Claims[0].ChapterID = "" }, CodeClaimGraphInvalid},
		{"unknown chapter", func(g *ClaimGraph) { g.Claims[0].ChapterID = "ch99" }, CodeClaimGraphUnknownChapter},
		{"invalid support", func(g *ClaimGraph) { g.Claims[0].Support = "definitely" }, CodeClaimGraphInvalid},
		{"invalid risk", func(g *ClaimGraph) { g.Claims[0].RiskLevel = "extreme" }, CodeClaimGraphInvalid},
		{"unconfirmed reference", func(g *ClaimGraph) {
			g.Claims[0].ReferenceKeys = []string{"fake2020key"}
		}, CodeClaimGraphUnconfirmedReference},
		{"unknown evidence", func(g *ClaimGraph) {
			g.Claims[0].EvidenceIDs = []string{"ev_999"}
		}, CodeClaimGraphUnknownEvidence},
		{"unknown duplicate_of", func(g *ClaimGraph) {
			g.Claims[0].DuplicateOf = []string{"claim_404"}
		}, CodeClaimGraphInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			graph := validClaimGraph()
			tc.mutate(&graph)
			assertQualityError(t, ValidateClaimGraph(s, graph), tc.wantCode)
		})
	}
}

func TestValidateClaimGraphAllowsVerificationStates(t *testing.T) {
	s := setupClaimGraphStore(t)
	graph := validClaimGraph()
	for _, support := range []string{"", SupportSupported, SupportPartiallySupported, SupportUnsupported, SupportOverstated, SupportSkipped} {
		graph.Claims[0].Support = support
		if err := ValidateClaimGraph(s, graph); err != nil {
			t.Fatalf("support %q rejected: %v", support, err)
		}
	}
}

func TestMergeChapterClaimsIsIncremental(t *testing.T) {
	updated := time.Date(2026, 6, 10, 14, 0, 0, 0, time.UTC)
	graph := ClaimGraph{
		UpdatedAt: time.Date(2026, 6, 10, 13, 0, 0, 0, time.UTC),
		Claims: []ClaimNode{
			{ID: "claim_001", Text: "Existing chapter one claim.", ChapterID: "ch01"},
			{ID: "claim_002", Text: "Old chapter two claim to be replaced.", ChapterID: "ch02"},
		},
	}
	merged := MergeChapterClaims(graph, "ch02", []ClaimNode{
		{SourceClaimID: "c1", Text: "Fresh chapter two claim.", ChapterID: "ch02"},
	}, updated)

	if merged.UpdatedAt != updated {
		t.Fatalf("updated_at = %v", merged.UpdatedAt)
	}
	if len(merged.Claims) != 2 {
		t.Fatalf("claims = %#v", merged.Claims)
	}
	// ch01 untouched, ch02 replaced with a fresh id continuing after claim_002.
	if merged.Claims[0].ID != "claim_001" || merged.Claims[0].ChapterID != "ch01" {
		t.Fatalf("kept claim = %#v", merged.Claims[0])
	}
	if merged.Claims[1].ID != "claim_002" || merged.Claims[1].Text != "Fresh chapter two claim." {
		t.Fatalf("merged claim = %#v", merged.Claims[1])
	}
}

func TestMergeChapterClaimsContinuesIDsAfterOtherChapters(t *testing.T) {
	graph := ClaimGraph{
		UpdatedAt: time.Date(2026, 6, 10, 13, 0, 0, 0, time.UTC),
		Claims: []ClaimNode{
			{ID: "claim_007", Text: "Chapter one claim.", ChapterID: "ch01"},
		},
	}
	merged := MergeChapterClaims(graph, "ch02", []ClaimNode{
		{Text: "New chapter two claim."},
	}, time.Date(2026, 6, 10, 14, 0, 0, 0, time.UTC))
	if merged.Claims[1].ID != "claim_008" || merged.Claims[1].ChapterID != "ch02" {
		t.Fatalf("new claim = %#v", merged.Claims[1])
	}
}

func TestMergeChapterClaimsMarksCrossChapterDuplicates(t *testing.T) {
	graph := ClaimGraph{
		UpdatedAt: time.Date(2026, 6, 10, 13, 0, 0, 0, time.UTC),
		Claims: []ClaimNode{
			{ID: "claim_001", Text: "Transformer models substantially improve text classification accuracy.", ChapterID: "ch01"},
		},
	}
	merged := MergeChapterClaims(graph, "ch02", []ClaimNode{
		{Text: "Transformer models substantially improve text classification accuracy!"},
		{Text: "Graph neural networks are an entirely different unrelated topic of study."},
	}, time.Date(2026, 6, 10, 14, 0, 0, 0, time.UTC))

	dup := merged.Claims[1]
	if len(dup.DuplicateOf) != 1 || dup.DuplicateOf[0] != "claim_001" {
		t.Fatalf("duplicate flags = %#v", dup)
	}
	if dup.RiskLevel != RiskLow {
		t.Fatalf("duplicate risk = %q, want graded low risk", dup.RiskLevel)
	}
	unique := merged.Claims[2]
	if len(unique.DuplicateOf) != 0 || unique.RiskLevel != "" {
		t.Fatalf("unique claim flagged: %#v", unique)
	}
}

func TestMergeChapterClaimsDoesNotFlagSameChapterDuplicates(t *testing.T) {
	merged := MergeChapterClaims(ClaimGraph{}, "ch01", []ClaimNode{
		{Text: "Deep models improve survey coverage across domains."},
		{Text: "Deep models improve survey coverage across domains."},
	}, time.Date(2026, 6, 10, 14, 0, 0, 0, time.UTC))
	for _, node := range merged.Claims {
		if len(node.DuplicateOf) != 0 {
			t.Fatalf("same-chapter duplicate flagged: %#v", node)
		}
	}
}

func TestBuildChapterClaimNodesProjectsClaimsAndCitationMap(t *testing.T) {
	claims := chapterClaims("ch01", 1,
		contracts.Claim{ID: "c1", Text: "Claim one.", Importance: "high", ReferenceKeys: []string{"doe2024deep"}, EvidenceIDs: []string{"ev_001"}},
		contracts.Claim{ID: "c2", Text: "Claim two.", Importance: "medium", ReferenceKeys: []string{}, EvidenceIDs: []string{"ev_002"}},
	)
	citationMap := chapterCitationMap("ch01", 1, contracts.CitationMapping{
		ParagraphID:   "p1",
		ClaimIDs:      []string{"c1", "c2"},
		ReferenceKeys: []string{"lee2023graph", "doe2024deep"},
	})
	nodes := BuildChapterClaimNodes(claims, citationMap, false)
	if len(nodes) != 2 {
		t.Fatalf("nodes = %#v", nodes)
	}
	// c1 keeps its own key first, then gains the mapped key without duplicates.
	if got := strings.Join(nodes[0].ReferenceKeys, ","); got != "doe2024deep,lee2023graph" {
		t.Fatalf("c1 reference keys = %q", got)
	}
	if nodes[0].SourceClaimID != "c1" || nodes[0].Support != "" {
		t.Fatalf("c1 node = %#v", nodes[0])
	}
	if got := strings.Join(nodes[1].ReferenceKeys, ","); got != "lee2023graph,doe2024deep" {
		t.Fatalf("c2 reference keys = %q", got)
	}
}

func TestBuildChapterClaimNodesFastModeMarksSkipped(t *testing.T) {
	claims := chapterClaims("ch01", 1, contracts.Claim{ID: "c1", Text: "Claim one.", Importance: "high"})
	nodes := BuildChapterClaimNodes(claims, chapterCitationMap("ch01", 1), true)
	if nodes[0].Support != SupportSkipped {
		t.Fatalf("fast mode support = %q", nodes[0].Support)
	}
}

func TestExtractChapterClaimGraphIncrementalUpdateAndRecovery(t *testing.T) {
	s := setupClaimGraphStore(t)
	now := time.Date(2026, 6, 10, 14, 0, 0, 0, time.UTC)

	writeChapterClaimArtifacts(t, s, "ch01", 1,
		chapterClaims("ch01", 1, contracts.Claim{ID: "c1", Text: "Deep models improve survey coverage.", Importance: "high", ReferenceKeys: []string{"doe2024deep"}, EvidenceIDs: []string{"ev_001"}}),
		chapterCitationMap("ch01", 1, contracts.CitationMapping{ParagraphID: "p1", ClaimIDs: []string{"c1"}, ReferenceKeys: []string{"doe2024deep"}}),
	)
	graph, outputs, err := ExtractChapterClaimGraph(s, "ch01", 1, false, now)
	if err != nil {
		t.Fatalf("extract ch01 error = %v", err)
	}
	if len(outputs) != 2 || len(graph.Claims) != 1 || graph.Claims[0].ID != "claim_001" {
		t.Fatalf("first extraction = %#v outputs %v", graph, outputs)
	}

	// Simulated interruption: a fresh extraction call loads the persisted graph
	// and merges the next chapter without touching ch01.
	writeChapterClaimArtifacts(t, s, "ch02", 1,
		chapterClaims("ch02", 1, contracts.Claim{ID: "c1", Text: "Graph methods complement deep models.", Importance: "medium", ReferenceKeys: []string{"lee2023graph"}, EvidenceIDs: []string{"ev_002"}}),
		chapterCitationMap("ch02", 1, contracts.CitationMapping{ParagraphID: "p1", ClaimIDs: []string{"c1"}, ReferenceKeys: []string{"lee2023graph"}}),
	)
	graph, _, err = ExtractChapterClaimGraph(s, "ch02", 1, false, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("extract ch02 error = %v", err)
	}
	if len(graph.Claims) != 2 {
		t.Fatalf("merged graph = %#v", graph.Claims)
	}
	if graph.Claims[0].ChapterID != "ch01" || graph.Claims[1].ChapterID != "ch02" || graph.Claims[1].ID != "claim_002" {
		t.Fatalf("merged order = %#v", graph.Claims)
	}

	// Re-running ch01 (recovery replay) replaces only ch01 nodes; ch02 survives.
	graph, _, err = ExtractChapterClaimGraph(s, "ch01", 1, false, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("re-extract ch01 error = %v", err)
	}
	if len(graph.Claims) != 2 {
		t.Fatalf("replayed graph = %#v", graph.Claims)
	}
	chapters := map[string]bool{}
	for _, node := range graph.Claims {
		chapters[node.ChapterID] = true
	}
	if !chapters["ch01"] || !chapters["ch02"] {
		t.Fatalf("replayed chapters = %#v", graph.Claims)
	}
	loaded, err := LoadClaimGraph(s)
	if err != nil || len(loaded.Claims) != 2 {
		t.Fatalf("persisted graph after replay = %#v err %v", loaded, err)
	}
}

func TestExtractChapterClaimGraphRejectsInvalidProjection(t *testing.T) {
	s := setupClaimGraphStore(t)
	now := time.Date(2026, 6, 10, 14, 0, 0, 0, time.UTC)

	// Unconfirmed reference key inside claims.json must be rejected by the
	// graph validation even though the chapter artifacts exist on disk.
	writeChapterClaimArtifacts(t, s, "ch01", 1,
		chapterClaims("ch01", 1, contracts.Claim{ID: "c1", Text: "Bad claim.", Importance: "high", ReferenceKeys: []string{"fake2020key"}, EvidenceIDs: []string{"ev_001"}}),
		chapterCitationMap("ch01", 1, contracts.CitationMapping{ParagraphID: "p1", ClaimIDs: []string{"c1"}, ReferenceKeys: []string{"fake2020key"}}),
	)
	_, _, err := ExtractChapterClaimGraph(s, "ch01", 1, false, now)
	assertQualityError(t, err, CodeClaimGraphUnconfirmedReference)

	// Unknown evidence id is rejected the same way.
	writeChapterClaimArtifacts(t, s, "ch02", 1,
		chapterClaims("ch02", 1, contracts.Claim{ID: "c1", Text: "Bad evidence.", Importance: "high", ReferenceKeys: []string{"doe2024deep"}, EvidenceIDs: []string{"ev_999"}}),
		chapterCitationMap("ch02", 1, contracts.CitationMapping{ParagraphID: "p1", ClaimIDs: []string{"c1"}, ReferenceKeys: []string{"doe2024deep"}}),
	)
	_, _, err = ExtractChapterClaimGraph(s, "ch02", 1, false, now)
	assertQualityError(t, err, CodeClaimGraphUnknownEvidence)

	// Missing chapter artifacts are an io failure.
	_, _, err = ExtractChapterClaimGraph(s, "ch01", 9, false, now)
	assertQualityError(t, err, CodeClaimGraphIOFailed)

	// Mismatched chapter metadata is invalid.
	writeChapterClaimArtifacts(t, s, "ch02", 2,
		chapterClaims("ch01", 2, contracts.Claim{ID: "c1", Text: "Wrong chapter.", Importance: "high"}),
		chapterCitationMap("ch01", 2),
	)
	_, _, err = ExtractChapterClaimGraph(s, "ch02", 2, false, now)
	assertQualityError(t, err, CodeClaimGraphInvalid)

	// Nothing was persisted by the failed extractions.
	if _, statErr := os.Stat(ClaimGraphJSONPath(s)); !os.IsNotExist(statErr) {
		t.Fatalf("claim graph persisted after failures: %v", statErr)
	}
}

func TestSaveClaimGraphToolValidatesAndPersists(t *testing.T) {
	s := setupClaimGraphStore(t)
	tool := NewSaveClaimGraphTool(s)

	graphJSON, err := json.Marshal(validClaimGraph())
	if err != nil {
		t.Fatalf("marshal graph: %v", err)
	}
	response := execTool(t, tool, `{"graph":`+string(graphJSON)+`}`)
	if !response.OK {
		t.Fatalf("save tool response = %#v", response)
	}

	bad := validClaimGraph()
	bad.Claims[0].ReferenceKeys = []string{"fake2020key"}
	badJSON, _ := json.Marshal(bad)
	response = execTool(t, tool, `{"graph":`+string(badJSON)+`}`)
	if response.OK || response.Error == nil || response.Error.Code != CodeClaimGraphUnconfirmedReference {
		t.Fatalf("bad graph response = %#v", response)
	}

	response = execTool(t, tool, `{"graph":{"updated_at":"2026-06-10T13:00:00Z","claims":[]},"extra":1}`)
	if response.OK || response.Error == nil || response.Error.Code != CodeClaimGraphInvalid {
		t.Fatalf("strict args response = %#v", response)
	}
}

func TestExtractChapterClaimsToolReportsDuplicates(t *testing.T) {
	s := setupClaimGraphStore(t)
	writeChapterClaimArtifacts(t, s, "ch01", 1,
		chapterClaims("ch01", 1, contracts.Claim{ID: "c1", Text: "Transformer models substantially improve text classification accuracy.", Importance: "high", ReferenceKeys: []string{"doe2024deep"}, EvidenceIDs: []string{"ev_001"}}),
		chapterCitationMap("ch01", 1, contracts.CitationMapping{ParagraphID: "p1", ClaimIDs: []string{"c1"}, ReferenceKeys: []string{"doe2024deep"}}),
	)
	writeChapterClaimArtifacts(t, s, "ch02", 1,
		chapterClaims("ch02", 1, contracts.Claim{ID: "c1", Text: "Transformer models substantially improve text classification accuracy.", Importance: "high", ReferenceKeys: []string{"lee2023graph"}, EvidenceIDs: []string{"ev_002"}}),
		chapterCitationMap("ch02", 1, contracts.CitationMapping{ParagraphID: "p1", ClaimIDs: []string{"c1"}, ReferenceKeys: []string{"lee2023graph"}}),
	)
	tool := NewExtractChapterClaimsTool(s)
	response := execTool(t, tool, `{"chapter_id":"ch01","draft_version":1}`)
	if !response.OK {
		t.Fatalf("extract ch01 response = %#v", response)
	}
	response = execTool(t, tool, `{"chapter_id":"ch02","draft_version":1,"fast_mode":true}`)
	if !response.OK {
		t.Fatalf("extract ch02 response = %#v", response)
	}
	data, ok := response.Data.(map[string]any)
	if !ok {
		t.Fatalf("response data = %#v", response.Data)
	}
	if data["duplicate_claims"] != float64(1) || data["total_claims"] != float64(2) {
		t.Fatalf("tool data = %#v", data)
	}
	graph, err := LoadClaimGraph(s)
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}
	var ch02 *ClaimNode
	for i := range graph.Claims {
		if graph.Claims[i].ChapterID == "ch02" {
			ch02 = &graph.Claims[i]
		}
	}
	if ch02 == nil || len(ch02.DuplicateOf) != 1 || ch02.Support != SupportSkipped {
		t.Fatalf("ch02 node = %#v", ch02)
	}
}

func TestLoadClaimGraphToolMissingFile(t *testing.T) {
	s := setupClaimGraphStore(t)
	response := execTool(t, NewLoadClaimGraphTool(s), `{}`)
	if response.OK || response.Error == nil || response.Error.Code != CodeClaimGraphIOFailed {
		t.Fatalf("missing graph response = %#v", response)
	}
}

func TestFormatClaimGraphMarkdown(t *testing.T) {
	graph := validClaimGraph()
	graph.Claims[0].Support = SupportSupported
	graph.Claims[0].RiskLevel = RiskLow
	graph.Claims[0].DuplicateOf = []string{"claim_777"}
	graph.Claims[0].NeedsRewrite = true
	md := FormatClaimGraphMarkdown(graph)
	for _, want := range []string{
		"# Claim Graph", "## claim_001", "- Chapter: ch01",
		"- Support: supported", "- Risk level: low",
		"- Duplicate of: claim_777", "- Needs rewrite: yes",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("markdown missing %q:\n%s", want, md)
		}
	}
	if !strings.Contains(FormatClaimGraphMarkdown(ClaimGraph{UpdatedAt: graph.UpdatedAt}), "No claims.") {
		t.Fatalf("empty graph markdown missing placeholder")
	}
}

func assertQualityError(t *testing.T, err error, wantCode string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error code %s, got nil", wantCode)
	}
	qErr, ok := AsError(err)
	if !ok {
		t.Fatalf("expected structured quality error, got %v", err)
	}
	if qErr.Code != wantCode {
		t.Fatalf("error code = %s, want %s (%v)", qErr.Code, wantCode, err)
	}
}
