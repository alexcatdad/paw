# paw 🐱

Personal dotfiles manager CLI built with TypeScript and Bun.

---

## A BIG ASS WARNING

**USE AT YOUR OWN RISK.**

This tool makes potentially **destructive changes** to your home directory:

- **Creates symlinks** that overwrite existing files (backs up originals, but still)
- **Executes arbitrary code** from your `dotfiles.config.ts` (hooks run with your full permissions)
- **Installs packages** via Homebrew/apt (runs `brew install`, `sudo apt install`)
- **Self-updates** by downloading binaries from GitHub

This is an open source tool with **no warranty**. Review the code, understand what it does, and use it at your own risk. If you're not comfortable with a tool modifying `~/.zshrc`, `~/.gitconfig`, etc., this isn't for you.

**You have been warned.**

---

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/alexcatdad/paw/main/install.sh | bash
```

## Quick Start

```bash
# Initialize with your dotfiles repo
paw init https://github.com/yourusername/dotfiles

# Or if you already have dotfiles cloned
paw install

# Preview changes without doing anything
paw install --dry-run
```

## Commands

| Command | Description |
|---------|-------------|
| `paw init <repo>` | Clone dotfiles repo and run initial setup |
| `paw install` | Full setup: install packages and create symlinks |
| `paw link` | Create symlinks only (skip packages) |
| `paw unlink` | Remove all managed symlinks |
| `paw sync` | Pull dotfiles repo and refresh links |
| `paw push [msg]` | Commit and push dotfiles changes |
| `paw status` | Show current symlink and package state |
| `paw audit` | Analyze repo structure and completeness |
| `paw scaffold` | List or generate missing config templates |
| `paw doctor` | Health check and diagnostics |
| `paw update` | Update paw binary (self-update) |
| `paw rollback` | Restore backups from last run |
| `paw backup list` | List backup files |
| `paw backup restore` | Restore a specific backup |
| `paw backup clean` | Remove old backups |

## Options

| Flag | Description |
|------|-------------|
| `-n, --dry-run` | Preview changes without making them |
| `-f, --force` | Overwrite existing files (creates backups) |
| `-v, --verbose` | Show detailed output |
| `-q, --quiet` | Suppress output (for sync in shell startup) |
| `--json` | Output as JSON (audit command) |
| `--skip-packages` | Skip package installation |
| `--skip-hooks` | Skip pre/post hooks |

## Configuration

Paw looks for `dotfiles.config.ts` in your dotfiles repo:

```typescript
import type { DotfilesConfig } from "paw";

export default {
  symlinks: {
    "shell/zshrc": ".zshrc",
    "git/gitconfig": ".gitconfig",
    "starship/starship.toml": ".config/starship.toml",
  },
  packages: {
    common: ["starship", "eza", "fzf", "zoxide", "ripgrep"],
    darwin: ["coreutils"],
    linux: {
      apt: ["build-essential"],
      brew: ["gcc"],
    },
  },
  templates: {
    "templates/gitconfig.local": ".gitconfig.local",
  },
  ignore: [],
  backup: {
    enabled: true,
    maxAge: 30,
    maxCount: 5,
  },
  hooks: {
    preInstall: async (ctx) => {
      // Runs before installation
    },
    postInstall: async (ctx) => {
      // Runs after installation
    },
  },
} satisfies DotfilesConfig;
```

## Auditing Your Dotfiles

Check what's missing from your setup:

```bash
$ paw audit

═══ Dotfiles Audit ═══

  Repository  ~/dotfiles
  Score       75/100

─── Missing ───

⚠ Missing SSH Config: SSH configuration and keys setup
→ Missing Tmux: Terminal multiplexer configuration

─── Summary ───

  Errors       0
  Warnings     1
  Suggestions  1
```

Generate missing configs:

```bash
$ paw scaffold list              # See available templates
$ paw scaffold "Git Config"      # Generate git config template
```

## Cross-Machine Sync

On machine A (making changes):
```bash
paw push "update zsh config"
```

On machine B (shell startup auto-syncs):
```
✓ Dotfiles synced (3 files updated)
```

Add to your `.zshrc` for automatic sync:
```bash
paw sync --quiet
```

## Building from Source

```bash
git clone https://github.com/alexcatdad/paw.git
cd paw
bun install
bun run dev status        # Run locally
bun run typecheck         # Type check
bun run build:all         # Build all platform binaries
```

## Security

- **Path validation**: Symlinks and scaffolded files are validated to stay within allowed directories
- **Package name validation**: Package names are validated to prevent command injection
- **Backup before overwrite**: Original files are backed up before being replaced
- **Dry-run mode**: Preview all changes before applying them

**However**: Your `dotfiles.config.ts` is executed as code. Only use dotfiles repos you trust.

## License

MIT
