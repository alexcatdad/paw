package backup

import (
	"testing"
	"time"

	"github.com/alexcatdad/paw/internal/fsx"
	"github.com/alexcatdad/paw/internal/symlink"
	"github.com/alexcatdad/paw/internal/testutil"
)

func TestSetDependenciesAndStatesToEntries(t *testing.T) {
	SetDependencies(fsx.NewOSFS(), testutil.FakeClock{Instant: time.Unix(10, 0)})
	states := []symlink.State{
		{Source: "a", Target: "b", Status: symlink.StatusLinked},
		{Source: "c", Target: "d", Status: symlink.StatusConflict},
		{Source: "e", Target: "f", Status: symlink.StatusBackup},
	}
	entries := StatesToEntries(states)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Source != "a" || entries[1].Target != "f" {
		t.Fatalf("unexpected entries: %#v", entries)
	}
}
