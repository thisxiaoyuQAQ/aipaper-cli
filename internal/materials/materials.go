package materials

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/references"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

const (
	StatusParsed  = "parsed"
	StatusFailed  = "failed"
	StatusSkipped = "skipped"
)

type Result struct {
	Manifest   contracts.MaterialManifest
	Candidates []contracts.ReferenceCandidate
	Outputs    []string
}

type ParsedMeta struct {
	ID         string                         `json:"id"`
	SourcePath string                         `json:"source_path"`
	Kind       string                         `json:"kind"`
	Parser     string                         `json:"parser"`
	Degraded   bool                           `json:"degraded"`
	Chunks     []TextChunk                    `json:"chunks,omitempty"`
	References []contracts.ReferenceCandidate `json:"references,omitempty"`
	Fields     map[string]any                 `json:"fields,omitempty"`
}

type TextChunk struct {
	ID    string `json:"id"`
	Start int    `json:"start"`
	End   int    `json:"end"`
	Text  string `json:"text"`
}

type parsedFile struct {
	kind       string
	parser     string
	degraded   bool
	text       string
	fields     map[string]any
	candidates []contracts.ReferenceCandidate
}

func ProcessDir(materialDir string, s store.Store) (Result, error) {
	if err := store.EnsureLayout(s); err != nil {
		return Result{}, err
	}
	files, err := listFiles(materialDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			result := Result{
				Manifest: contracts.MaterialManifest{Items: []contracts.MaterialItem{{
					ID:       "material_001",
					Path:     filepath.ToSlash(filepath.Clean(materialDir)),
					Kind:     "unknown",
					Status:   StatusFailed,
					Degraded: false,
					Error:    "material directory does not exist",
				}}},
			}
			if err := writeManifest(s, result.Manifest, &result); err != nil {
				return result, err
			}
			return result, nil
		}
		return Result{}, err
	}

	result := Result{Manifest: contracts.MaterialManifest{Items: []contracts.MaterialItem{}}}
	for i, path := range files {
		id := fmt.Sprintf("material_%03d", i+1)
		relPath := relTo(materialDir, path)
		item := contracts.MaterialItem{
			ID:       id,
			Path:     relPath,
			Kind:     kindFromPath(path),
			Degraded: isDegradedKind(kindFromPath(path)),
		}

		parsed, err := parseMaterial(path, len(result.Candidates)+1)
		if err != nil {
			if errors.Is(err, errUnsupportedFormat) {
				item.Status = StatusSkipped
				item.Error = "unsupported material format"
			} else {
				item.Status = StatusFailed
				item.Parser = parsed.parser
				item.Error = err.Error()
			}
			result.Manifest.Items = append(result.Manifest.Items, item)
			continue
		}

		textRel := s.Rel("materials", "extracted", id+".md")
		metaRel := s.Rel("materials", "parsed", id+".json")
		meta := ParsedMeta{
			ID:         id,
			SourcePath: relPath,
			Kind:       parsed.kind,
			Parser:     parsed.parser,
			Degraded:   parsed.degraded,
			Chunks:     chunkText(parsed.text, id, 4000),
			References: parsed.candidates,
			Fields:     parsed.fields,
		}
		if res, err := store.WriteFile(s.Path(filepath.FromSlash(textRel)), []byte(ensureTrailingNewline(parsed.text)), store.Overwrite); err != nil {
			return result, err
		} else {
			result.Outputs = append(result.Outputs, res.Path)
		}
		if res, err := store.WriteJSON(s.Path(filepath.FromSlash(metaRel)), meta, store.Overwrite); err != nil {
			return result, err
		} else {
			result.Outputs = append(result.Outputs, res.Path)
		}

		item.Kind = parsed.kind
		item.Status = StatusParsed
		item.Parser = parsed.parser
		item.Degraded = parsed.degraded
		item.OutputText = textRel
		item.OutputMeta = metaRel
		result.Candidates = append(result.Candidates, parsed.candidates...)
		result.Manifest.Items = append(result.Manifest.Items, item)
	}

	if err := writeManifest(s, result.Manifest, &result); err != nil {
		return result, err
	}
	return result, nil
}

