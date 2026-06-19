package quality

import (
	"reflect"
	"strings"
	"testing"
)

func TestDefaultPaperQualityPolicyScopes(t *testing.T) {
	policy := DefaultPaperQualityPolicy()
	if policy.Version != PaperQualityPolicyVersion {
		t.Fatalf("Version = %q, want %q", policy.Version, PaperQualityPolicyVersion)
	}
	scopes := map[string][]string{
		"coordinator": policy.CoordinatorSections,
		"narrative":   policy.ArchitectNarrative,
		"evidence":    policy.EvidenceDepth,
		"section":     policy.SectionPlan,
		"writer":      policy.Writer,
		"verifier":    policy.Verifier,
		"editor":      policy.Editor,
		"report":      policy.Report,
	}
	for name, sections := range scopes {
		if len(sections) == 0 {
			t.Fatalf("%s scope is empty", name)
		}
	}
}

func TestPaperQualityPolicyContainsCoreRules(t *testing.T) {
	policy := DefaultPaperQualityPolicy()
	all := strings.Join(append(append(append(append(append(append(append(
		append([]string{}, policy.CoordinatorSections...),
		policy.ArchitectNarrative...),
		policy.EvidenceDepth...),
		policy.SectionPlan...),
		policy.Writer...),
		policy.Verifier...),
		policy.Editor...),
		policy.Report...), "\n")
	for _, want := range []string{
		PaperQualityPolicyVersion,
		"confirmed references",
		"citation existence is not claim support",
		"metadata_only",
		"writer_notes",
		"claim_type",
		"required rewrite instruction",
		"human review",
	} {
		if !strings.Contains(all, want) {
			t.Fatalf("policy missing %q:\n%s", want, all)
		}
	}
}

func TestPaperQualityPolicyStableOutput(t *testing.T) {
	first := DefaultPaperQualityPolicy()
	second := DefaultPaperQualityPolicy()
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("policy output is not stable\nfirst=%#v\nsecond=%#v", first, second)
	}
}

func TestPaperQualityForbiddenDraftPatterns(t *testing.T) {
	patterns := PaperQualityForbiddenDraftPatterns()
	if len(patterns) == 0 {
		t.Fatal("forbidden draft patterns are empty")
	}
	joined := strings.Join(patterns, "\n")
	for _, want := range []string{"证据不足", "metadata_only", "宣传式评价"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("forbidden patterns missing %q: %s", want, joined)
		}
	}
}
