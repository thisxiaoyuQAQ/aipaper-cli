package materials

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

func TestProcessDirWritesManifestArtifactsAndBibTeXCandidates(t *testing.T) {
	materialDir := filepath.Join("..", "..", "fixtures", "materials")
	s := store.NewAt(filepath.Join(t.TempDir(), "store"))

	result, err := ProcessDir(materialDir, s)
	if err != nil {
		t.Fatalf("ProcessDir() error = %v", err)
	}
	if len(result.Manifest.Items) != 7 {
		t.Fatalf("manifest item len = %d", len(result.Manifest.Items))
	}
	if len(result.Candidates) != 3 {
		t.Fatalf("candidates = %#v", result.Candidates)
	}
	if result.Candidates[0].ID != "cand_001" || result.Candidates[0].Source != "bibtex" {
		t.Fatalf("candidate = %#v", result.Candidates[0])
	}

	items := itemsByPath(result.Manifest)
	assertItem(t, items["link.url"], "material_001", "url", StatusParsed, "url_basic", true)
	assertItem(t, items["notes.md"], "material_002", "markdown", StatusParsed, "markdown", false)
	assertItem(t, items["plain.txt"], "material_003", "txt", StatusParsed, "txt", false)
	assertItem(t, items["references.bib"], "material_004", "bibtex", StatusParsed, "bibtex", false)
	assertItem(t, items["sample.pdf"], "material_005", "pdf", StatusParsed, "pdf_text", false)
	assertItem(t, items["table.csv"], "material_006", "csv", StatusParsed, "csv_basic", true)
	assertItem(t, items["unsupported.bin"], "material_007", "unknown", StatusSkipped, "", false)

	var manifest contracts.MaterialManifest
	if err := store.ReadJSON(s.Path("materials", "manifest.json"), &manifest); err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if len(manifest.Items) != len(result.Manifest.Items) {
		t.Fatalf("written manifest len = %d", len(manifest.Items))
	}

	notes := readString(t, s.Path(filepath.FromSlash(items["notes.md"].OutputText)))
	if !strings.Contains(notes, "Retrieval-augmented generation") {
		t.Fatalf("notes extracted text = %q", notes)
	}
	pdfText := readString(t, s.Path(filepath.FromSlash(items["sample.pdf"].OutputText)))
	if !strings.Contains(pdfText, "PDF Fixture Text") {
		t.Fatalf("pdf extracted text = %q", pdfText)
	}
	var meta ParsedMeta
	if err := store.ReadJSON(s.Path(filepath.FromSlash(items["references.bib"].OutputMeta)), &meta); err != nil {
		t.Fatalf("read bibtex meta: %v", err)
	}
	if len(meta.References) != 1 || meta.Fields["entry_count"].(float64) != 1 {
		t.Fatalf("bibtex meta = %#v", meta)
	}
}

func TestProcessDirHandlesMissingEmptyAndFailedMaterials(t *testing.T) {
	missingStore := store.NewAt(filepath.Join(t.TempDir(), "missing-store"))
	missing, err := ProcessDir(filepath.Join(t.TempDir(), "does-not-exist"), missingStore)
	if err != nil {
		t.Fatalf("missing ProcessDir() error = %v", err)
	}
	if len(missing.Manifest.Items) != 1 || missing.Manifest.Items[0].Status != StatusFailed {
		t.Fatalf("missing manifest = %#v", missing.Manifest)
	}
	if err := store.ReadJSON(missingStore.Path("materials", "manifest.json"), &contracts.MaterialManifest{}); err != nil {
		t.Fatalf("missing manifest was not written: %v", err)
	}

	emptyDir := t.TempDir()
	emptyStore := store.NewAt(filepath.Join(t.TempDir(), "empty-store"))
	empty, err := ProcessDir(emptyDir, emptyStore)
	if err != nil {
		t.Fatalf("empty ProcessDir() error = %v", err)
	}
	if len(empty.Manifest.Items) != 0 {
		t.Fatalf("empty manifest = %#v", empty.Manifest)
	}

	badDir := t.TempDir()
	writeText(t, filepath.Join(badDir, "bad.bib"), `@article{bad, title = {unterminated}`)
	bad, err := ProcessDir(badDir, store.NewAt(filepath.Join(t.TempDir(), "bad-store")))
	if err != nil {
		t.Fatalf("bad ProcessDir() error = %v", err)
	}
	if len(bad.Manifest.Items) != 1 || bad.Manifest.Items[0].Status != StatusFailed {
		t.Fatalf("bad manifest = %#v", bad.Manifest)
	}
}

