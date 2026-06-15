package quality

import "strings"

const (
	ArticleTemplateZhCoursePaper = "zh_course_paper"
	ArticleTemplateReviewPaper   = "review_paper"
)

type ArticleTemplate struct {
	ID                     string
	DisplayName            string
	RequiredBlocks         []string
	DefaultCitationStyle   string
	MinEvidencePerChapter  int
	LowContentMinWords     int
	ForbiddenDraftPatterns []string
	WriterGuidance         []string
}

func ResolveArticleTemplate(id string) ArticleTemplate {
	id = strings.ToLower(strings.TrimSpace(id))
	switch id {
	case ArticleTemplateReviewPaper:
		return ArticleTemplate{
			ID:                     ArticleTemplateReviewPaper,
			DisplayName:            "综述论文",
			RequiredBlocks:         []string{"abstract", "keywords", "introduction", "thematic_review", "discussion", "conclusion", "references"},
			DefaultCitationStyle:   "gbt7714",
			MinEvidencePerChapter:  2,
			LowContentMinWords:     180,
			ForbiddenDraftPatterns: []string{"证据不足", "待验证", "只能提出框架", "不能证明", "仅作为研究设计"},
			WriterGuidance: []string{
				"Write as a substantive literature review: synthesize findings instead of listing metadata.",
				"Every section must connect evidence to the paper topic and avoid using evidence gaps as the main body.",
			},
		}
	case ArticleTemplateZhCoursePaper, "":
		fallthrough
	default:
		return ArticleTemplate{
			ID:                     ArticleTemplateZhCoursePaper,
			DisplayName:            "中文课程论文",
			RequiredBlocks:         []string{"title", "abstract", "keywords", "introduction", "body", "conclusion", "references"},
			DefaultCitationStyle:   "gbt7714",
			MinEvidencePerChapter:  1,
			LowContentMinWords:     160,
			ForbiddenDraftPatterns: []string{"证据不足", "待验证", "只能提出框架", "不能证明", "仅作为研究设计"},
			WriterGuidance: []string{
				"Write in a Chinese academic-paper style with clear problem, analysis, and conclusion blocks.",
				"Use concrete entities, dimensions, and comparisons; do not fill the body with caveats.",
			},
		}
	}
}

func ValidArticleTemplateID(id string) bool {
	id = strings.ToLower(strings.TrimSpace(id))
	return id == "" || id == ArticleTemplateZhCoursePaper || id == ArticleTemplateReviewPaper
}
