package references

import "testing"

func TestParseBibTeXAndBuildCandidates(t *testing.T) {
	data := []byte(`@article{smith2024rag,
  title = {Retrieval {Augmented} Generation for Review Writing},
  author = {Smith, Jane and Li, Wei},
  year = {2024},
  journal = {Journal of AI Writing},
  doi = {10.1000/RAG-Review},
  abstract = "A fixture abstract."
}`)

	entries, err := ParseBibTeX(data)
	if err != nil {
		t.Fatalf("ParseBibTeX() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries len = %d", len(entries))
	}
	entry := entries[0]
	if entry.Type != "article" || entry.Key != "smith2024rag" {
		t.Fatalf("entry = %#v", entry)
	}
	if entry.Fields["title"] != "Retrieval {Augmented} Generation for Review Writing" {
		t.Fatalf("title = %q", entry.Fields["title"])
	}

	candidates := CandidatesFromBibTeX(entries, 7)
	if len(candidates) != 1 {
		t.Fatalf("candidates len = %d", len(candidates))
	}
	candidate := candidates[0]
	if candidate.ID != "cand_007" || candidate.Source != "bibtex" || candidate.Status != "pending" {
		t.Fatalf("candidate = %#v", candidate)
	}
	if candidate.Year != 2024 || candidate.DedupeGroup != "doi:10.1000/rag-review" {
		t.Fatalf("candidate year/dedupe = %#v", candidate)
	}
	if len(candidate.Authors) != 2 || candidate.Authors[0] != "Smith, Jane" {
		t.Fatalf("authors = %#v", candidate.Authors)
	}
}

func TestParseBibTeXRejectsMalformedEntry(t *testing.T) {
	_, err := ParseBibTeX([]byte(`@article{bad, title = {unterminated}`))
	if err == nil {
		t.Fatalf("ParseBibTeX() succeeded for malformed entry")
	}
}
