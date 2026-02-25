package drift

import "github.com/alexcatdad/paw/internal/backup"

type Scope string

const (
	ScopeAll      Scope = "all"
	ScopeFiles    Scope = "files"
	ScopePackages Scope = "packages"
)

type FindingKind string

const (
	KindFileConflict                 FindingKind = "file_conflict"
	KindFileSourceMissingTarget      FindingKind = "file_source_missing_target_exists"
	KindFileMissing                  FindingKind = "file_missing"
	KindBrewfileDrift                FindingKind = "brewfile_drift"
	KindBrewUnavailable              FindingKind = "brew_unavailable"
	KindBrewTrackedFileMissing       FindingKind = "brewfile_missing"
	KindFileUnsupportedSymlinkTarget FindingKind = "file_conflict_symlink_target"
	KindFileUnsupportedDirTarget     FindingKind = "file_conflict_dir_target"
)

type Finding struct {
	Scope      Scope       `json:"scope"`
	Kind       FindingKind `json:"kind"`
	SourcePath string      `json:"sourcePath,omitempty"`
	TargetPath string      `json:"targetPath,omitempty"`
	Status     string      `json:"status"`
	Details    string      `json:"details,omitempty"`
	CanApply   bool        `json:"canApply"`
}

type Summary struct {
	DriftCount   int            `json:"driftCount"`
	FilesDrift   int            `json:"filesDrift"`
	PackageDrift int            `json:"packageDrift"`
	ByKind       map[string]int `json:"byKind"`
	ByScope      map[string]int `json:"byScope"`
	ByStatus     map[string]int `json:"byStatus"`
	CanApply     int            `json:"canApply"`
	CannotApply  int            `json:"cannotApply"`
}

type Report struct {
	Timestamp string    `json:"timestamp"`
	RepoPath  string    `json:"repoPath"`
	HomePath  string    `json:"homePath"`
	Scope     Scope     `json:"scope"`
	Findings  []Finding `json:"findings"`
	Summary   Summary   `json:"summary"`
}

type ApplyResult struct {
	Applied        []Finding            `json:"applied"`
	Skipped        []Finding            `json:"skipped"`
	Failed         []Finding            `json:"failed"`
	Backups        []backup.BackupEntry `json:"backups"`
	RemainingDrift []Finding            `json:"remainingDrift"`
}
