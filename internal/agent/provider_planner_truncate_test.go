package agent

import (
	"testing"
	"unicode/utf8"
)

// TestTruncate_ValidUTF8OnByteBoundary verifies the byte-budget truncation never
// splits a multi-byte rune into invalid UTF-8 (D2): the classic bug was
// s[:maxLen] cutting mid-rune for non-ASCII (e.g. Turkish) content.
func TestTruncate_ValidUTF8OnByteBoundary(t *testing.T) {
	// "ğ" is 2 bytes (0xC4 0x9F); a byte-limit landing between them must back off.
	s := "ağğğğğğğğğ"
	for maxLen := 1; maxLen < len(s); maxLen++ {
		got := truncate(s, maxLen)
		if !utf8.ValidString(got) {
			t.Fatalf("truncate(%q, %d) produced invalid UTF-8: %q", s, maxLen, got)
		}
	}

	// Short input is returned unchanged.
	if got := truncate("abc", 10); got != "abc" {
		t.Errorf("expected unchanged short input, got %q", got)
	}
}
