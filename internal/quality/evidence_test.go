package quality

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

func newTestStore(t *testing.T) store.Store {
	t.Helper()
	return store.NewAt(t.TempDir())
}

func writeConfirmed(t *testing.T, s store.Store, keys ...string) {
	t.Helper()
	confirmed := contracts.ConfirmedReferences{Items: []contracts.ConfirmedReference{}}
	for _, key := range keys {
		confirmed.Items = append(confirmed.Items, contracts.ConfirmedReference{
			Key:               key,
			Title:             "Title of " + key,
			Authors:           []string{"Doe, J."},
			Year:              2024,
			SourceMaterialIDs: []string{},
			ConfirmedAt:       time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
		})
	}
	if _, err := store.WriteJSON(s.Path("references", "confirmed.json"), confirmed, store.Overwrite); err != nil {
		t.Fatalf("write confirmed.json: %v", err)
	}
}

func writeExtracted(t *testing.T, s store.Store, materialID, text string) {
	t.Helper()
	path := s.Path("materials", "extracted", materialID+".md")
	if _, err := store.WriteFile(path, []byte(text), store.Overwrite); err != nil {
		t.Fatalf("write extracted text: %v", err)
	}
}

func sampleTable() EvidenceTable {
	return EvidenceTable{
		GeneratedAt: time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC),
		Items: []Evidence{
			{
				ID:           "ev_001",
				ReferenceKey: "doe2024deep",
				Depth:        DepthAbstract,
				Topics:       []string{"deep learning"},
				KeyFindings:  []string{"finding one"},
				Confidence:   ConfidenceHigh,
			},
			{
				ID:           "ev_002",
				ReferenceKey: "smith2023survey",
				MaterialID:   "material_001",
				Depth:        DepthSnippet,
				Topics:       []string{"surveys"},
				KeyFindings:  []string{"finding two"},
				Limitations:  []string{"small sample"},
				Excerpt:      "quoted snippet text",
				Confidence:   ConfidenceMedium,
				Coverage:     "partial",
				RiskFlags:    []string{"single_source"},
			},
		},
	}
}

func TestSaveAndLoadEvidenceTableRoundTrip(t *testing.T) {
	s := newTestStore(t)
	writeConfirmed(t, s, "doe2024deep", "smith2023survey")
	writeExtracted(t, s, "material_001", "extracted full text\n")

	table := sampleTable()
	outputs, err := SaveEvidenceTable(s, table)
	if err != nil {
		t.Fatalf("SaveEvidenceTable() error = %v", err)
	}
	if len(outputs) != 2 || outputs[0] != EvidenceTableJSONRel || outputs[1] != EvidenceTableMarkdownRel {
		t.Fatalf("outputs = %#v", outputs)
	}
	for _, rel := range outputs {
		if _, err := os.Stat(s.Path(filepath.FromSlash(rel))); err != nil {
			t.Fatalf("stat %s: %v", rel, err)
		}
	}

	loaded, err := LoadEvidenceTable(s)
	if err != nil {
		t.Fatalf("LoadEvidenceTable() error = %v", err)
	}
	if !loaded.GeneratedAt.Equal(table.GeneratedAt) {
		t.Fatalf("generated_at = %v, want %v", loaded.GeneratedAt, table.GeneratedAt)
	}
	if len(loaded.Items) != 2 || loaded.Items[1].Excerpt != "quoted snippet text" {
		t.Fatalf("loaded items = %#v", loaded.Items)
	}
}

func TestSaveEvidenceTableRejectsUnconfirmedReference(t *testing.T) {
	s := newTestStore(t)
	writeConfirmed(t, s, "doe2024deep")

	table := sampleTable()
	table.Items = table.Items[:1]
	table.Items[0].ReferenceKey = "unknown2020key"

	_, err := SaveEvidenceTable(s, table)
	qErr, ok := AsError(err)
	if !ok || qErr.Code != CodeEvidenceUnconfirmedReference {
		t.Fatalf("error = %v, want code %s", err, CodeEvidenceUnconfirmedReference)
	}
	if _, statErr := os.Stat(EvidenceTableJSONPath(s)); !os.IsNotExist(statErr) {
		t.Fatalf("evidence-table.json should not be written on validation failure")
	}
}

func TestSaveEvidenceTableRejectsMissingConfirmedFile(t *testing.T) {
	s := newTestStore(t)
	table := sampleTable()
	table.Items = table.Items[:1]

	_, err := SaveEvidenceTable(s, table)
	qErr, ok := AsError(err)
	if !ok || qErr.Code != CodeEvidenceUnconfirmedReference {
		t.Fatalf("error = %v, want code %s", err, CodeEvidenceUnconfirmedReference)
	}
}