func TestProcessDirParsesRISAndChineseCSVExports(t *testing.T) {
	dir := t.TempDir()
	writeText(t, filepath.Join(dir, "cnki.ris"), "TY  - JOUR\nTI  - 中文文献检索研究\nAU  - 张三\nPY  - 2024\nJO  - 情报杂志\nDO  - 10.1000/cnki\nUR  - https://example.cn/cnki\nAB  - 摘要\nER  - \n")
	writeText(t, filepath.Join(dir, "wanfang.csv"), "题名,作者,年份,来源,DOI,链接,摘要\n国内文献数据库研究,李四；王五,2023,图书情报工作,10.1000/wanfang,https://example.cn/wanfang,中文摘要\n")

	result, err := ProcessDir(dir, store.NewAt(filepath.Join(t.TempDir(), "store")))
	if err != nil {
		t.Fatalf("ProcessDir() error = %v", err)
	}
	if len(result.Candidates) != 2 {
		t.Fatalf("candidates = %#v", result.Candidates)
	}
	if got := result.Candidates[0]; got.Source != "ris" || got.Title != "中文文献检索研究" || got.Reliability != "user_export" {
		t.Fatalf("ris candidate = %#v", got)
	}
	if got := result.Candidates[1]; got.Source != "csv_export" || got.Title != "国内文献数据库研究" || got.Venue != "图书情报工作" || len(got.Authors) != 2 {
		t.Fatalf("csv candidate = %#v", got)
	}

	items := itemsByPath(result.Manifest)
	assertItem(t, items["cnki.ris"], "material_001", "ris", StatusParsed, "ris", false)
	assertItem(t, items["wanfang.csv"], "material_002", "csv", StatusParsed, "csv_basic", true)
}

func TestProcessDirMarksDOCXAsDegraded(t *testing.T) {
	dir := t.TempDir()
	docxPath := filepath.Join(dir, "notes.docx")
	writeMinimalDOCX(t, docxPath, "DOCX fixture text")

	result, err := ProcessDir(dir, store.NewAt(filepath.Join(t.TempDir(), "store")))
	if err != nil {
		t.Fatalf("ProcessDir() error = %v", err)
	}
	if len(result.Manifest.Items) != 1 {
		t.Fatalf("manifest = %#v", result.Manifest)
	}
	item := result.Manifest.Items[0]
	assertItem(t, item, "material_001", "docx", StatusParsed, "docx_basic", true)
}

func TestChunkTextSplitsLongText(t *testing.T) {
	chunks := chunkText(strings.Repeat("a", 9), "material_001", 4)
	if len(chunks) != 3 {
		t.Fatalf("chunks = %#v", chunks)
	}
	if chunks[0].ID != "material_001_chunk_001" || chunks[2].Text != "a" {
		t.Fatalf("chunks = %#v", chunks)
	}
}

func itemsByPath(manifest contracts.MaterialManifest) map[string]contracts.MaterialItem {
	items := map[string]contracts.MaterialItem{}
	for _, item := range manifest.Items {
		items[item.Path] = item
	}
	return items
}

func assertItem(t *testing.T, item contracts.MaterialItem, id, kind, status, parser string, degraded bool) {
	t.Helper()
	if item.ID != id || item.Kind != kind || item.Status != status || item.Parser != parser || item.Degraded != degraded {
		t.Fatalf("item = %#v, want id=%s kind=%s status=%s parser=%s degraded=%v", item, id, kind, status, parser, degraded)
	}
	if status == StatusParsed && (item.OutputText == "" || item.OutputMeta == "") {
		t.Fatalf("parsed item missing outputs: %#v", item)
	}
}

func readString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return string(data)
}

func writeText(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func writeMinimalDOCX(t *testing.T, path, text string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer file.Close()
	zw := zip.NewWriter(file)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatalf("zip Create() error = %v", err)
	}
	if _, err := w.Write([]byte(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>` + text + `</w:t></w:r></w:p></w:body></w:document>`)); err != nil {
		t.Fatalf("zip Write() error = %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip Close() error = %v", err)
	}
}
