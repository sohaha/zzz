package agent

import (
	"fmt"

	"github.com/sohaha/zlsgo/zfile"
)

const (
	PromptCommitMessage = `Please review all uncommitted changes in the git repository (both modified and new files). Write a commit message with: (1) a short one-line summary, (2) two newlines, (3) then a detailed explanation. Do not include any footers or metadata like 'Generated with Claude Code' or 'Co-Authored-By'. Feel free to look at the last few commits to get a sense of the commit message style for consistency. First run 'git add .' to stage all changes including new untracked files, then commit using 'git commit -m "your message"' (don't push, just commit, no need to ask for confirmation).`

	PromptWorkflowContext = `## CONTINUOUS WORKFLOW CONTEXT

This is part of a continuous development loop where work happens incrementally across multiple iterations. You might run once, then a human developer might make changes, then you run again, and so on. This could happen daily or on any schedule.

**Important**: You don't need to complete the entire goal in one iteration. Just make meaningful progress on one thing, then leave clear notes for the next iteration (human or AI). Think of it as a relay race where you're passing the baton.

**Project Completion Signal**: If you determine that not just your current task but the ENTIRE project goal is fully complete (nothing more to be done on the overall goal), only include the exact phrase "%s" in your response. Only use this when absolutely certain that the whole project is finished, not just your individual task. We will stop working on this project when multiple developers independently determine that the project is complete.

## PRIMARY GOAL
<PRIMARY_GOAL>
%s
</PRIMARY_GOAL>
`

	PromptNotesUpdateExisting = `Update the %s file with relevant context for the next iteration. Add new notes and remove outdated information to keep it current and useful.`

	PromptNotesCreateNew = `Create a %s file with relevant context and instructions for the next iteration.`

	PromptNotesGuidelines = `

This file helps coordinate work across iterations (both human and AI developers). It should:

- Contain relevant context and instructions for the next iteration
- Stay concise and actionable (like a notes file, not a detailed report)
- Help the next developer understand what to do next

The file should NOT include:
- Lists of completed work or full reports
- Information that can be discovered by running tests/coverage
- Unnecessary details`

	PromptReviewerContext = `## CODE REVIEW CONTEXT

You are performing a review pass on changes just made by another developer. This is NOT a new feature implementation - you are reviewing and validating existing changes using the instructions given below by the user. Feel free to use git commands to see what changes were made if it's helpful to you.`

	PromptCIFixContext = `## CI FAILURE FIX CONTEXT

You are analyzing and fixing a CI/CD failure for a pull request.

**Your task:**
1. Inspect the failed CI workflow using the commands below
2. Analyze the error logs to understand what went wrong
3. Make the necessary code changes to fix the issue
4. Stage and commit your changes (they will be pushed to update the PR)

**Commands to inspect CI failures:**
- 'gh run list --status failure --limit 3' - List recent failed runs
- 'gh run view <RUN_ID> --log-failed' - View failed job logs (shorter output)
- 'gh run view <RUN_ID> --log' - View full logs for a specific run

**Important:**
- Focus only on fixing the CI failure, not adding new features
- Make minimal changes necessary to pass CI
- If the failure seems unfixable (e.g., flaky test, infrastructure issue), explain why in your response`
)

func BuildEnhancedPrompt(ctx *Context) string {
	workflowContext := fmt.Sprintf(PromptWorkflowContext, ctx.CompletionSignal, ctx.Prompt)

	if zfile.FileExist(ctx.NotesFile) {
		notes, _ := zfile.ReadFile(ctx.NotesFile)
		workflowContext += fmt.Sprintf(`## CONTEXT FROM PREVIOUS ITERATION

The following is from %s, maintained by previous iterations to provide context:

%s

`, ctx.NotesFile, notes)
	}

	notesPrompt := "## ITERATION NOTES\n\n"
	if zfile.FileExist(ctx.NotesFile) {
		notesPrompt += fmt.Sprintf(PromptNotesUpdateExisting, ctx.NotesFile)
	} else {
		notesPrompt += fmt.Sprintf(PromptNotesCreateNew, ctx.NotesFile)
	}
	notesPrompt += PromptNotesGuidelines

	return workflowContext + notesPrompt
}
