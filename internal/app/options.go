package app

// GlobalOptions are shared command flags.
type GlobalOptions struct {
	DryRun        bool
	Force         bool
	Verbose       bool
	SkipPackages  bool
	SkipHooks     bool
	NoInteractive bool
	Quiet         bool
	Path          string
	SkipUpdate    bool
	AutoUpdate    bool
	JSON          bool
	SkipVerify    bool
	LogFormat     string
}

const (
	ExitOK            = 0
	ExitUsage         = 2
	ExitConfig        = 3
	ExitSystem        = 4
	ExitConflict      = 5
	ExitHookFailure   = 6
	ExitRollbackError = 7
)
