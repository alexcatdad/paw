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
	// Original basic cases
	if !MatchHostname("work-*", "work-laptop") {
		t.Fatal("expected wildcard host match")
	}
	if MatchHostname("prod-*", "dev-box") {
		t.Fatal("unexpected host match")
	}
}

func TestMatchHostnameEmptyAndWildcard(t *testing.T) {
	// Empty pattern and bare "*" always match
	if !MatchHostname("", "anything") {
		t.Fatal("empty pattern should match any hostname")
	}
	if !MatchHostname("   ", "anything") {
		t.Fatal("whitespace-only pattern should match any hostname")
	}
	if !MatchHostname("*", "any-host") {
		t.Fatal("bare '*' should match any hostname")
	}
}

func TestMatchHostnameExactMatch(t *testing.T) {
	if !MatchHostname("myhost", "myhost") {
		t.Fatal("exact match should succeed")
	}
	if MatchHostname("myhost", "otherhost") {
		t.Fatal("exact match should fail for different hostname")
	}
	// Case-insensitive exact match
	if !MatchHostname("MyHost", "myhost") {
		t.Fatal("exact match should be case-insensitive")
	}
}

func TestMatchHostnamePrefixGlob(t *testing.T) {
	// "dev-*" should match hostnames starting with "dev-"
	if !MatchHostname("dev-*", "dev-laptop") {
		t.Fatal("dev-* should match dev-laptop")
	}
	if !MatchHostname("dev-*", "dev-workstation") {
		t.Fatal("dev-* should match dev-workstation")
	}
	if MatchHostname("dev-*", "prod-laptop") {
		t.Fatal("dev-* should not match prod-laptop")
	}
}

func TestMatchHostnameSuffixGlob(t *testing.T) {
	// "*.local" should match hostnames ending with ".local"
	if !MatchHostname("*.local", "myhost.local") {
		t.Fatal("*.local should match myhost.local")
	}
	if !MatchHostname("*.local", "workstation.local") {
		t.Fatal("*.local should match workstation.local")
	}
	if MatchHostname("*.local", "myhost.remote") {
		t.Fatal("*.local should not match myhost.remote")
	}
}

func TestMatchHostnameSingleCharWildcard(t *testing.T) {
	// "host-?-prod" should match exactly one character in position
	if !MatchHostname("host-?-prod", "host-a-prod") {
		t.Fatal("host-?-prod should match host-a-prod")
	}
	if !MatchHostname("host-?-prod", "host-1-prod") {
		t.Fatal("host-?-prod should match host-1-prod")
	}
	if MatchHostname("host-?-prod", "host-ab-prod") {
		t.Fatal("host-?-prod should not match host-ab-prod (two chars)")
	}
	if MatchHostname("host-?-prod", "host--prod") {
		t.Fatal("host-?-prod should not match host--prod (zero chars)")
	}
}

func TestMatchHostnameCaseInsensitiveGlob(t *testing.T) {
	if !MatchHostname("DEV-*", "dev-laptop") {
		t.Fatal("glob match should be case-insensitive")
	}
	if !MatchHostname("dev-*", "DEV-LAPTOP") {
		t.Fatal("glob match should be case-insensitive (uppercase hostname)")
	}
}

func TestMatchHostnameInvalidPattern(t *testing.T) {
	// filepath.Match reports an error for patterns with unclosed brackets.
	// The fallback is exact (case-insensitive) match.
	badPattern := "host-[-z"
	// Should not match a different hostname
	if MatchHostname(badPattern, "host-a") {
		t.Fatal("invalid pattern should fall back to exact match and not match host-a")
	}
	// Should match itself exactly (case-insensitive)
	if !MatchHostname(badPattern, badPattern) {
		t.Fatal("invalid pattern should fall back to exact match and match itself")
	}
}

func TestMatchHostnameNoPartialMatch(t *testing.T) {
	// Without a wildcard, the full string must match
	if MatchHostname("host", "host-extra") {
		t.Fatal("exact pattern should not partially match longer hostname")
	}
	if MatchHostname("host-extra", "host") {
		t.Fatal("exact pattern should not match prefix of hostname")
	}
}
