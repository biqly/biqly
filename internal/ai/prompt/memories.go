package prompt

import "bytes"

// maxPromptMemories caps remembered facts so a large memory list cannot
// crowd out schema context.
const maxPromptMemories = 20

// writeMemories renders the user's durable remembered facts as a prompt
// section. Facts are user-curated (or saved from prior conversations) and
// steer interpretation without overriding the semantic model.
func (*Builder) writeMemories(sb *bytes.Buffer, memories []string) {
	if len(memories) == 0 {
		return
	}
	if len(memories) > maxPromptMemories {
		memories = memories[:maxPromptMemories]
	}
	writePromptString(sb, "\n\n## Remembered Facts\n")
	writePromptString(sb, "Durable facts and preferences this user has saved. Apply them when interpreting the question; they never override the semantic model.\n\n")
	for _, m := range memories {
		writePromptf(sb, "- %s\n", m)
	}
	writePromptString(sb, "\n")
}
