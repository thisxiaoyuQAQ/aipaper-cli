package requirements

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
)

func TestModelBuildsValidatedRequirements(t *testing.T) {
	dir := t.TempDir()
	model := NewModel(contracts.Requirements{
		Topic:             "RAG reviews",
		ResearchQuestions: []string{"How should references be confirmed?"},
		Scope:             "mini review",
		Language:          "en",
		TargetWords:       800,
		MaterialDir:       dir,
		AllowOnlineSearch: true,
		SearchProviders:   []string{"semantic_scholar", "crossref"},
	})

	req, err := model.Requirements()
	if err != nil {
		t.Fatalf("Requirements() error = %v", err)
	}
	if req.CitationStyle != "gbt7714" || req.ArticleTemplate != "zh_course_paper" || len(req.SearchProviders) != 2 {
		t.Fatalf("requirements = %#v", req)
	}

	model = model.UpdateKey("enter")
	if !model.Done() || model.Err() != nil {
		t.Fatalf("model done=%v err=%v", model.Done(), model.Err())
	}
}

func TestModelRejectsInvalidFields(t *testing.T) {
	model := NewModel(contracts.Requirements{
		Topic:       "RAG reviews",
		Language:    "en",
		TargetWords: 0,
		MaterialDir: filepath.Join(t.TempDir(), "missing"),
	})

	_, err := model.Requirements()
	if err == nil || !strings.Contains(err.Error(), "目标字数") {
		t.Fatalf("error = %v", err)
	}
	model = model.SetField(FieldTargetWords, "500")
	_, err = model.Requirements()
	if err == nil || !strings.Contains(err.Error(), "材料目录") {
		t.Fatalf("error = %v", err)
	}
}

func TestModelAcceptsReviewPaperTemplate(t *testing.T) {
	model := NewModel(contracts.Requirements{MaterialDir: t.TempDir(), Language: "zh-CN", TargetWords: 1200})
	model = model.SetField(FieldTopic, "原神与鸣潮比较")
	model = model.SetField(FieldArticleTemplate, "review_paper")

	req, err := model.Requirements()
	if err != nil {
		t.Fatalf("Requirements() error = %v", err)
	}
	if req.ArticleTemplate != "review_paper" || req.CitationStyle != "gbt7714" {
		t.Fatalf("requirements = %#v", req)
	}
}

func TestModelRejectsInvalidArticleTemplate(t *testing.T) {
	model := NewModel(contracts.Requirements{MaterialDir: t.TempDir(), Language: "zh-CN", TargetWords: 1200})
	model = model.SetField(FieldTopic, "模板校验")
	model = model.SetField(FieldArticleTemplate, "review-paper")

	_, err := model.Requirements()
	if err == nil || !strings.Contains(err.Error(), "文章模板") {
		t.Fatalf("error = %v", err)
	}
}

func TestModelNavigationEditAndToggle(t *testing.T) {
	model := NewModel(contracts.Requirements{MaterialDir: t.TempDir(), Language: "en", TargetWords: 1})
	model = model.SetField(FieldTopic, "")
	model = model.UpdateKey("r")
	model = model.UpdateKey("a")
	model = model.UpdateKey("g")
	if model.FieldValue(FieldTopic) != "rag" {
		t.Fatalf("topic = %q", model.FieldValue(FieldTopic))
	}
	model = model.UpdateKey("backspace")
	if model.FieldValue(FieldTopic) != "ra" {
		t.Fatalf("topic after backspace = %q", model.FieldValue(FieldTopic))
	}

	for model.CurrentField() != FieldAllowOnlineSearch {
		model = model.UpdateKey("tab")
	}
	if model.FieldValue(FieldAllowOnlineSearch) != "false" {
		t.Fatalf("initial allow online = %q", model.FieldValue(FieldAllowOnlineSearch))
	}
	model = model.UpdateKey("space")
	if model.FieldValue(FieldAllowOnlineSearch) != "true" {
		t.Fatalf("toggled allow online = %q", model.FieldValue(FieldAllowOnlineSearch))
	}
	if !strings.Contains(model.View(), "写作需求") {
		t.Fatalf("view = %s", model.View())
	}
}
