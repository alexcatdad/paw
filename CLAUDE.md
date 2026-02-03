# Paw - Personal Dotfiles Manager

A CLI tool for managing dotfiles across machines with automatic synchronization, package installation, and symlink management.

## Quick Reference

```bash
# Development
bun run dev <command>     # Run locally
bun run typecheck         # Type check
bun run build:all         # Build all platform binaries

# Common commands
paw init <repo-url>       # Clone dotfiles and setup
paw install               # Full install (packages + symlinks)
paw sync                  # Pull and refresh
paw status                # Current state
paw audit                 # Analyze repo completeness
paw scaffold list         # Show scaffoldable configs
```

## Architecture

```
src/
├── index.ts              # CLI entry point, command routing
├── types/index.ts        # All TypeScript interfaces
└── core/
    ├── audit.ts          # Repo analysis and scoring
    ├── audit-patterns.ts # Common dotfile patterns
    ├── backup.ts         # Backup/restore/rollback
    ├── config.ts         # Load dotfiles.config.ts
    ├── init.ts           # Clone and setup
    ├── logger.ts         # Colored console output
    ├── os.ts             # Platform detection, paths
    ├── packages.ts       # Homebrew/apt installation
    ├── paw-config.ts     # ~/.config/paw/config.json
    ├── push.ts           # Commit and push
    ├── scaffold.ts       # Generate missing configs
    ├── suggestions.ts    # Config file suggestions
    ├── symlinks.ts       # Symlink creation/status
    ├── sync.ts           # Pull and refresh
    ├── templates.ts      # Template generation
    └── update.ts         # Self-update
```

## Patterns

### Adding a New Command

1. Add types to `src/types/index.ts` if needed
2. Create module in `src/core/<command>.ts`
3. Add command function in `src/index.ts`
4. Add case to switch statement
5. Update help text

### Logger Usage

```typescript
import { logger } from "./logger";

logger.header("Section");     // ═══ Section ═══
logger.subheader("Sub");      // ─── Sub ───
logger.success("Done");       // ✓ Done
logger.error("Failed");       // ✗ Failed
logger.warn("Warning");       // ⚠ Warning
logger.info("Info");          // → Info
logger.skip("Skipped");       // ○ Skipped
logger.dryRun("Would do");    // ◌ [dry-run] Would do
logger.table({ Key: "val" }); // Key  val
```

## User's Dotfiles Config

Users create `dotfiles.config.ts` in their repo:

```typescript
import { defineConfig } from "paw";

export default defineConfig({
  symlinks: {
    "shell/zshrc": ".zshrc",
    "git/gitconfig": ".gitconfig",
  },
  packages: {
    common: ["starship", "eza", "fzf"],
    darwin: ["coreutils"],
  },
  templates: {},
  ignore: [],
});
```

## State Files

- `~/.config/paw/config.json` - Repo path and URL
- `~/.config/paw/last-run.json` - Backups and symlinks for rollback
- `~/.config/paw/update-state.json` - Version check cache
