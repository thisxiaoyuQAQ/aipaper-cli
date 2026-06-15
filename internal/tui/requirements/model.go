package requirements

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/contracts"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/i18n"
	"github.com/thisxiaoyuQAQ/aipaper-cli/internal/quality"
)

type Field int

const (
	FieldTopic Field = iota
	FieldResearchQuestions
	FieldScope
	FieldLanguage
	FieldCitationStyle
	FieldArticleTemplate
	FieldQualityMode
	FieldTargetWords
	FieldMaterialDir
	FieldAllowOnlineSearch
	FieldSearchProviders
	FieldChapterPreferences
	FieldConstraints
)

var orderedFields = []Field{
	FieldTopic,
	FieldResearchQuestions,
	FieldScope,
	FieldLanguage,
	FieldCitationStyle,
	FieldArticleTemplate,
	FieldQualityMode,
	FieldTargetWords,
	FieldMaterialDir,
	FieldAllowOnlineSearch,
	FieldSearchProviders,
	FieldChapterPreferences,
	FieldConstraints,
}

type Options struct {
	I18N i18n.T
}

type Model struct {
	values   map[Field]string
	cursor   int
	done     bool
	canceled bool
	err      error
	i18n     i18n.T
}

func NewModel(defaults contracts.Requirements, opts ...Options) Model {
	tr := i18n.New("")
	if len(opts) > 0 && !opts[0].I18N.IsZero() {
		tr = opts[0].I18N
	}
	qualityMode := defaults.QualityMode
	if qualityMode == "" {
		qualityMode = "enhanced"
	}
	values := map[Field]string{
		FieldTopic:              defaults.Topic,
		FieldResearchQuestions:  strings.Join(defaults.ResearchQuestions, "\n"),
		FieldScope:              defaults.Scope,
		FieldLanguage:           defaults.Language,
		FieldCitationStyle:      defaults.CitationStyle,
		FieldArticleTemplate:    defaults.ArticleTemplate,
		FieldQualityMode:        qualityMode,
		FieldTargetWords:        intString(defaults.TargetWords),
		FieldMaterialDir:        defaults.MaterialDir,
		FieldAllowOnlineSearch:  boolString(defaults.AllowOnlineSearch),
		FieldSearchProviders:    strings.Join(defaults.SearchProviders, ", "),
		FieldChapterPreferences: strings.Join(defaults.ChapterPreferences, "\n"),
		FieldConstraints:        strings.Join(defaults.Constraints, "\n"),
	}
	return Model{values: values, i18n: tr}
}

func (m Model) UpdateKey(key string) Model {
	key = normalizeKey(key)
	if m.done || m.canceled {
		return m
	}
	switch key {
	case "up", "shift+tab":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "tab":
		if m.cursor < len(orderedFields)-1 {
			m.cursor++
		}
	case "space":
		if m.CurrentField() == FieldAllowOnlineSearch {
			m.SetField(FieldAllowOnlineSearch, boolString(!parseBool(m.values[FieldAllowOnlineSearch])))
		} else {
			m.appendRune(' ')
		}
	case "backspace":
		m.backspace()
	case "enter":
		_, err := m.Requirements()
		if err != nil {
			m.err = err
			return m
		}
		m.done = true
		m.err = nil
	case "q", "ctrl+c", "esc":
		m.canceled = true
	default:
		if len([]rune(key)) == 1 {
			m.appendRune([]rune(key)[0])
		}
	}
	return m
}

func (m Model) SetField(field Field, value string) Model {
	if m.values == nil {
		m.values = map[Field]string{}
	}
	m.values[field] = value
	return m
}

func (m Model) Requirements() (contracts.Requirements, error) {
	targetWords, err := strconv.Atoi(strings.TrimSpace(m.values[FieldTargetWords]))
	if err != nil {
		return contracts.Requirements{}, errors.New(m.i18n.Text(i18n.RequirementsErrTargetWordsInteger))
	}
	req := contracts.Requirements{
		Topic:              strings.TrimSpace(m.values[FieldTopic]),
		ResearchQuestions:  splitList(m.values[FieldResearchQuestions]),
		Scope:              strings.TrimSpace(m.values[FieldScope]),
		Language:           normalizeLanguage(m.values[FieldLanguage]),
		CitationStyle:      normalizeCitationStyle(m.values[FieldCitationStyle], m.values[FieldLanguage]),
		ArticleTemplate:    normalizeArticleTemplate(m.values[FieldArticleTemplate]),
		QualityMode:        normalizeQualityMode(m.values[FieldQualityMode]),
		TargetWords:        targetWords,
		MaterialDir:        strings.TrimSpace(m.values[FieldMaterialDir]),
		AllowOnlineSearch:  parseBool(m.values[FieldAllowOnlineSearch]),
		SearchProviders:    splitComma(m.values[FieldSearchProviders]),
		ChapterPreferences: splitList(m.values[FieldChapterPreferences]),
		Constraints:        splitList(m.values[FieldConstraints]),
	}
	if !req.AllowOnlineSearch {
		req.SearchProviders = nil
	}
	if err := m.validate(req); err != nil {
		return contracts.Requirements{}, err
	}
	return req, nil
}

