package auth

import "testing"

func TestDeviceFingerprint_StableAndSensitive(t *testing.T) {
	a := DeviceFingerprint("Mozilla/5.0", "10.0.0.5")
	b := DeviceFingerprint("Mozilla/5.0", "10.0.0.99")
	c := DeviceFingerprint("Mozilla/5.0", "10.0.1.1")
	d := DeviceFingerprint("Chrome/120 X11", "10.0.0.5")
	e := DeviceFingerprint("Mozilla/5.0", "")

	if a != b {
		t.Errorf("same /24 should produce same fingerprint: %q vs %q", a, b)
	}
	if a == c {
		t.Errorf("different /24 should differ: %q == %q", a, c)
	}
	if a == d {
		t.Errorf("different UA should differ: %q == %q", a, d)
	}
	if e == "" {
		t.Errorf("empty IP should still hash to a non-empty value")
	}
	if len(a) != 16 {
		t.Errorf("fingerprint length should be 16 hex chars, got %d (%q)", len(a), a)
	}
}

func TestDeviceFingerprint_IPv6Truncation(t *testing.T) {
	a := DeviceFingerprint("UA", "2001:db8:abcd:1234::1")
	b := DeviceFingerprint("UA", "2001:db8:abcd:beef::42")
	if a != b {
		t.Errorf("expected same /48 IPv6 to collide: %q vs %q", a, b)
	}
}

func TestAccountState_Predicates(t *testing.T) {
	empty := AccountState{}
	if empty.IsFrozen() || empty.IsDeleted() {
		t.Fatal("zero state should be neither frozen nor deleted")
	}
}
