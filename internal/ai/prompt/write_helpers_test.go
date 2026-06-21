package prompt

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestWritePromptfWritesToWriter(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	writePromptf(&buf, "hello %s", "world")
	if buf.String() != "hello world" {
		t.Fatalf("got %q, want %q", buf.String(), "hello world")
	}
}

func TestWritePromptfNoArgs(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	writePromptf(&buf, "plain string")
	if buf.String() != "plain string" {
		t.Fatalf("got %q", buf.String())
	}
}

func TestWritePromptfHandlesWriteError(t *testing.T) {
	t.Parallel()
	ew := &errorWriter{err: errors.New("write failed")}
	writePromptf(ew, "should not panic %s", "on error")
}

// errorWriter implements io.Writer but always returns an error.
type errorWriter struct {
	err error
}

func (e *errorWriter) Write(_ []byte) (int, error)       { return 0, e.err }
func (e *errorWriter) WriteString(_ string) (int, error) { return 0, e.err }

func TestWritePromptStringWrites(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	writePromptString(&buf, "some text")
	if buf.String() != "some text" {
		t.Fatalf("got %q", buf.String())
	}
}

func TestWritePromptStringEmpty(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	writePromptString(&buf, "")
	if buf.String() != "" {
		t.Fatalf("got %q, want empty", buf.String())
	}
}

func TestWritePromptStringHandlesError(t *testing.T) {
	t.Parallel()
	ew := &errorWriter{err: errors.New("write failed")}
	writePromptString(ew, "should not panic on error")
}

// Test that writePromptString works with *bytes.Buffer (which implements WriteString).
func TestWritePromptStringWithBuffer(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	writePromptString(&buf, "test\nline")
	if buf.String() != "test\nline" {
		t.Fatalf("got %q", buf.String())
	}
}

// Test that bytes.Buffer implements promptStringWriter.
func TestBufferImplementsPromptStringWriter(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	var iface promptStringWriter = &buf
	_ = iface
}

// Test with a strings.Builder which also implements WriteString.
func TestWritePromptStringWithStringsBuilder(t *testing.T) {
	t.Parallel()
	var sb strings.Builder
	writePromptString(&sb, "builder test")
	if sb.String() != "builder test" {
		t.Fatalf("got %q", sb.String())
	}
}