func (m Model) validate(req contracts.Requirements) error {
	if strings.TrimSpace(req.Topic) == "" {
		return errors.New(m.i18n.Text(i18n.RequirementsErrTopicRequired))
	}
	if req.Language != "zh-CN" && req.Language != "en" {
		return errors.New(m.i18n.Text(i18n.RequirementsErrLanguage))
	}
	switch req.CitationStyle {
	case "gbt7714", "apa":
	default:
		return errors.New(m.i18n.Text(i18n.RequirementsErrCitation))
	}
	switch req.QualityMode {
	case "", "fast", "enhanced", "strict":
	default:
		return errors.New(m.i18n.Text(i18n.RequirementsErrQualityMode))
	}
	if !quality.ValidArticleTemplateID(req.ArticleTemplate) {
		return errors.New(m.i18n.Text(i18n.RequirementsErrArticleTemplate))
	}
	if req.TargetWords <= 0 {
		return errors.New(m.i18n.Text(i18n.RequirementsErrTargetWordsPositive))
	}
	if strings.TrimSpace(req.MaterialDir) == "" {
		return errors.New(m.i18n.Text(i18n.RequirementsErrMaterialDirRequired))
	}
	if info, err := os.Stat(req.MaterialDir); err != nil {
		return fmt.Errorf("%s", fmt.Sprintf(m.i18n.Text(i18n.RequirementsErrMaterialDirAccess), err))
	} else if !info.IsDir() {
		return errors.New(m.i18n.Text(i18n.RequirementsErrMaterialDirNotDir))
	}
	return nil
}

func Validate(req contracts.Requirements) error {
	if strings.TrimSpace(req.Topic) == "" {
		return fmt.Errorf("topic is required")
	}
	if req.Language != "zh-CN" && req.Language != "en" {
		return fmt.Errorf("language must be zh-CN or en")
	}
	switch req.CitationStyle {
	case "gbt7714", "apa":
	default:
		return fmt.Errorf("citation style must be gbt7714 or apa")
	}
	switch req.QualityMode {
	case "", "fast", "enhanced", "strict":
	default:
		return fmt.Errorf("quality mode must be fast, enhanced, or strict")
	}
	if !quality.ValidArticleTemplateID(req.ArticleTemplate) {
		return fmt.Errorf("article template must be zh_course_paper or review_paper")
	}
	if req.TargetWords <= 0 {
		return fmt.Errorf("target words must be positive")
	}
	if strings.TrimSpace(req.MaterialDir) == "" {
		return fmt.Errorf("material dir is required")
	}
	if info, err := os.Stat(req.MaterialDir); err != nil {
		return fmt.Errorf("material dir is not accessible: %w", err)
	} else if !info.IsDir() {
		return fmt.Errorf("material dir is not a directory")
	}
	return nil
}

func (m Model) Done() bool {
	return m.done
}

func (m Model) Canceled() bool {
	return m.canceled
}

func (m Model) Err() error {
	return m.err
}

func (m Model) CurrentField() Field {
	if m.cursor < 0 || m.cursor >= len(orderedFields) {
		return FieldTopic
	}
	return orderedFields[m.cursor]
}

func (m Model) FieldValue(field Field) string {
	return m.values[field]
}

func (m Model) Cursor() int {
	return m.cursor
}

func (m *Model) appendRune(r rune) {
	field := m.CurrentField()
	m.values[field] += string(r)
}

func (m *Model) backspace() {
	field := m.CurrentField()
	runes := []rune(m.values[field])
	if len(runes) == 0 {
		return
	}
	m.values[field] = string(runes[:len(runes)-1])
}

func splitList(value string) []string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	parts := strings.Split(value, "\n")
	var out []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func splitComma(value string) []string {
	parts := strings.Split(value, ",")
	var out []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func normalizeLanguage(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "en"
	}
	return value
}

func normalizeCitationStyle(style, language string) string {
	style = strings.ToLower(strings.TrimSpace(style))
	if style != "" {
		return style
	}
	return "gbt7714"
}

func normalizeArticleTemplate(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return quality.ArticleTemplateZhCoursePaper
	}
	return value
}

func normalizeQualityMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		return "enhanced"
	}
	return mode
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func intString(value int) string {
	if value <= 0 {
		return ""
	}
	return strconv.Itoa(value)
}

func normalizeKey(key string) string {
	if key == " " {
		return "space"
	}
	return strings.ToLower(strings.TrimSpace(key))
}