func writeManifest(s store.Store, manifest contracts.MaterialManifest, result *Result) error {
	res, err := store.WriteJSON(s.Path("materials", "manifest.json"), manifest, store.Overwrite)
	if err != nil {
		return err
	}
	result.Outputs = append(result.Outputs, res.Path)
	return nil
}

func listFiles(materialDir string) ([]string, error) {
	info, err := os.Stat(materialDir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("material path is not a directory: %s", materialDir)
	}
	var files []string
	err = filepath.WalkDir(materialDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool {
		return filepath.ToSlash(files[i]) < filepath.ToSlash(files[j])
	})
	return files, nil
}

func parseMaterial(path string, candidateStart int) (parsedFile, error) {
	kind := kindFromPath(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return parsedFile{kind: kind, parser: parserForKind(kind)}, err
	}
	switch kind {
	case "markdown":
		return parsedFile{kind: kind, parser: "markdown", text: string(data)}, nil
	case "txt":
		return parsedFile{kind: kind, parser: "txt", text: string(data)}, nil
	case "bibtex":
		entries, err := references.ParseBibTeX(data)
		if err != nil {
			return parsedFile{kind: kind, parser: "bibtex"}, err
		}
		candidates := references.CandidatesFromBibTeX(entries, candidateStart)
		return parsedFile{
			kind:       kind,
			parser:     "bibtex",
			text:       formatBibTeXSummary(entries),
			fields:     map[string]any{"entry_count": len(entries), "entries": entries},
			candidates: candidates,
		}, nil
	case "pdf":
		text, err := extractPDFText(data)
		if err != nil {
			return parsedFile{kind: kind, parser: "pdf_text"}, err
		}
		return parsedFile{kind: kind, parser: "pdf_text", text: text}, nil
	case "docx":
		text, err := extractDOCXText(path)
		if err != nil {
			return parsedFile{kind: kind, parser: "docx_basic", degraded: true}, err
		}
		return parsedFile{kind: kind, parser: "docx_basic", degraded: true, text: text}, nil
	case "url":
		text, fields, err := parseURLMaterial(data)
		if err != nil {
			return parsedFile{kind: kind, parser: "url_basic", degraded: true}, err
		}
		return parsedFile{kind: kind, parser: "url_basic", degraded: true, text: text, fields: fields}, nil
	case "csv":
		text, fields, err := parseCSVMaterial(data)
		if err != nil {
			return parsedFile{kind: kind, parser: "csv_basic", degraded: true}, err
		}
		return parsedFile{kind: kind, parser: "csv_basic", degraded: true, text: text, fields: fields}, nil
	default:
		return parsedFile{kind: kind}, errUnsupportedFormat
	}
}

func relTo(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(filepath.Clean(path))
	}
	return filepath.ToSlash(rel)
}

func kindFromPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown":
		return "markdown"
	case ".txt":
		return "txt"
	case ".bib", ".bibtex":
		return "bibtex"
	case ".pdf":
		return "pdf"
	case ".docx":
		return "docx"
	case ".url", ".webloc":
		return "url"
	case ".csv":
		return "csv"
	default:
		return "unknown"
	}
}

func parserForKind(kind string) string {
	switch kind {
	case "markdown", "txt", "bibtex":
		return kind
	case "pdf":
		return "pdf_text"
	case "docx":
		return "docx_basic"
	case "url":
		return "url_basic"
	case "csv":
		return "csv_basic"
	default:
		return ""
	}
}

func isDegradedKind(kind string) bool {
	return kind == "docx" || kind == "url" || kind == "csv"
}

func chunkText(text, materialID string, size int) []TextChunk {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	if size <= 0 {
		size = 4000
	}
	runes := []rune(text)
	chunks := make([]TextChunk, 0, (len(runes)/size)+1)
	for start := 0; start < len(runes); start += size {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, TextChunk{
			ID:    fmt.Sprintf("%s_chunk_%03d", materialID, len(chunks)+1),
			Start: start,
			End:   end,
			Text:  string(runes[start:end]),
		})
	}
	return chunks
}

func ensureTrailingNewline(text string) string {
	if strings.HasSuffix(text, "\n") {
		return text
	}
	return text + "\n"
}

var errUnsupportedFormat = errors.New("unsupported material format")
