# GitHub Pages Demo Site

## Overview

Single-page landing site with embedded terminal demo, served from `docs/index.html` via GitHub Pages on `main` branch.

## Tech

- Single HTML file (`docs/index.html`)
- Tailwind CSS via CDN
- Web components for reuse (no framework, no build step)
- System monospace font stack — no web fonts

## Brutalist Aesthetic

- High contrast black/white, one accent color (terminal green or amber)
- Chunky borders, no rounded corners, no shadows
- Oversized monospace headings
- Visible structure — thick borders as dividers, content blocks feel like terminal panes
- No icons, no gradients, no SVGs
- Blunt, direct copy matching paw's README personality

## Typography

```
ui-monospace, 'Cascadia Code', 'Source Code Pro', Menlo, Consolas, monospace
```

## Page Sections

1. **Hero** — Name, tagline, install command with copy button
2. **Terminal demo** — Auto-typing animation cycling through commands
3. **Features** — Text-heavy bordered blocks (no icons), covering: cross-machine sync, backup/rollback, audit scoring, scaffold templates, dry-run safety, lifecycle hooks
4. **Config example** — `dotfiles.config.ts` sample in a terminal-style pane
5. **Footer** — GitHub link, MIT, "Fork it and make it your own"

## Web Components

### `<terminal-demo>`

Auto-typing terminal animation that cycles through scenes:

1. `paw status` — symlink/package state with colored output
2. `paw audit` — scoring, warnings, suggestions
3. `paw install --dry-run` — dry-run preview

Behavior:
- Types command character by character
- Reveals output line by line
- Pause between scenes
- Loops indefinitely

### `<feature-card>`

Bordered box with bold label and description text. No icons.

### `<code-block>`

Terminal-pane styled code display with optional copy button. Used for install command and config example.

## Terminal Demo Output

Simulated output based on real paw logger formatting:

### Scene 1: `paw status`
```
═══ Dotfiles Status ═══

  Repository  ~/dotfiles
  Branch      main (clean)

─── Symlinks ───

✓ shell/zshrc        → ~/.zshrc
✓ git/gitconfig      → ~/.gitconfig
✓ starship.toml      → ~/.config/starship.toml
○ tmux.conf          → ~/.tmux.conf (skipped)

─── Packages ───

✓ starship eza fzf zoxide ripgrep
⚠ coreutils (not installed)
```

### Scene 2: `paw audit`
```
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

### Scene 3: `paw install --dry-run`
```
═══ Install (dry-run) ═══

─── Packages ───

◌ [dry-run] Would install: coreutils

─── Symlinks ───

◌ [dry-run] Would link shell/zshrc → ~/.zshrc
◌ [dry-run] Would link git/gitconfig → ~/.gitconfig
◌ [dry-run] Would link starship.toml → ~/.config/starship.toml

─── Summary ───

  Packages   1 to install
  Symlinks   3 to create
  Backups    2 files would be backed up
```
