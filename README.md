# paw 🐱

`paw` is a Go-based dotfiles manager for Linux, macOS, and WSL.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/alexcatdad/paw/main/install.sh | bash
```

## Quick Start

```bash
paw init https://github.com/yourusername/dotfiles
paw install
paw status
```

## Core Commands

- `paw init <repo>`
- `paw install`
- `paw link`
- `paw unlink`
- `paw status`
- `paw sync`
- `paw push [message]`
- `paw update`
- `paw rollback`
- `paw backup list|restore|clean`
- `paw audit`
- `paw scaffold`
- `paw doctor`
- `paw migrate-ts-config`
- `paw completion [bash|zsh|fish]`
- `paw man`

## Config (`paw.toml`)

`paw` is native-only TOML. No runtime TS/JS execution.

```toml
version = 1
layout = "hybrid"

[packages]
common = ["ripgrep", "fzf"]
darwin = ["ghostty"]
linux_apt = ["git", "curl"]
linux_brew = []
wsl_apt = ["git", "curl"]
wsl_brew = []

[ignore]
paths = [".zshrc.local", ".gitconfig.local"]

[backup]
enabled = true
max_age = 30
max_count = 5

[hooks]
post_install = "echo installed"

[overrides]
"extras/ssh-config" = { target = ".ssh/config", platform = ["darwin", "linux", "wsl"] }
```

## Hybrid Layout

Store managed files under `home/` and paw links them into `$HOME`:

```text
home/.zshrc                   -> ~/.zshrc
home/.config/git/config       -> ~/.config/git/config
home/.config/starship.toml    -> ~/.config/starship.toml
```

Use `[overrides]` for exceptions and conditional links.

## Security Notes

- Hooks are shell command strings and run with user permissions.
- Paths are validated to stay inside `$HOME` (targets) and repo (sources).
- Package names are validated before shell execution.

## Testing

Deterministic Linux CI/local run:

```bash
./scripts/test/docker-ci.sh
```

Coverage stage gate (`65`, `80`, `90`) and package minima:

```bash
COVERAGE_STAGE=65 ./scripts/test/coverage-check.sh
```

## License

MIT
