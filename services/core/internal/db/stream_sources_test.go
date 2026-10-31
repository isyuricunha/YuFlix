package db

import "testing"

func TestSafeDefaultRouting(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", "direct"}, {"direct", "direct"}, {"egress", "egress"}, {"auto", "auto"},
		{"external", "direct"}, {"../../private", "direct"},
	} {
		if got := normalizedStreamRoute(tc.in); got != tc.want {
			t.Fatalf("route %q became %q, expected %q", tc.in, got, tc.want)
		}
	}
}
