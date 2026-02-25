package drift

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/alexcatdad/paw/internal/backup"
	"github.com/alexcatdad/paw/internal/output"
	"github.com/alexcatdad/paw/internal/repo"
	"github.com/alexcatdad/paw/internal/symlink"
)

const (
	StatusDrift       = "drift"
	StatusMissing     = "missing"
	StatusConflict    = "conflict"
	StatusUnavailable = "unavailable"
)

const brewfileRelPath = "home/.config/homebrew/Brewfile"

type Context struct {
	Scope    Scope
	RepoPath string
	HomePath string
	Entries  []symlink.Entry
	DryRun   bool
	Logger   *output.Logger
}

func ParseScope(raw string) (Scope, error) {
	value := Scope(strings.ToLower(strings.TrimSpace(raw)))
	if value == "" {
		return ScopeAll, nil
	}
	switch value {
	case ScopeAll, ScopeFiles, ScopePackages:
		return value, nil
	default:
		return "", fmt.Errorf("invalid scope %q (expected all|files|packages)", raw)
	}
}

func Inspect(ctx Context) (Report, error) {
	scope, err := normalizeScope(ctx.Scope)
	if err != nil {
		return Report{}, err
	}
	report := Report{
		Timestamp: clk.Now().UTC().Format(time.RFC3339),
		RepoPath:  ctx.RepoPath,
		HomePath:  ctx.HomePath,
		Scope:     scope,
		Findings:  []Finding{},
	}

	if includesScope(scope, ScopeFiles) {
		findings, err := InspectFiles(ctx)
		if err != nil {
			return Report{}, err
		}
		report.Findings = append(report.Findings, findings...)
	}
	if includesScope(scope, ScopePackages) {
		findings, err := InspectBrew(ctx)
		if err != nil {
			return Report{}, err
		}
		report.Findings = append(report.Findings, findings...)
	}

	report.Summary = summarize(report.Findings)
	return report, nil
}

func InspectFiles(ctx Context) ([]Finding, error) {
	if len(ctx.Entries) == 0 {
		return []Finding{}, nil
	}
	states, err := symlink.Status(ctx.Entries)
	if err != nil {
		return nil, err
	}

	findings := make([]Finding, 0)
	for idx, st := range states {
		entry := ctx.Entries[idx]
		switch st.Status {
		case symlink.StatusConflict:
			finding, err := classifyConflict(entry)
			if err != nil {
				return nil, err
			}
			findings = append(findings, finding)
		case symlink.StatusSourceMissing:
			finding, ok, err := classifySourceMissing(entry)
			if err != nil {
				return nil, err
			}
			if ok {
				findings = append(findings, finding)
			}
		case symlink.StatusMissing:
			findings = append(findings, Finding{
				Scope:      ScopeFiles,
				Kind:       KindFileMissing,
				SourcePath: entry.SourceAbs,
				TargetPath: entry.TargetAbs,
				Status:     StatusMissing,
				Details:    "managed target is missing",
				CanApply:   false,
			})
		}
	}
	return findings, nil
}

func InspectBrew(ctx Context) ([]Finding, error) {
	tracked := filepath.Join(ctx.RepoPath, filepath.FromSlash(brewfileRelPath))
	if _, err := runner.LookPath("brew"); err != nil {
		return []Finding{{
			Scope:      ScopePackages,
			Kind:       KindBrewUnavailable,
			SourcePath: tracked,
			Status:     StatusUnavailable,
			Details:    "brew is not available in PATH",
			CanApply:   false,
		}}, nil
	}

	snapshot, err := dumpBrewfileSnapshot()
	if err != nil {
		return nil, err
	}
	trackedBytes, err := fsys.ReadFile(tracked)
	if err != nil {
		if os.IsNotExist(err) {
			return []Finding{{
				Scope:      ScopePackages,
				Kind:       KindBrewTrackedFileMissing,
				SourcePath: tracked,
				Status:     StatusMissing,
				Details:    "tracked Brewfile does not exist",
				CanApply:   true,
			}}, nil
		}
		return nil, err
	}
	if bytes.Equal(snapshot, trackedBytes) {
		return []Finding{}, nil
	}
	return []Finding{{
		Scope:      ScopePackages,
		Kind:       KindBrewfileDrift,
		SourcePath: tracked,
		Status:     StatusDrift,
		Details:    "tracked Brewfile differs from current brew bundle dump",
		CanApply:   true,
	}}, nil
}

