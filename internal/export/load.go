package export

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/artifacts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/store"
)

var draftFilePattern = regexp.MustCompile(`^draft-v([1-9][0-9]*)\.md$`)

type outlineFile struct {
	TitleSuggestion string           `json:"title_suggestion"`
	Chapters        []outlineChapter `json:"chapters"`
}

type outlineChapter struct {
	ChapterID string `json:"chapter_id"`
	Title     string `json:"title"`
}

func LoadInput(s store.Store) (ExportInput, error) {
	var input ExportInput
	loadRequirements(s, &input)
	loadRun(s, &input)
	outline := loadOutline(s, &input)

	if err := store.ReadJSON(s.Path("references", "confirmed.json"), &input.ConfirmedReferences); err != nil {
		return ExportInput{}, err
	}

	chapters, err := loadAcceptedChapters(s, outline)
	if err != nil {
		return ExportInput{}, err
	}
	input.Chapters = chapters
	return input, nil
}

func loadRequirements(s store.Store, input *ExportInput) {
	var req contracts.Requirements
	if err := store.ReadJSON(s.RequirementsPath(), &req); err != nil {
		return
	}
	input.Title = strings.TrimSpace(req.Topic)
	input.CitationStyle = strings.TrimSpace(req.CitationStyle)
}

func loadRun(s store.Store, input *ExportInput) {
	var run contracts.Run
	if err := store.ReadJSON(s.RunPath(), &run); err != nil {
		return
	}
	input.CostEstimate = run.CostEstimate
}

func loadOutline(s store.Store, input *ExportInput) outlineFile {
	var outline outlineFile
	data, err := os.ReadFile(s.Path("outline", "outline.json"))
	if err != nil {
		return outlineFile{}
	}
	if err := json.Unmarshal(data, &outline); err != nil {
		return outlineFile{}
	}
	if strings.TrimSpace(outline.TitleSuggestion) != "" {
		input.Title = strings.TrimSpace(outline.TitleSuggestion)
	}
	return outline
}

func loadAcceptedChapters(s store.Store, outline outlineFile) ([]ChapterInput, error) {
	draftsDir := s.Path("drafts")
	entries, err := os.ReadDir(draftsDir)
	if os.IsNotExist(err) {
		return nil, Error{Code: CodeNoAcceptedChapters, Message: "drafts directory does not exist"}
	}
	if err != nil {
		return nil, err
	}

	titleByID := map[string]string{}
	orderByID := map[string]int{}
	for i, chapter := range outline.Chapters {
		titleByID[chapter.ChapterID] = strings.TrimSpace(chapter.Title)
		orderByID[chapter.ChapterID] = i
	}

	var chapters []ChapterInput
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		chapterID := entry.Name()
		acceptedRel, err := artifacts.AcceptedPath(chapterID)
		if err != nil {
			continue
		}
		acceptedPath := s.Path(filepath.FromSlash(acceptedRel))
		accepted, err := os.ReadFile(acceptedPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}

		version, err := matchAcceptedVersion(s, chapterID, accepted)
		if err != nil {
			return nil, err
		}
		chapter, err := loadChapterVersion(s, chapterID, version, string(accepted), titleByID[chapterID])
		if err != nil {
			return nil, err
		}
		chapters = append(chapters, chapter)
	}

	if len(chapters) == 0 {
		return nil, Error{Code: CodeNoAcceptedChapters, Message: "no accepted chapters found"}
	}
	sort.SliceStable(chapters, func(i, j int) bool {
		oi, iok := orderByID[chapters[i].ID]
		oj, jok := orderByID[chapters[j].ID]
		if iok && jok {
			return oi < oj
		}
		if iok != jok {
			return iok
		}
		return chapters[i].ID < chapters[j].ID
	})
	return chapters, nil
}

func matchAcceptedVersion(s store.Store, chapterID string, accepted []byte) (int, error) {
	dir := s.Path("drafts", chapterID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	acceptedSum := store.SHA256(accepted)
	var versions []int
	byVersion := map[int]string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := draftFilePattern.FindStringSubmatch(entry.Name())
		if len(matches) != 2 {
			continue
		}
		version, err := strconv.Atoi(matches[1])
		if err != nil {
			continue
		}
		versions = append(versions, version)
		byVersion[version] = entry.Name()
	}
	sort.Sort(sort.Reverse(sort.IntSlice(versions)))
	for _, version := range versions {
		data, err := os.ReadFile(filepath.Join(dir, byVersion[version]))
		if err != nil {
			return 0, err
		}
		if store.SHA256(data) == acceptedSum {
			return version, nil
		}
	}
	return 0, fmt.Errorf("accepted chapter %s does not match any draft-vN.md", chapterID)
}

func loadChapterVersion(s store.Store, chapterID string, version int, acceptedMarkdown, title string) (ChapterInput, error) {
	claimsRel, err := artifacts.ClaimsPath(chapterID, version)
	if err != nil {
		return ChapterInput{}, err
	}
	citationRel, err := artifacts.CitationMapPath(chapterID, version)
	if err != nil {
		return ChapterInput{}, err
	}
	reviewRel, err := artifacts.ReviewPath(chapterID, version)
	if err != nil {
		return ChapterInput{}, err
	}

	var claims contracts.ClaimsFile
	if err := store.ReadJSON(s.Path(filepath.FromSlash(claimsRel)), &claims); err != nil {
		return ChapterInput{}, err
	}
	var citationMap contracts.CitationMap
	if err := store.ReadJSON(s.Path(filepath.FromSlash(citationRel)), &citationMap); err != nil {
		return ChapterInput{}, err
	}
	var review contracts.Review
	if err := store.ReadJSON(s.Path(filepath.FromSlash(reviewRel)), &review); err != nil {
		return ChapterInput{}, err
	}
	if strings.TrimSpace(title) == "" {
		title = chapterID
	}
	return ChapterInput{
		ID:               chapterID,
		Title:            title,
		Version:          version,
		Status:           artifacts.StatusCommitted,
		AcceptedMarkdown: acceptedMarkdown,
		Claims:           claims,
		CitationMap:      citationMap,
		Review:           review,
	}, nil
}
