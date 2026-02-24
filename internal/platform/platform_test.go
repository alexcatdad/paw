package platform

import "testing"

func TestMatchPlatform(t *testing.T) {
	if !MatchPlatform([]string{"linux", "wsl"}, "linux") {
		t.Fatal("expected platform match")
	}
	if MatchPlatform([]string{"darwin"}, "linux") {
		t.Fatal("expected mismatch")
	}
}

func TestMatchHostname(t *testing.T) {
	if !MatchHostname("work-*", "work-laptop") {
		t.Fatal("expected wildcard host match")
	}
	if MatchHostname("prod-*", "dev-box") {
		t.Fatal("unexpected host match")
	}
}