func Apply(ctx Context) (ApplyResult, error) {
	scope, err := normalizeScope(ctx.Scope)
	if err != nil {
		return ApplyResult{}, err
	}
	initial, err := Inspect(Context{
		Scope:    scope,
		RepoPath: ctx.RepoPath,
		HomePath: ctx.HomePath,
		Entries:  ctx.Entries,
		DryRun:   ctx.DryRun,
		Logger:   ctx.Logger,
	})
	if err != nil {
		return ApplyResult{}, err
	}

	result := ApplyResult{
		Applied: []Finding{},
		Skipped: []Finding{},
		Failed:  []Finding{},
		Backups: []backup.BackupEntry{},
	}

	if includesScope(scope, ScopeFiles) {
		applied, skipped, failed, backups := ApplyFiles(ctx, initial.Findings)
		result.Applied = append(result.Applied, applied...)
		result.Skipped = append(result.Skipped, skipped...)
		result.Failed = append(result.Failed, failed...)
		result.Backups = append(result.Backups, backups...)
	}
	if includesScope(scope, ScopePackages) {
		applied, skipped, failed, err := ApplyBrew(ctx, initial.Findings)
		if err != nil {
			return ApplyResult{}, err
		}
		result.Applied = append(result.Applied, applied...)
		result.Skipped = append(result.Skipped, skipped...)
		result.Failed = append(result.Failed, failed...)
	}

	if ctx.DryRun {
		result.RemainingDrift = driftOnly(initial.Findings)
		return result, nil
	}

	final, err := Inspect(Context{
		Scope:    scope,
		RepoPath: ctx.RepoPath,
		HomePath: ctx.HomePath,
		Entries:  ctx.Entries,
		Logger:   ctx.Logger,
	})
	if err != nil {
		return ApplyResult{}, err
	}
	result.RemainingDrift = driftOnly(final.Findings)
	return result, nil
}

func ApplyFiles(ctx Context, findings []Finding) ([]Finding, []Finding, []Finding, []backup.BackupEntry) {
	lookup := map[string]symlink.Entry{}
	for _, entry := range ctx.Entries {
		lookup[entry.TargetAbs] = entry
	}

	applied := []Finding{}
	skipped := []Finding{}
	failed := []Finding{}
	backups := []backup.BackupEntry{}
	logger := ctx.Logger
	if logger == nil {
		logger = output.NewLogger("text", true, false)
	}

	for _, finding := range findings {
		if finding.Scope != ScopeFiles {
			continue
		}
		if !isFileDriftKind(finding.Kind) {
			continue
		}
		if !finding.CanApply {
			skipped = append(skipped, withDetail(finding, "cannot be imported automatically"))
			continue
		}

		entry, ok := lookup[finding.TargetPath]
		if !ok {
			failed = append(failed, withDetail(finding, "managed entry not found"))
			continue
		}
		if err := validateSourceWithinRepo(entry.SourceAbs, ctx.RepoPath); err != nil {
			failed = append(failed, withDetail(finding, err.Error()))
			continue
		}
		targetBytes, err := fsys.ReadFile(entry.TargetAbs)
		if err != nil {
			failed = append(failed, withDetail(finding, err.Error()))
			continue
		}

		if ctx.DryRun {
			skipped = append(skipped, withDetail(finding, "dry-run: would import target into repo and relink"))
			continue
		}

		if err := fsys.MkdirAll(filepath.Dir(entry.SourceAbs), 0o755); err != nil {
			failed = append(failed, withDetail(finding, err.Error()))
			continue
		}
		if err := fsys.WriteFile(entry.SourceAbs, targetBytes, 0o644); err != nil {
			failed = append(failed, withDetail(finding, err.Error()))
			continue
		}

		states, err := symlink.Create([]symlink.Entry{entry}, symlink.LinkOptions{
			DryRun:        false,
			Force:         true,
			NoInteractive: true,
		}, logger)
		if err != nil {
			failed = append(failed, withDetail(finding, err.Error()))
			continue
		}
		if len(states) > 0 {
			st := states[0]
			if st.Status != symlink.StatusLinked && st.Status != symlink.StatusBackup {
				failed = append(failed, withDetail(finding, fmt.Sprintf("relink status: %s", st.Status)))
				continue
			}
			if strings.TrimSpace(st.BackupPath) != "" {
				backups = append(backups, backup.BackupEntry{
					Original:  st.Target,
					Backup:    st.BackupPath,
					Timestamp: clk.Now().UnixMilli(),
				})
			}
		}
		applied = append(applied, withDetail(finding, "imported target content and relinked"))
	}

	return applied, skipped, failed, backups
}

