package ldap

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestFilterForEscapesInjection(t *testing.T) {
	s := Settings{UserFilter: "(uid=%s)"}
	// A malicious username must be RFC 4515-escaped so it cannot alter the filter.
	got := s.filterFor("*)(uid=*))(|(uid=*")
	if strings.Contains(got, "*)(") || strings.Contains(got, ")(|(") {
		t.Fatalf("filter not escaped: %q", got)
	}
	if !strings.Contains(got, `\2a`) { // '*' -> \2a
		t.Fatalf("asterisk should be escaped to \\2a: %q", got)
	}
}

func TestFilterForDefault(t *testing.T) {
	s := Settings{}
	if got := s.filterFor("alice"); got != "(uid=alice)" {
		t.Fatalf("default filter: %q", got)
	}
}

func TestAuthenticateRejectsEmptyPassword(t *testing.T) {
	c := New()
	if _, err := c.Authenticate(context.Background(), Settings{Host: "ldap.example", BaseDN: "dc=x"}, "alice", ""); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("empty password must be rejected as invalid credentials, got %v", err)
	}
	if _, err := c.Authenticate(context.Background(), Settings{Host: "ldap.example"}, "", "pw"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("empty username must be rejected as invalid credentials, got %v", err)
	}
}

func TestSettingsAddrDefaults(t *testing.T) {
	if got := (Settings{Host: "h", Security: SecurityLDAPS}).addr(); got != "h:636" {
		t.Fatalf("ldaps default port: %q", got)
	}
	if got := (Settings{Host: "h"}).addr(); got != "h:389" {
		t.Fatalf("plain default port: %q", got)
	}
	if got := (Settings{Host: "h", Port: 1389}).addr(); got != "h:1389" {
		t.Fatalf("explicit port: %q", got)
	}
}
