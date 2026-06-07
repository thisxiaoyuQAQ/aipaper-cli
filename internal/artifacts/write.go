package artifacts

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/checkpoint"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

type DraftBundle struct {
	ChapterID     string
	Version       int
	DraftMarkdown string
	Claims        contracts.ClaimsFile
	CitationMap   contracts.CitationMap
	WriterNotes   string
}

type WriteResult struct {
	Outputs []checkpoint.OutputArtifact
	Paths   []string
}

func WriteDraftBundle(s store.Store, bundle DraftBundle) (WriteResult, error) {
	if err := validateDraftBundle(bundle); err != nil {
		return WriteResult{}, err
	}
	var result WriteResult
	if err := writeTextArtifact(s, DraftPath, bundle.ChapterID, bundle.Version, bundle.DraftMarkdown, "draft", &result); err != nil {
		return WriteResult{}, err
	}
	if err := writeJSONArtifact(s, ClaimsPath, bundle.ChapterID, bundle.Version, bundle.Claims, "claims", &result); err != nil {
		return WriteResult{}, err
	}
	if err := writeJSONArtifact(s, CitationMapPath, bundle.ChapterID, bundle.Version, bundle.CitationMap, "citation_map", &result); err != nil {
		return WriteResult{}, err
	}
	if bundle.WriterNotes != "" {
		rel, err := WriterNotesPath(bundle.ChapterID)
		if err != nil {
			return WriteResult{}, err
		}
		res, err := store.WriteFile(s.Path(filepath.FromSlash(rel)), []byte(ensureTrailingNewline(bundle.WriterNotes)), store.Overwrite)
		if err != nil {
			return WriteResult{}, err
		}
		result.add("writer_notes", rel, res.SHA256)
	}
	return result, nil
}

func WriteReview(s store.Store, review contracts.Review, markdown string) (WriteResult, error) {
	if err := validateReview(review); err != nil {
		return WriteResult{}, err
	}
	var result WriteResult
	if err := writeJSONArtifact(s, ReviewPath, review.ChapterID, review.DraftVersion, review, "review", &result); err != nil {
		return WriteResult{}, err
	}
	if markdown != "" {
		if err := writeTextArtifact(s, ReviewMarkdownPath, review.ChapterID, review.DraftVersion, markdown, "review_markdown", &result); err != nil {
			return WriteResult{}, err
		}
	}
	return result, nil
}

func CommitAccepted(s store.Store, chapterID string, version int, review contracts.Review) (checkpoint.OutputArtifact, error) {
	if review.ChapterID != chapterID || review.DraftVersion != version {
		return checkpoint.OutputArtifact{}, fmt.Errorf("review metadata does not match accepted chapter target")
	}
	gate := EvaluateReview(review)
	if !gate.Passed {
		return checkpoint.OutputArtifact{}, fmt.Errorf("chapter %s v%d is not accepted: %v", chapterID, version, gate.Reasons)
	}
	draftRel, err := DraftPath(chapterID, version)
	if err != nil {
		return checkpoint.OutputArtifact{}, err
	}
	acceptedRel, err := AcceptedPath(chapterID)
	if err != nil {
		return checkpoint.OutputArtifact{}, err
	}
	data, err := os.ReadFile(s.Path(filepath.FromSlash(draftRel)))
	if err != nil {
		return checkpoint.OutputArtifact{}, err
	}
	res, err := store.WriteFile(s.Path(filepath.FromSlash(acceptedRel)), data, store.CreateOnly)
	if err != nil {
		return checkpoint.OutputArtifact{}, err
	}
	return checkpoint.OutputArtifact{Kind: "accepted_chapter", Path: acceptedRel, SHA256: res.SHA256}, nil
}

func validateDraftBundle(bundle DraftBundle) error {
	if err := validateChapterVersion(bundle.ChapterID, bundle.Version); err != nil {
		return err
	}
	if bundle.DraftMarkdown == "" {
		return fmt.Errorf("draft markdown is required")
	}
	if bundle.Claims.ChapterID != bundle.ChapterID || bundle.Claims.DraftVersion != bundle.Version {
		return fmt.Errorf("claims metadata does not match draft bundle")
	}
	if bundle.CitationMap.ChapterID != bundle.ChapterID || bundle.CitationMap.DraftVersion != bundle.Version {
		return fmt.Errorf("citation map metadata does not match draft bundle")
	}
	return nil
}

func validateReview(review contracts.Review) error {
	return validateChapterVersion(review.ChapterID, review.DraftVersion)
}

type versionedPathFunc func(string, int) (string, error)

func writeTextArtifact(s store.Store, pathFn versionedPathFunc, chapterID string, version int, data, kind string, result *WriteResult) error {
	rel, err := pathFn(chapterID, version)
	if err != nil {
		return err
	}
	res, err := store.WriteFile(s.Path(filepath.FromSlash(rel)), []byte(ensureTrailingNewline(data)), store.CreateOnly)
	if err != nil {
		return err
	}
	result.add(kind, rel, res.SHA256)
	return nil
}

func writeJSONArtifact(s store.Store, pathFn versionedPathFunc, chapterID string, version int, value any, kind string, result *WriteResult) error {
	rel, err := pathFn(chapterID, version)
	if err != nil {
		return err
	}
	res, err := store.WriteJSON(s.Path(filepath.FromSlash(rel)), value, store.CreateOnly)
	if err != nil {
		return err
	}
	result.add(kind, rel, res.SHA256)
	return nil
}

func (r *WriteResult) add(kind, relPath, sha string) {
	r.Outputs = append(r.Outputs, checkpoint.OutputArtifact{Kind: kind, Path: relPath, SHA256: sha})
	r.Paths = append(r.Paths, relPath)
}

func ensureTrailingNewline(s string) string {
	if s == "" || s[len(s)-1] == '\n' {
		return s
	}
	return s + "\n"
}
