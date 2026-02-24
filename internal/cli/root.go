package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/alexcatdad/paw/internal/app"
	"github.com/alexcatdad/paw/internal/backup"
	"github.com/alexcatdad/paw/internal/clock"
	"github.com/alexcatdad/paw/internal/execx"
	"github.com/alexcatdad/paw/internal/fsx"
	"github.com/alexcatdad/paw/internal/hooks"
	"github.com/alexcatdad/paw/internal/output"
	"github.com/alexcatdad/paw/internal/packages"
	"github.com/alexcatdad/paw/internal/repo"
	"github.com/alexcatdad/paw/internal/symlink"
	"github.com/alexcatdad/paw/internal/update"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

type Dependencies struct {
	Runner execx.Runner
	Clock  clock.Clock
	FS     fsx.FS
}

func NewRootCommand() *cobra.Command {
	return NewRootCommandWithDeps(Dependencies{})
}

func NewRootCommandWithDeps(deps Dependencies) *cobra.Command {
	if deps.Runner == nil {
		deps.Runner = execx.NewOSRunner()
	}
	if deps.Clock == nil {
		deps.Clock = clock.RealClock{}
	}
	if deps.FS == nil {
		deps.FS = fsx.NewOSFS()
	}
	repo.SetDependencies(deps.Runner, deps.Clock)
	update.SetDependencies(deps.Runner, deps.FS, deps.Clock)
	packages.SetDependencies(deps.Runner)
	hooks.SetDependencies(deps.Runner)
	symlink.SetDependencies(deps.Runner, deps.FS, deps.Clock)
	backup.SetDependencies(deps.FS, deps.Clock)

	opts := &app.GlobalOptions{}

	cmd := &cobra.Command{
		Use:           "paw",
		Short:         "Personal dotfiles manager",
		Long:          "paw is a dotfiles manager for Linux, macOS, and WSL.",
		Version:       Version(),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cmd, opts)
		},
	}
	cmd.CompletionOptions.DisableDefaultCmd = true

	cmd.SetVersionTemplate("paw v{{.Version}}\n")
	flags := cmd.PersistentFlags()
	flags.BoolVarP(&opts.DryRun, "dry-run", "n", false, "Show what would be done")
	flags.BoolVarP(&opts.Force, "force", "f", false, "Overwrite existing files")
	flags.BoolVarP(&opts.Verbose, "verbose", "v", false, "Verbose output")
	flags.BoolVar(&opts.SkipPackages, "skip-packages", false, "Skip package installation")
	flags.BoolVar(&opts.SkipHooks, "skip-hooks", false, "Skip lifecycle hooks")
	flags.BoolVar(&opts.NoInteractive, "no-interactive", false, "Disable interactive prompts")
	flags.BoolVarP(&opts.Quiet, "quiet", "q", false, "Quiet output")
	flags.StringVarP(&opts.Path, "path", "p", "", "Path used by init command")
	flags.BoolVar(&opts.SkipUpdate, "skip-update", false, "Skip update check")
	flags.BoolVar(&opts.AutoUpdate, "auto-update", false, "Auto update during sync")
	flags.BoolVar(&opts.JSON, "json", false, "JSON output mode")
	flags.BoolVar(&opts.SkipVerify, "skip-verify", false, "Skip attestation verification in update")
	flags.StringVar(&opts.LogFormat, "log-format", "text", "Log format: text|json")

	cmd.AddCommand(
		newInstallCommand(opts),
		newLinkCommand(opts),
		newUnlinkCommand(opts),
		newStatusCommand(opts),
		newBackupCommand(opts),
		newRollbackCommand(opts),
		newSyncCommand(opts),
		newInitCommand(opts),
		newPushCommand(opts),
		newUpdateCommand(opts),
		newDoctorCommand(opts),
		newAuditCommand(opts),
		newScaffoldCommand(opts),
		newMigrateCommand(opts),
		newCompletionCommand(cmd),
		newManCommand(cmd),
	)

	return cmd
}

func newLogger(opts *app.GlobalOptions) *output.Logger {
	return output.NewLogger(opts.LogFormat, opts.Quiet, opts.Verbose)
}

func newCompletionCommand(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish]",
		Short: "Generate shell completions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := root.OutOrStdout()
			switch args[0] {
			case "bash":
				return root.GenBashCompletion(out)
			case "zsh":
				return root.GenZshCompletion(out)
			case "fish":
				return root.GenFishCompletion(out, true)
			default:
				return fmt.Errorf("unsupported shell %q", args[0])
			}
		},
	}
	return cmd
}

func newManCommand(root *cobra.Command) *cobra.Command {
	var outDir string
	cmd := &cobra.Command{
		Use:   "man",
		Short: "Generate man pages",
		RunE: func(cmd *cobra.Command, args []string) error {
			if outDir == "" {
				outDir = filepath.Join("docs", "man")
			}
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return err
			}
			header := &doc.GenManHeader{Title: "PAW", Section: "1", Source: "paw", Manual: "paw Manual"}
			return doc.GenManTree(root, header, outDir)
		},
	}
	cmd.Flags().StringVar(&outDir, "dir", "", "Output directory for generated man pages")
	return cmd
}
