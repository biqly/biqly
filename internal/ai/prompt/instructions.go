package prompt

import (
	"bytes"
	"strings"
)

// maxPromptInstructions caps free-form business rules so a large instruction
// list cannot crowd out schema context.
const maxPromptInstructions = 20

// Instruction is a free-form business rule injected into the prompt. Title is
// optional; Body carries the markdown rule text.
type Instruction struct {
	Title string
	Body  string
}

// writeInstructions renders active business-rule instructions as a
// "## Business Rules" prompt section. Instructions are admin-curated free-form
// rules that steer interpretation without overriding the semantic model.
func (*Builder) writeInstructions(sb *bytes.Buffer, instructions []Instruction) {
	filtered := make([]Instruction, 0, len(instructions))
	for _, in := range instructions {
		if strings.TrimSpace(in.Body) == "" && strings.TrimSpace(in.Title) == "" {
			continue
		}
		filtered = append(filtered, in)
	}
	if len(filtered) == 0 {
		return
	}
	if len(filtered) > maxPromptInstructions {
		filtered = filtered[:maxPromptInstructions]
	}
	writePromptString(sb, "\n\n## Business Rules\n")
	writePromptString(sb, "Curated business rules for this datasource. Apply them when interpreting the question; they never override the semantic model.\n\n")
	for _, in := range filtered {
		title := strings.TrimSpace(in.Title)
		body := strings.TrimSpace(in.Body)
		switch {
		case title != "" && body != "":
			writePromptf(sb, "- **%s**: %s\n", title, body)
		case title != "":
			writePromptf(sb, "- **%s**\n", title)
		default:
			writePromptf(sb, "- %s\n", body)
		}
	}
	writePromptString(sb, "\n")
}