func TestSaveEvidenceTableDepthRequiresExtractedText(t *testing.T) {
	s := newTestStore(t)
	writeConfirmed(t, s, "doe2024deep", "smith2023survey")

	cases := []struct {
		name   string
		mutate func(*Evidence)
	}{
		{"missing extracted file", func(ev *Evidence) {}},
		{"no material id", func(ev *Evidence) { ev.MaterialID = "" }},
		{"fulltext depth without text", func(ev *Evidence) { ev.Depth = DepthFulltextExcerpt }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			table := sampleTable()
			tc.mutate(&table.Items[1])
			_, err := SaveEvidenceTable(s, table)
			qErr, ok := AsError(err)
			if !ok || qErr.Code != CodeEvidenceDepthUnsupported {
				t.Fatalf("error = %v, want code %s", err, CodeEvidenceDepthUnsupported)
			}
		})
	}

	t.Run("empty extracted file rejected", func(t *testing.T) {
		writeExtracted(t, s, "material_001", "")
		_, err := SaveEvidenceTable(s, sampleTable())
		qErr, ok := AsError(err)
		if !ok || qErr.Code != CodeEvidenceDepthUnsupported {
			t.Fatalf("error = %v, want code %s", err, CodeEvidenceDepthUnsupported)
		}
	})

	t.Run("non-empty extracted file accepted", func(t *testing.T) {
		writeExtracted(t, s, "material_001", "real text\n")
		if _, err := SaveEvidenceTable(s, sampleTable()); err != nil {
			t.Fatalf("SaveEvidenceTable() error = %v", err)
		}
	})
}

func TestValidateEvidenceTableSchemaRules(t *testing.T) {
	s := newTestStore(t)
	writeConfirmed(t, s, "doe2024deep")

	base := func() EvidenceTable {
		table := sampleTable()
		table.Items = table.Items[:1]
		return table
	}

	cases := []struct {
		name     string
		mutate   func(*EvidenceTable)
		wantCode string
	}{
		{"zero generated_at", func(tb *EvidenceTable) { tb.GeneratedAt = time.Time{} }, CodeEvidenceInvalid},
		{"bad id style", func(tb *EvidenceTable) { tb.Items[0].ID = "evidence-1" }, CodeEvidenceInvalid},
		{"empty reference key", func(tb *EvidenceTable) { tb.Items[0].ReferenceKey = "" }, CodeEvidenceInvalid},
		{"invalid depth", func(tb *EvidenceTable) { tb.Items[0].Depth = "fulltext" }, CodeEvidenceInvalid},
		{"invalid confidence", func(tb *EvidenceTable) { tb.Items[0].Confidence = "certain" }, CodeEvidenceInvalid},
		{"excerpt below snippet depth", func(tb *EvidenceTable) { tb.Items[0].Excerpt = "not allowed" }, CodeEvidenceInvalid},
		{"duplicate id", func(tb *EvidenceTable) {
			dup := tb.Items[0]
			tb.Items = append(tb.Items, dup)
		}, CodeEvidenceDuplicateID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			table := base()
			tc.mutate(&table)
			err := ValidateEvidenceTable(s, table)
			qErr, ok := AsError(err)
			if !ok || qErr.Code != tc.wantCode {
				t.Fatalf("error = %v, want code %s", err, tc.wantCode)
			}
		})
	}
}

func TestLoadEvidenceTableStrictAndMissing(t *testing.T) {
	s := newTestStore(t)

	_, err := LoadEvidenceTable(s)
	qErr, ok := AsError(err)
	if !ok || qErr.Code != CodeEvidenceIOFailed {
		t.Fatalf("missing file error = %v, want code %s", err, CodeEvidenceIOFailed)
	}

	raw := []byte("{\n  \"generated_at\": \"2026-06-10T12:00:00Z\",\n  \"items\": [],\n  \"unknown_field\": true\n}\n")
	if _, err := store.WriteFile(EvidenceTableJSONPath(s), raw, store.Overwrite); err != nil {
		t.Fatalf("write raw json: %v", err)
	}
	_, err = LoadEvidenceTable(s)
	qErr, ok = AsError(err)
	if !ok || qErr.Code != CodeEvidenceIOFailed || !strings.Contains(qErr.Message, "unknown_field") {
		t.Fatalf("strict read error = %v, want unknown field rejection", err)
	}
}

