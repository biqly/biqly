package prompt

import (
	"fmt"
	"io"
)

type promptStringWriter interface {
	WriteString(string) (int, error)
}

func writePromptf(w io.Writer, format string, args ...any) {
	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		return
	}
}

func writePromptString(w promptStringWriter, s string) {
	if _, err := w.WriteString(s); err != nil {
		return
	}
}
