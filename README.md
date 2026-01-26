# paw 🐱

Personal dotfiles manager CLI built with TypeScript and Bun.

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
```

## Commands

| Command | Description |
|---------|-------------|
| `paw init <repo>` | Clone dotfiles repo and run initial setup |
| `paw install` | Full setup: install packages and create symlinks |
| `paw link` | Create symlinks only |
| `paw sync` | Pull dotfiles repo and refresh links |
| `paw push [msg]` | Commit and push dotfiles changes |
| `paw update` | Update paw binary |
| `paw status` | Show current state |
| `paw doctor` | Health check |

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
  },
  templates: {
    "templates/gitconfig.local": ".gitconfig.local",
  },
} satisfies DotfilesConfig;
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

## Building from Source

```bash
git clone https://github.com/alexcatdad/paw.git
cd paw
bun install
bun run build
```

## License

MIT