func ApplyBrew(ctx Context, findings []Finding) ([]Finding, []Finding, []Finding, error) {
	targetKinds := []FindingKind{KindBrewfileDrift, KindBrewTrackedFileMissing}
	targetFindings := []Finding{}
	for _, finding := range findings {
		if finding.Scope != ScopePackages {
			continue
		}
		if slices.Contains(targetKinds, finding.Kind) {
			targetFindings = append(targetFindings, finding)
		}
	}
	if len(targetFindings) == 0 {
		return []Finding{}, []Finding{}, []Finding{}, nil
	}
	if ctx.DryRun {
		skipped := make([]Finding, 0, len(targetFindings))
		for _, finding := range targetFindings {
			skipped = append(skipped, withDetail(finding, "dry-run: would rewrite tracked Brewfile"))
		}
		return []Finding{}, skipped, []Finding{}, nil
	}
	if _, err := runner.LookPath("brew"); err != nil {
		failed := make([]Finding, 0, len(targetFindings))
		for _, finding := range targetFindings {
			failed = append(failed, withDetail(finding, "brew is not available in PATH"))
		}
		return []Finding{}, []Finding{}, failed, nil
	}
	snapshot, err := dumpBrewfileSnapshot()
	if err != nil {
		return nil, nil, nil, err
	}
	applied := []Finding{}
	failed := []Finding{}
	for _, finding := range targetFindings {
		if err := fsys.MkdirAll(filepath.Dir(finding.SourcePath), 0o755); err != nil {
			failed = append(failed, withDetail(finding, err.Error()))
			continue
		}
		if err := fsys.WriteFile(finding.SourcePath, snapshot, 0o644); err != nil {
			failed = append(failed, withDetail(finding, err.Error()))
			continue
		}
		applied = append(applied, withDetail(finding, "rewrote tracked Brewfile from brew bundle dump"))
	}
	return applied, []Finding{}, failed, nil
}

func summarize(findings []Finding) Summary {
	summary := Summary{
		ByKind:   map[string]int{},
		ByScope:  map[string]int{},
		ByStatus: map[string]int{},
	}
	for _, finding := range findings {
		summary.ByKind[string(finding.Kind)]++
		summary.ByScope[string(finding.Scope)]++
		summary.ByStatus[finding.Status]++
		if finding.CanApply {
			summary.CanApply++
		} else {
			summary.CannotApply++
		}
		if isDriftFinding(finding) {
			summary.DriftCount++
			switch finding.Scope {
			case ScopeFiles:
				summary.FilesDrift++
			case ScopePackages:
				summary.PackageDrift++
			}
		}
	}
	return summary
}

func classifyConflict(entry symlink.Entry) (Finding, error) {
	info, err := fsys.Lstat(entry.TargetAbs)
	if err != nil {
		return Finding{}, err
	}
	f := Finding{
		Scope:      ScopeFiles,
		SourcePath: entry.SourceAbs,
		TargetPath: entry.TargetAbs,
		Status:     StatusConflict,
		CanApply:   false,
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		f.Kind = KindFileUnsupportedSymlinkTarget
		f.Details = "target is a symlink and cannot be imported automatically"
	case info.IsDir():
		f.Kind = KindFileUnsupportedDirTarget
		f.Details = "target is a directory and cannot be imported automatically"
	case info.Mode().IsRegular():
		f.Kind = KindFileConflict
		f.CanApply = true
		f.Details = "target differs from managed source"
	default:
		f.Kind = KindFileConflict
		f.Details = "target type is not importable"
	}
	return f, nil
}

