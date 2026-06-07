package artifacts

import (
	"fmt"
	"path"
	"regexp"
)

var chapterIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func DraftPath(chapterID string, version int) (string, error) {
	if err := validateChapterVersion(chapterID, version); err != nil {
		return "", err
	}
	return path.Join("drafts", chapterID, fmt.Sprintf("draft-v%d.md", version)), nil
}

func ClaimsPath(chapterID string, version int) (string, error) {
	if err := validateChapterVersion(chapterID, version); err != nil {
		return "", err
	}
	return path.Join("drafts", chapterID, fmt.Sprintf("claims-v%d.json", version)), nil
}

func CitationMapPath(chapterID string, version int) (string, error) {
	if err := validateChapterVersion(chapterID, version); err != nil {
		return "", err
	}
	return path.Join("drafts", chapterID, fmt.Sprintf("citation-map-v%d.json", version)), nil
}

func ReviewPath(chapterID string, version int) (string, error) {
	if err := validateChapterVersion(chapterID, version); err != nil {
		return "", err
	}
	return path.Join("drafts", chapterID, fmt.Sprintf("review-v%d.json", version)), nil
}

func ReviewMarkdownPath(chapterID string, version int) (string, error) {
	if err := validateChapterVersion(chapterID, version); err != nil {
		return "", err
	}
	return path.Join("drafts", chapterID, fmt.Sprintf("review-v%d.md", version)), nil
}

func WriterNotesPath(chapterID string) (string, error) {
	if err := validateChapterID(chapterID); err != nil {
		return "", err
	}
	return path.Join("drafts", chapterID, "writer_notes.md"), nil
}

func AcceptedPath(chapterID string) (string, error) {
	if err := validateChapterID(chapterID); err != nil {
		return "", err
	}
	return path.Join("drafts", chapterID, "accepted.md"), nil
}

func validateChapterVersion(chapterID string, version int) error {
	if err := validateChapterID(chapterID); err != nil {
		return err
	}
	if version <= 0 {
		return fmt.Errorf("draft version must be positive")
	}
	return nil
}

func validateChapterID(chapterID string) error {
	if chapterID == "" {
		return fmt.Errorf("chapter id is required")
	}
	if !chapterIDPattern.MatchString(chapterID) {
		return fmt.Errorf("chapter id %q contains unsafe characters", chapterID)
	}
	return nil
}
