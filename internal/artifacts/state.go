package artifacts

import (
	"fmt"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
)

type ChapterState struct {
	ChapterID      string `json:"chapter_id"`
	Status         string `json:"status"`
	DraftVersion   int    `json:"draft_version"`
	RevisionRounds int    `json:"revision_rounds"`
}

func NewChapterState(chapterID string) (ChapterState, error) {
	if err := validateChapterID(chapterID); err != nil {
		return ChapterState{}, err
	}
	return ChapterState{ChapterID: chapterID, Status: StatusDrafting, DraftVersion: 1}, nil
}

func (s ChapterState) AfterDraft(version int) (ChapterState, error) {
	if err := s.validate(); err != nil {
		return ChapterState{}, err
	}
	if version <= 0 {
		return ChapterState{}, fmt.Errorf("draft version must be positive")
	}
	s.DraftVersion = version
	s.Status = StatusReviewing
	return s, nil
}

func (s ChapterState) AfterReview(review contracts.Review) (ChapterState, GateResult, error) {
	if err := s.validate(); err != nil {
		return ChapterState{}, GateResult{}, err
	}
	if review.ChapterID != s.ChapterID || review.DraftVersion != s.DraftVersion {
		return ChapterState{}, GateResult{}, fmt.Errorf("review metadata does not match chapter state")
	}
	gate := StatusAfterReview(review, s.RevisionRounds)
	s.Status = gate.Status
	if gate.Status == StatusRevisionRequired {
		s.RevisionRounds++
	}
	return s, gate, nil
}

func (s ChapterState) NextDraft() (ChapterState, error) {
	if err := s.validate(); err != nil {
		return ChapterState{}, err
	}
	if s.Status != StatusRevisionRequired {
		return ChapterState{}, fmt.Errorf("next draft requires status %s, got %s", StatusRevisionRequired, s.Status)
	}
	s.DraftVersion++
	s.Status = StatusDrafting
	return s, nil
}

func (s ChapterState) Commit() (ChapterState, error) {
	if err := s.validate(); err != nil {
		return ChapterState{}, err
	}
	if s.Status != StatusAccepted {
		return ChapterState{}, fmt.Errorf("commit requires status %s, got %s", StatusAccepted, s.Status)
	}
	s.Status = StatusCommitted
	return s, nil
}

func (s ChapterState) validate() error {
	if err := validateChapterID(s.ChapterID); err != nil {
		return err
	}
	if s.DraftVersion <= 0 {
		return fmt.Errorf("draft version must be positive")
	}
	if s.RevisionRounds < 0 {
		return fmt.Errorf("revision rounds cannot be negative")
	}
	return nil
}
