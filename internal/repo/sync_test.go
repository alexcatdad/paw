package repo

import "testing"

func TestShouldRefreshLinks(t *testing.T) {
	if !ShouldRefreshLinks([]string{"home/.zshrc"}) {
		t.Fatal("expected refresh")
	}
	if !ShouldRefreshLinks([]string{"paw.toml"}) {
		t.Fatal("expected refresh for config")
	}
	if ShouldRefreshLinks([]string{"README.md"}) {
		t.Fatal("unexpected refresh")
	}
}
