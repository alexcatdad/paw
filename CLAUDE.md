# Paw - Go Dotfiles Manager

`paw` is a Go CLI for managing dotfiles across Linux, macOS, and WSL.

## Development

```bash
go test ./...
go build ./cmd/paw
```

## Core Commands

```bash
paw init <repo-url>
paw install
paw link
paw unlink
paw status
paw sync
paw push [message]
paw update
paw rollback
paw backup list|restore|clean
paw audit
paw scaffold
paw doctor
paw migrate-ts-config
paw completion [bash|zsh|fish]
paw man
```

## Architecture

```text
cmd/paw/main.go
internal/
  app/        # shared flags and exit codes
  cli/        # cobra command wiring
  config/     # strict paw.toml load/validate + TS migration helper
  platform/   # linux/darwin/wsl detection and command checks
  repo/       # repo discovery, init/push/sync helpers, paw config state
  symlink/    # hybrid auto-link + overrides + conflict + transaction + lock
  backup/     # backup list/restore/clean + rollback state
  packages/   # brew/apt install and package checks
  hooks/      # shell command hook execution
  update/     # self-update from GitHub releases
  audit/      # structure audit and scoring
  scaffold/   # template scaffolding
  output/     # text/json logger
  state/      # state file paths
```

## Config Model

- Native-only `paw.toml` (`version = 1` required)
- Layout is `hybrid`
- `home/` mirrors `$HOME` and links automatically
- `[overrides]` for exceptional/conditional paths
- hooks are shell command strings

## State Files

- `~/.config/paw/config.json` - dotfiles repo metadata
- `~/.config/paw/state/last-run.json` - rollback metadata
- `~/.config/paw/state/update-state.json` - update check cache
- `~/.config/paw/state/lock` - operation lock file
