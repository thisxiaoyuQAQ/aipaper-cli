package agent

import (
	"strings"
	"testing"
)

func TestCoordinatorSystemPromptIncludesHardRulesAndRecovery(t *testing.T) {
	prompt := CoordinatorSystemPrompt(PromptOptions{RecoveryPrompt: "resume from step 7; do not overwrite drafts/ch01/draft.md"})
	for _, want := range []string{
		"Coordinator",
		"Host responsibilities",
		"references/confirmed.json",
		"overall >= 80",
		"checkpoint",
		"Paper Quality Skill policy",
		"citation existence is not claim support",
		"evidence depth controls claim strength",
		"do not overwrite drafts/ch01/draft.md",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}