func classifySourceMissing(entry symlink.Entry) (Finding, bool, error) {
	info, err := fsys.Lstat(entry.TargetAbs)
	if err != nil {
		if os.IsNotExist(err) {
			return Finding{}, false, nil
		}
		return Finding{}, false, err
	}
	f := Finding{
		Scope:      ScopeFiles,
		SourcePath: entry.SourceAbs,
		TargetPath: entry.TargetAbs,
		Status:     symlink.StatusSourceMissing,
		CanApply:   false,
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		f.Kind = KindFileUnsupportedSymlinkTarget
		f.Details = "source is missing and target symlink cannot be imported automatically"
	case info.IsDir():
		f.Kind = KindFileUnsupportedDirTarget
		f.Details = "source is missing and target directory cannot be imported automatically"
	case info.Mode().IsRegular():
		f.Kind = KindFileSourceMissingTarget
		f.CanApply = true
		f.Details = "source is missing but target file exists and can be imported"
	default:
		f.Kind = KindFileSourceMissingTarget
		f.Details = "source is missing but target type is not importable"
	}
	return f, true, nil
}

func dumpBrewfileSnapshot() ([]byte, error) {
	tmpDir, err := os.MkdirTemp("", "paw-drift-brew-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	tmpPath := filepath.Join(tmpDir, "Brewfile")
	if err := runner.Run("brew", "bundle", "dump", "--file", tmpPath, "--force"); err != nil {
		return nil, err
	}
	return fsys.ReadFile(tmpPath)
}

func withDetail(f Finding, detail string) Finding {
	result := f
	if strings.TrimSpace(detail) == "" {
		return result
	}
	if strings.TrimSpace(result.Details) == "" {
		result.Details = detail
		return result
	}
	result.Details = result.Details + "; " + detail
	return result
}

func normalizeScope(scope Scope) (Scope, error) {
	if scope == "" {
		return ScopeAll, nil
	}
	switch scope {
	case ScopeAll, ScopeFiles, ScopePackages:
		return scope, nil
	default:
		return "", fmt.Errorf("invalid scope %q (expected all|files|packages)", scope)
	}
}

func includesScope(current Scope, expected Scope) bool {
	return current == ScopeAll || current == expected
}

func driftOnly(findings []Finding) []Finding {
	result := []Finding{}
	for _, finding := range findings {
		if isDriftFinding(finding) {
			result = append(result, finding)
		}
	}
	return result
}

func isDriftFinding(finding Finding) bool {
	switch finding.Kind {
	case KindFileConflict, KindFileSourceMissingTarget, KindFileMissing, KindBrewfileDrift, KindBrewTrackedFileMissing, KindFileUnsupportedDirTarget, KindFileUnsupportedSymlinkTarget:
		return true
	default:
		return false
	}
}

func isFileDriftKind(kind FindingKind) bool {
	switch kind {
	case KindFileConflict, KindFileSourceMissingTarget, KindFileMissing, KindFileUnsupportedDirTarget, KindFileUnsupportedSymlinkTarget:
		return true
	default:
		return false
	}
}

func validateSourceWithinRepo(sourcePath string, repoPath string) error {
	if strings.TrimSpace(repoPath) == "" {
		return nil
	}
	base := repoPath
	if !filepath.IsAbs(base) {
		abs, err := filepath.Abs(base)
		if err == nil {
			base = abs
		}
	}
	source := sourcePath
	if !filepath.IsAbs(source) {
		source = filepath.Join(base, source)
	}
	if err := repo.ValidateWithinBase(source, base); err == nil {
		return nil
	}

	evalBase, baseErr := filepath.EvalSymlinks(base)
	evalSourceDir, sourceErr := filepath.EvalSymlinks(filepath.Dir(source))
	if baseErr == nil && sourceErr == nil {
		evalSource := filepath.Join(evalSourceDir, filepath.Base(source))
		if err := repo.ValidateWithinBase(evalSource, evalBase); err == nil {
			return nil
		}
	}
	return fmt.Errorf("path escapes allowed base")
}
