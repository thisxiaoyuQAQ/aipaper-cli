package agent

import "strings"

const (
	RoleCoordinator = "coordinator"
	RoleArchitect   = "architect"
	RoleWriter      = "writer"
	RoleEditor      = "editor"
)

type PromptOptions struct {
	RecoveryPrompt string
}

func CoordinatorSystemPrompt(opts PromptOptions) string {
	sections := []string{
		"You are the Coordinator for aipaper-cli.",
		"Host responsibilities: configuration, model/runtime construction, tool execution, event projection, checkpoint recovery, and shutdown only.",
		"Coordinator responsibilities: decide the next workflow step from structured tool facts; never rely on hidden state or unsupported assumptions.",
		"Tools return facts as JSON. Do not treat tool output as scheduling advice.",
		"Reference hard rule: Architect, Writer, and Editor may only use references/confirmed.json. Never cite candidates, rejected references, or online search results that were not confirmed by the user.",
		"Quality gate: a chapter passes only when overall >= 80, citation_consistency >= 90, and there are no high-risk unsupported claims. Rewrite at most two rounds, then mark needs_human_review.",
		"Checkpoint rule: artifacts must be written before checkpoint Record. Do not repeat or overwrite artifacts listed by a valid recovery prompt.",
		"Host must not decide rewrites from scores. You must decide after reading Editor facts.",
		"Return concise structured JSON when the runtime requests a machine-readable decision.",
	}
	sections = append(sections, qualityPromptSections()...)
	sections = append(sections, writerQualityPromptSections()...)
	if strings.TrimSpace(opts.RecoveryPrompt) != "" {
		sections = append(sections, "Recovery context:\n"+strings.TrimSpace(opts.RecoveryPrompt))
	}
	return strings.Join(sections, "\n\n")
}
