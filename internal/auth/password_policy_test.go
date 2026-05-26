package auth

import (
	"strings"
	"testing"
)

func TestPasswordPolicy_Defaults_AcceptStrongPasswords(t *testing.T) {
	p := DefaultPasswordPolicy()
	strong := []string{
		"Tr0ub4dor&3Spires",
		"GreenHorse-Volcano-7!Pier",
		"Quartz#Banana42!Lake",
	}
	for _, pw := range strong {
		if err := p.Validate(pw); err != nil {
			t.Errorf("strong password %q rejected: %v", pw, err)
		}
	}
}

func TestPasswordPolicy_RejectsShortAndMissingClasses(t *testing.T) {
	p := DefaultPasswordPolicy()
	cases := map[string]string{
		"short": "Aa1!", // length 4 < min 8
		"no upper":   "lowercase1!",
		"no lower":   "UPPERCASE1!",
		"no digit":   "NoDigits!Here",
		"no special": "NoSpecial1Char",
	}
	for name, pw := range cases {
		if err := p.Validate(pw); err == nil {
			t.Errorf("%s: expected error for %q", name, pw)
		}
	}
}

func TestPasswordPolicy_RejectsIdentityContainment(t *testing.T) {
	p := DefaultPasswordPolicy()
	err := p.Validate("Alice.Smith2024!", "alice.smith@example.com", "Alice Smith")
	if err == nil {
		t.Fatal("expected error for password containing identity")
	}
	if !strings.Contains(err.Error(), "email or name") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPasswordPolicy_AllowsShortIdentityTokens(t *testing.T) {
	p := DefaultPasswordPolicy()
	// "ab" is shorter than the 4-char floor — it should not trip the check.
	if err := p.Validate("Thunder#Owl42Pi", "ab"); err != nil {
		t.Errorf("short identity token false positive: %v", err)
	}
}

func TestPasswordPolicy_RejectsCommonPasswords(t *testing.T) {
	p := DefaultPasswordPolicy()
	cases := []string{"Password1!", "Welcome1!", "P@ssw0rd!", "Qwerty123!"}
	for _, pw := range cases {
		if err := p.Validate(pw); err == nil {
			t.Errorf("expected %q to be rejected as common", pw)
		}
	}
}

func TestIsCommonPassword_LeetAndTrailingDigits(t *testing.T) {
	cases := []struct {
		password string
		want     bool
	}{
		{"password", true},
		{"Password", true},
		{"password123", true},
		{"P@ssw0rd", true},
		{"qwerty2024", true},
		{"correct horse battery staple", false},
		{"GreenHorse-Volcano-7!Pier", false},
	}
	for _, c := range cases {
		if got := IsCommonPassword(c.password); got != c.want {
			t.Errorf("IsCommonPassword(%q) = %v, want %v", c.password, got, c.want)
		}
	}
}

func TestPasswordScore_Ranges(t *testing.T) {
	cases := []struct {
		password string
		min      int
		max      int
	}{
		{"aaaa", 0, 1},                      // run + only one class
		{"Password1!", 0, 2},                // common + repeating capital
		{"abcdEFGH1!", 1, 3},                // 10 chars, 4 classes, has sequence "abcd"
		{"GreenHorse-Volcano-7!Pier", 3, 4}, // 25 chars + 4 classes
		{"a", 0, 0},
		{"", 0, 0},
	}
	for _, c := range cases {
		got := PasswordScore(c.password)
		if got < c.min || got > c.max {
			t.Errorf("PasswordScore(%q) = %d, want in [%d,%d]", c.password, got, c.min, c.max)
		}
	}
}

func TestHasRunOrSequence(t *testing.T) {
	cases := map[string]bool{
		"aaaa":      true,
		"abcd":      true,
		"abcD":      true, // lower-cased before check
		"1234":      true,
		"abc":       false,
		"a1b2c3":    false,
		"qwerty":    false, // adjacent on keyboard but not ascii-monotone
		"abracadab": false,
	}
	for pw, want := range cases {
		if got := hasRunOrSequence(pw); got != want {
			t.Errorf("hasRunOrSequence(%q) = %v, want %v", pw, got, want)
		}
	}
}

func TestParseBoolEnv(t *testing.T) {
	cases := map[string]struct {
		input string
		dflt  bool
		want  bool
	}{
		"true":   {"true", false, true},
		"yes":    {"yes", false, true},
		"on":     {"on", false, true},
		"1":      {"1", false, true},
		"false":  {"false", true, false},
		"no":     {"no", true, false},
		"off":    {"off", true, false},
		"0":      {"0", true, false},
		"typo":   {"yas", true, true},
		"empty":  {"", false, false},
	}
	for name, c := range cases {
		if got := parseBoolEnv(c.input, c.dflt); got != c.want {
			t.Errorf("%s: parseBoolEnv(%q, %v) = %v, want %v", name, c.input, c.dflt, got, c.want)
		}
	}
}

func TestPasswordPolicy_MinScoreGate(t *testing.T) {
	p := DefaultPasswordPolicy()
	p.MinScore = 4 // require Strong; the all-classes 8-char baseline should fail
	if err := p.Validate("Abcd1234!"); err == nil {
		t.Errorf("expected MinScore=4 to reject medium-strength password")
	}
	if err := p.Validate("GreenHorse-Volcano-7!Pier"); err != nil {
		t.Errorf("expected MinScore=4 to accept long passphrase, got %v", err)
	}
}
