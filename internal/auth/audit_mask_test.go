package auth

import "testing"

func TestMaskEmail(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"a@x.com", "*@x.com"},
		{"ab@x.com", "a*@x.com"},
		{"baris@example.com", "b***s@example.com"},
		{"no-at-sign", "***"},
		{"@trailing.com", "***"},
		{"leading@", "***"},
	}
	for _, c := range cases {
		if got := MaskEmail(c.in); got != c.want {
			t.Errorf("MaskEmail(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMaskIP(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"192.168.1.5", "192.168.***.***"},
		{"10.0.0.1:5432", "10.0.***.***"},
		{"not-an-ip", "***"},
	}
	for _, c := range cases {
		if got := MaskIP(c.in); got != c.want {
			t.Errorf("MaskIP(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMaskToken(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"short", "***"},
		{"abcd1234efgh5678", "abcd***5678"},
	}
	for _, c := range cases {
		if got := MaskToken(c.in); got != c.want {
			t.Errorf("MaskToken(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMaskAuditMetadata(t *testing.T) {
	in := map[string]any{
		"email":         "user@example.com",
		"ip":            "10.0.0.5",
		"refresh_token": "abcd1234efgh5678",
		"role_id":       "admin",
	}
	out := maskAuditMetadata(in)
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", out)
	}
	if m["email"] != "u**r@example.com" {
		t.Errorf("email not masked: %v", m["email"])
	}
	if m["ip"] != "10.0.***.***" {
		t.Errorf("ip not masked: %v", m["ip"])
	}
	if m["refresh_token"] != "abcd***5678" {
		t.Errorf("token not masked: %v", m["refresh_token"])
	}
	if m["role_id"] != "admin" {
		t.Errorf("non-sensitive field altered: %v", m["role_id"])
	}
}
