package audit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alexcatdad/paw/internal/config"
	"github.com/alexcatdad/paw/internal/output"
)

func TestRunBranchesWithAndWithoutConfig(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, "home"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, ".zshrc"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Missing paw.toml branch.
	resMissing, err := Run(repoDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resMissing.Score <= 0 {
		t.Fatalf("unexpected score for missing config run: %d", resMissing.Score)
	}

	// Found paw.toml branch + config suggestions.
	if err := os.WriteFile(filepath.Join(repoDir, "paw.toml"), []byte("version=1\nlayout='hybrid'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	resWithCfg, err := Run(repoDir, &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(resWithCfg.Findings) == 0 {
		t.Fatal("expected findings")
	}
}

func TestPrintAndHelperBranches(t *testing.T) {
	logger := output.NewLogger("text", true, false)
	res := Result{
		RepoPath: "/tmp/repo",
		Score:    91,
		Findings: []Finding{
			{Severity: SeverityError, Message: "error finding"},
			{Severity: SeverityWarning, Message: "warning finding"},
			{Severity: SeveritySuggestion, Message: "suggestion finding", Suggestion: "do this"},
			{Severity: SeverityInfo, Message: "info finding"},
		},
	}
	Print(res, Options{Verbose: true}, logger)
	Print(res, Options{Verbose: true, MinSeverity: SeverityWarning}, logger)
	Print(res, Options{JSON: true}, logger)

	if !hasPrefix([]string{"home/.zshrc"}, "home/") {
		t.Fatal("expected hasPrefix match")
	}
	if hasPrefix([]string{"README.md"}, "home/") {
		t.Fatal("expected hasPrefix miss")
	}
	if hasMixedNaming([]string{".zshrc", "README.md"}) == false {
		t.Fatal("expected mixed naming")
	}
	if hasMixedNaming([]string{"home/.zshrc", "home/.gitconfig"}) {
		t.Fatal("expected consistent naming")
	}
	if !severityAllowed(SeverityError, SeverityWarning) {
		t.Fatal("expected severity ordering to allow error >= warning filter")
	}
	if severityAllowed(SeverityInfo, SeverityWarning) {
		t.Fatal("expected info to be filtered by warning minimum")
	}
	if fmtScore(87) != "87/100" {
		t.Fatal("unexpected score formatting")
	}
}
