package auth

import "testing"

func TestNormalizeIP(t *testing.T) {
	ptr := func(s string) *string { return &s }
	cases := []struct {
		name string
		in   *string
		want *string
	}{
		{"nil", nil, nil},
		{"empty", ptr(""), ptr("")},
		{"ipv6 loopback with port", ptr("[::1]:52253"), ptr("::1")},
		{"ipv4 with port", ptr("1.2.3.4:80"), ptr("1.2.3.4")},
		{"bare ipv4 unchanged", ptr("1.2.3.4"), ptr("1.2.3.4")},
		{"bare ipv6 unchanged", ptr("2001:db8::1"), ptr("2001:db8::1")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeIP(tc.in)
			switch {
			case tc.want == nil && got != nil:
				t.Fatalf("got %q, want nil", *got)
			case tc.want != nil && got == nil:
				t.Fatalf("got nil, want %q", *tc.want)
			case tc.want != nil && got != nil && *got != *tc.want:
				t.Fatalf("got %q, want %q", *got, *tc.want)
			}
		})
	}
}
