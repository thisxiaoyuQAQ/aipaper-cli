package contracts

import "time"

const (
	DefaultOutputDir = "output"
	DefaultProject   = "aipaper"
)

type Requirements struct {
	Topic              string   `json:"topic"`
	ResearchQuestions  []string `json:"research_questions"`
	Scope              string   `json:"scope"`
	Language           string   `json:"language"`
	CitationStyle      string   `json:"citation_style"`
	ArticleTemplate    string   `json:"article_template,omitempty"`
	QualityMode        string   `json:"quality_mode,omitempty"`
	TargetWords        int      `json:"target_words"`
	MaterialDir        string   `json:"material_dir"`
	AllowOnlineSearch  bool     `json:"allow_online_search"`
	SearchProviders    []string `json:"search_providers,omitempty"`
	ChapterPreferences []string `json:"chapter_preferences"`
	Constraints        []string `json:"constraints"`
}

type Run struct {
	RunID        string         `json:"run_id"`
	CreatedAt    time.Time      `json:"created_at"`
	ResumedFrom  *int           `json:"resumed_from"`
	Provider     string         `json:"provider,omitempty"`
	Model        string         `json:"model,omitempty"`
	CostEstimate map[string]any `json:"cost_estimate"`
	Events       []RunEvent     `json:"events"`
}

type RunEvent struct {
	At      time.Time      `json:"at"`
	Kind    string         `json:"kind"`
	Message string         `json:"message,omitempty"`
	Fields  map[string]any `json:"fields,omitempty"`
}

type Progress struct {
	Phase             string    `json:"phase"`
	CurrentStep       int       `json:"current_step"`
	CurrentChapter    string    `json:"current_chapter,omitempty"`
	CompletedChapters []string  `json:"completed_chapters"`
	PendingChapters   []string  `json:"pending_chapters"`
	Status            string    `json:"status"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type MaterialManifest struct {
	Items []MaterialItem `json:"items"`
}

type MaterialItem struct {
	ID         string `json:"id"`
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
	Parser     string `json:"parser,omitempty"`
	Degraded   bool   `json:"degraded"`
	OutputText string `json:"output_text,omitempty"`
	OutputMeta string `json:"output_meta,omitempty"`
	Error      string `json:"error,omitempty"`
}

type ReferenceCandidates struct {
	Items []ReferenceCandidate `json:"items"`
}

type ReferenceCandidate struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Authors         []string `json:"authors"`
	Year            int      `json:"year,omitempty"`
	Source          string   `json:"source"`
	DOI             string   `json:"doi,omitempty"`
	URL             string   `json:"url,omitempty"`
	Abstract        string   `json:"abstract,omitempty"`
	Venue           string   `json:"venue,omitempty"`
	CitationCount   int      `json:"citation_count,omitempty"`
	RelevanceScore  float64  `json:"relevance_score,omitempty"`
	RelevanceReason string   `json:"relevance_reason,omitempty"`
	ExpansionSource string   `json:"expansion_source,omitempty"`
	DedupeGroup     string   `json:"dedupe_group,omitempty"`
	Status          string   `json:"status"`
}

type ConfirmedReferences struct {
	Items []ConfirmedReference `json:"items"`
}

type ConfirmedReference struct {
	Key               string    `json:"key"`
	Title             string    `json:"title"`
	Authors           []string  `json:"authors"`
	Year              int       `json:"year,omitempty"`
	DOI               string    `json:"doi,omitempty"`
	URL               string    `json:"url,omitempty"`
	Abstract          string    `json:"abstract,omitempty"`
	Venue             string    `json:"venue,omitempty"`
	SourceMaterialIDs []string  `json:"source_material_ids"`
	ConfirmedAt       time.Time `json:"confirmed_at"`
}

type ClaimsFile struct {
	ChapterID    string  `json:"chapter_id"`
	DraftVersion int     `json:"draft_version"`
	Claims       []Claim `json:"claims"`
}

type Claim struct {
	ID            string   `json:"id"`
	Text          string   `json:"text"`
	Importance    string   `json:"importance"`
	ReferenceKeys []string `json:"reference_keys"`
	// EvidenceIDs binds the claim to evidence table entries (ev_NNN). New
	// writer output must carry at least one; legacy claims.json without the
	// field still loads and is treated as compatibility mode (warning only).
	EvidenceIDs []string `json:"evidence_ids,omitempty"`
	Confidence  float64  `json:"confidence,omitempty"`
}

type CitationMap struct {
	ChapterID    string            `json:"chapter_id"`
	DraftVersion int               `json:"draft_version"`
	Mappings     []CitationMapping `json:"mappings"`
}

type CitationMapping struct {
	ParagraphID   string   `json:"paragraph_id"`
	ClaimIDs      []string `json:"claim_ids"`
	ReferenceKeys []string `json:"reference_keys"`
}

type Review struct {
	ChapterID         string       `json:"chapter_id"`
	DraftVersion      int          `json:"draft_version"`
	Scores            ReviewScores `json:"scores"`
	Passed            bool         `json:"passed"`
	UnsupportedClaims []string     `json:"unsupported_claims"`
	RequiredFixes     []string     `json:"required_fixes"`
	OptionalFixes     []string     `json:"optional_fixes"`
	// RewriteInstructions are structured editor directives that drive the
	// rewrite loop (module 28). Legacy review.json without the field still
	// loads; new editor output should carry one instruction per problem.
	RewriteInstructions []RewriteInstruction `json:"rewrite_instructions,omitempty"`
}

// RewriteInstruction tells the Writer exactly what to change and which
// evidence to use. Severity aligns with the existing required/optional fixes:
// a required instruction blocks the chapter from passing until covered.
type RewriteInstruction struct {
	ClaimID              string   `json:"claim_id,omitempty"` // optional claim graph link
	Location             string   `json:"location"`           // chapter/paragraph locator
	Problem              string   `json:"problem"`            // unsupported / overstated / generalization / duplicate / weak evidence ...
	Instruction          string   `json:"instruction"`        // concrete change to make
	SuggestedEvidenceIDs []string `json:"suggested_evidence_ids,omitempty"`
	Severity             string   `json:"severity"` // required | optional
}

type ReviewScores struct {
	Overall             int `json:"overall"`
	CitationConsistency int `json:"citation_consistency"`
	StructureLogic      int `json:"structure_logic"`
	Coverage            int `json:"coverage"`
	Readability         int `json:"readability"`
}