func TestFormatEvidenceTableMarkdownSnapshot(t *testing.T) {
	got := FormatEvidenceTableMarkdown(sampleTable())
	want := strings.Join([]string{
		"# Evidence Table",
		"",
		"- Generated at: 2026-06-10T12:00:00Z",
		"",
		"## ev_001",
		"",
		"- Reference key: doe2024deep",
		"- Depth: abstract",
		"- Confidence: high",
		"- Topics: deep learning",
		"- Key findings:",
		"  - finding one",
		"",
		"## ev_002",
		"",
		"- Reference key: smith2023survey",
		"- Material: material_001",
		"- Depth: snippet",
		"- Confidence: medium",
		"- Coverage: partial",
		"- Topics: surveys",
		"- Risk flags: single_source",
		"- Key findings:",
		"  - finding two",
		"- Limitations:",
		"  - small sample",
		"",
		"> quoted snippet text",
		"",
	}, "\n") + "\n"
	if got != want {
		t.Fatalf("markdown snapshot mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFormatEvidenceTableMarkdownEmpty(t *testing.T) {
	got := FormatEvidenceTableMarkdown(EvidenceTable{GeneratedAt: time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)})
	if !strings.Contains(got, "No evidence items.") {
		t.Fatalf("empty markdown = %q", got)
	}
}

func execTool(t *testing.T, tool interface {
	Execute(context.Context, json.RawMessage) (json.RawMessage, error)
}, args string) ToolResponse {
	t.Helper()
	raw, err := tool.Execute(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("tool execute error = %v", err)
	}
	var resp ToolResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal tool response: %v", err)
	}
	return resp
}

func TestSaveEvidenceTableTool(t *testing.T) {
	s := newTestStore(t)
	writeConfirmed(t, s, "doe2024deep")

	tool := NewSaveEvidenceTableTool(s)
	if tool.Name() != "save_evidence_table" {
		t.Fatalf("tool name = %s", tool.Name())
	}

	okArgs := `{"table":{"generated_at":"2026-06-10T12:00:00Z","items":[{"id":"ev_001","reference_key":"doe2024deep","depth":"abstract","topics":["t"],"key_findings":["f"],"confidence":"high"}]}}`
	resp := execTool(t, tool, okArgs)
	if !resp.OK {
		t.Fatalf("save tool response = %#v", resp)
	}

	badArgs := `{"table":{"generated_at":"2026-06-10T12:00:00Z","items":[{"id":"ev_001","reference_key":"nope2020","depth":"abstract","topics":[],"key_findings":[],"confidence":"low"}]}}`
	resp = execTool(t, tool, badArgs)
	if resp.OK || resp.Error == nil || resp.Error.Code != CodeEvidenceUnconfirmedReference {
		t.Fatalf("unconfirmed response = %#v", resp)
	}

	resp = execTool(t, tool, `{"not_table":1}`)
	if resp.OK || resp.Error == nil || resp.Error.Code != CodeEvidenceInvalid {
		t.Fatalf("invalid args response = %#v", resp)
	}
}

func TestLoadEvidenceTableTool(t *testing.T) {
	s := newTestStore(t)
	tool := NewLoadEvidenceTableTool(s)

	resp := execTool(t, tool, `{}`)
	if resp.OK || resp.Error == nil || resp.Error.Code != CodeEvidenceIOFailed {
		t.Fatalf("missing table response = %#v", resp)
	}

	writeConfirmed(t, s, "doe2024deep")
	table := sampleTable()
	table.Items = table.Items[:1]
	if _, err := SaveEvidenceTable(s, table); err != nil {
		t.Fatalf("SaveEvidenceTable() error = %v", err)
	}

	resp = execTool(t, tool, `{}`)
	if !resp.OK {
		t.Fatalf("load tool response = %#v", resp)
	}
	data, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}
	var loaded EvidenceTable
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal table: %v", err)
	}
	if len(loaded.Items) != 1 || loaded.Items[0].ID != "ev_001" {
		t.Fatalf("loaded via tool = %#v", loaded)
	}
}

func TestToolsRegistry(t *testing.T) {
	s := newTestStore(t)
	tools := Tools(s)
	if len(tools) != 4 {
		t.Fatalf("Tools() len = %d", len(tools))
	}
	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name()] = true
	}
	for _, want := range []string{
		"save_evidence_table", "load_evidence_table",
		"save_section_quality_plan", "load_section_quality_plan",
	} {
		if !names[want] {
			t.Fatalf("tool %s missing, names = %#v", want, names)
		}
	}
}
