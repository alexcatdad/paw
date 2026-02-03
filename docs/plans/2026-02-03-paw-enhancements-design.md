# Paw Enhancements Design

**Date:** 2026-02-03
**Status:** Approved

## Overview

This document outlines five enhancements to the paw dotfiles manager:

1. E2E Testing with Docker
2. Interactive Conflict Resolution
3. Binary Verification via GitHub Attestations
4. Full Lifecycle Hooks
5. Machine-Specific Configs

---

## 1. E2E Testing

### Goals

- Confidence in releases (test critical paths before releasing)
- Regression prevention (catch bugs when modifying code)
- Full coverage of all commands

### Structure

```
tests/
├── e2e/
│   ├── docker/
│   │   └── Dockerfile          # Clean Ubuntu/Debian with Bun
│   ├── helpers/
│   │   ├── fixtures.ts         # Generate test repos on-the-fly
│   │   ├── docker.ts           # Container lifecycle management
│   │   └── assertions.ts       # Custom matchers (symlink exists, etc.)
│   ├── init.test.ts
│   ├── install.test.ts
│   ├── link.test.ts
│   ├── unlink.test.ts
│   ├── sync.test.ts
│   ├── push.test.ts
│   ├── rollback.test.ts
│   ├── status.test.ts
│   ├── doctor.test.ts
│   ├── audit.test.ts
│   └── scaffold.test.ts
└── bun.test.ts                 # Test config
```

### Fixture Generator API

```typescript
// Generate a minimal dotfiles repo for testing
const repo = await createTestRepo({
  symlinks: { "shell/zshrc": ".zshrc" },
  packages: { common: ["starship"] },
  // Optional: create conflicting files in home dir
  conflicts: [".zshrc"],
});
```

### Docker Strategy

- Base image: `oven/bun:latest` on Debian
- Each test gets fresh container with clean `$HOME`
- Mount built `paw` binary into container
- Tests run `paw` commands via `docker exec`
- Container destroyed after each test

### Commands to Test

| Command | Test Scenarios |
|---------|----------------|
| `init` | Clone repo, invalid URL, existing repo |
| `install` | Fresh install, with conflicts, dry-run, skip-packages |
| `link` | Create symlinks, force overwrite, source missing |
| `unlink` | Remove symlinks, partial removal |
| `sync` | Pull changes, conflict handling, link refresh |
| `push` | Commit changes, empty commit, custom message |
| `rollback` | Restore backups, no previous state |
| `status` | Show state, missing symlinks |
| `doctor` | Health check, missing tools |
| `audit` | Score calculation, findings |
| `scaffold` | Generate configs, list available |

---

## 2. Interactive Conflict Resolution

### User Experience

When a conflict is detected during `paw install` or `paw link` (without `--force`):

```
Creating symlinks...
✓ ~/.gitconfig (linked)
⚠ ~/.zshrc already exists

  [s] Skip this file
  [b] Backup & link
  [o] Overwrite (no backup)
  [d] Show diff
  [a] Abort
  ─────────────────────
  [S] Skip all remaining
  [B] Backup all remaining

Choice [s/b/o/d/a/S/B]:
```

### Implementation

New file: `src/core/prompt.ts`

```typescript
export interface ConflictChoice {
  action: "skip" | "backup" | "overwrite" | "abort";
  applyToAll?: boolean;
}

export async function promptConflict(
  targetPath: string,
  sourcePath: string
): Promise<ConflictChoice>;

export async function showDiff(
  existingPath: string,
  sourcePath: string
): Promise<void>;
```

### Behavior

- `--force` bypasses prompts (existing behavior, always creates backups)
- `--no-interactive` or piped stdin → skip all conflicts with warning
- Diff uses `diff -u` or falls back to side-by-side comparison
- Choice state persists across conflicts in same run (for "all remaining" options)

### Changes Required

- `src/core/prompt.ts` - New file for interactive prompts
- `src/core/symlinks.ts` - Integrate conflict prompts into `createSymlinks()`
- `src/index.ts` - Add `--no-interactive` flag

---

## 3. Binary Verification

### Approach

Use GitHub's artifact attestations with Sigstore for cryptographic verification.

### Build Changes

In `.github/workflows/release.yml`:

```yaml
jobs:
  build:
    permissions:
      id-token: write      # Required for attestation
      contents: write
      attestations: write

    steps:
      - name: Build binaries
        run: bun run build:all

      - name: Attest artifacts
        uses: actions/attest-build-provenance@v2
        with:
          subject-path: 'dist/paw-*'
```

### Verification in `update.ts`

```typescript
async function downloadAndVerifyBinary(
  targetDir: string,
  options: InstallOptions
): Promise<string | null> {
  const binaryName = getBinaryName();

  // Download with attestation verification
  const result = await $`gh release download latest \
    --repo ${REPO} \
    --pattern ${binaryName} \
    --dir ${targetDir} \
    --verify-attestation`.quiet().nothrow();

  if (result.exitCode !== 0) {
    const stderr = result.stderr.toString();
    if (stderr.includes("attestation")) {
      logger.error("Binary verification failed - attestation invalid");
      return null;
    }
    // Handle other errors...
  }

  return `${targetDir}/${binaryName}`;
}
```

### Fallback Behavior

- If `gh` doesn't support `--verify-attestation` (older version) → warn but proceed
- If attestation missing from release → warn user, require `--skip-verify` flag to proceed

### Changes Required

- `.github/workflows/release.yml` - Add attestation step
- `src/core/update.ts` - Add verification to download
- `src/index.ts` - Add `--skip-verify` flag for update command

---

## 4. Full Lifecycle Hooks

### New Hooks

Adding to existing `preInstall/postInstall` and `preLink/postLink`:

```typescript
export interface DotfilesConfig {
  hooks?: {
    // Existing
    preInstall?: (ctx: HookContext) => Promise<void>;
    postInstall?: (ctx: HookContext) => Promise<void>;
    preLink?: (ctx: HookContext) => Promise<void>;
    postLink?: (ctx: HookContext) => Promise<void>;

    // New: Sync (git pull + link refresh)
    preSync?: (ctx: HookContext) => Promise<void>;
    postSync?: (ctx: SyncHookContext) => Promise<void>;

    // New: Push (commit + push)
    prePush?: (ctx: HookContext) => Promise<void>;
    postPush?: (ctx: PushHookContext) => Promise<void>;

    // New: Self-update
    preUpdate?: (ctx: HookContext) => Promise<void>;
    postUpdate?: (ctx: UpdateHookContext) => Promise<void>;

    // New: Rollback
    preRollback?: (ctx: HookContext) => Promise<void>;
    postRollback?: (ctx: HookContext) => Promise<void>;
  };
}
```

### Extended Context for Post-Hooks

```typescript
interface SyncHookContext extends HookContext {
  filesChanged: string[];
  linksRefreshed: boolean;
}

interface PushHookContext extends HookContext {
  commitHash: string;
  filesCommitted: string[];
}

interface UpdateHookContext extends HookContext {
  previousVersion: string;
  newVersion: string;
}
```

### Example Usage

```typescript
export default defineConfig({
  hooks: {
    postSync: async (ctx) => {
      if (ctx.filesChanged.some(f => f.includes("zshrc"))) {
        console.log("Shell config updated - restart your terminal");
      }
    },
    postPush: async (ctx) => {
      // Notify or trigger CI
      await ctx.shell(`curl -X POST https://my-webhook.com/dotfiles-updated`);
    },
  },
});
```

### Changes Required

- `src/types/index.ts` - Add new hook types and contexts
- `src/core/sync.ts` - Add preSync/postSync hook calls
- `src/core/push.ts` - Add prePush/postPush hook calls
- `src/core/update.ts` - Add preUpdate/postUpdate hook calls
- `src/core/backup.ts` - Add preRollback/postRollback hook calls

---

## 5. Machine-Specific Configs

### Syntax

Symlinks can optionally specify conditions:

```typescript
export default defineConfig({
  symlinks: {
    // Simple: always link
    "shell/zshrc": ".zshrc",
    "git/gitconfig": ".gitconfig",

    // Conditional: object form with `when`
    "shell/zshrc.work": {
      target: ".zshrc.local",
      when: { hostname: "work-*" }
    },

    // Platform-specific
    "macos/karabiner": {
      target: ".config/karabiner",
      when: { platform: "darwin" }
    },

    // Combined conditions (AND logic)
    "linux/i3config": {
      target: ".config/i3/config",
      when: { platform: "linux", hostname: "desktop-*" }
    },
  },
});
```

### Type Definitions

```typescript
interface SymlinkCondition {
  hostname?: string;   // Glob pattern matched against os.hostname()
  platform?: Platform; // "darwin" | "linux"
}

type SymlinkTarget = string | {
  target: string;
  when: SymlinkCondition;
};

export interface DotfilesConfig {
  symlinks: Record<string, SymlinkTarget>;
  // ... rest unchanged
}
```

### Matching Logic

```typescript
import { hostname } from "os";

function matchGlob(value: string, pattern: string): boolean {
  const regex = new RegExp(
    "^" + pattern.replace(/\*/g, ".*").replace(/\?/g, ".") + "$"
  );
  return regex.test(value);
}

function shouldLink(condition: SymlinkCondition): boolean {
  if (condition.platform && getPlatform() !== condition.platform) {
    return false;
  }
  if (condition.hostname && !matchGlob(hostname(), condition.hostname)) {
    return false;
  }
  return true;
}
```

### Status Output

`paw status` shows why symlinks were skipped:

```
─── Symlinks ───
  ✓ ~/.zshrc (linked)
  ✓ ~/.gitconfig (linked)
  ○ ~/.zshrc.local (skipped: hostname work-* ≠ macbook-pro)
  ○ ~/.config/i3/config (skipped: platform linux ≠ darwin)
```

### Changes Required

- `src/types/index.ts` - Add `SymlinkCondition` and `SymlinkTarget` types
- `src/core/symlinks.ts` - Add condition checking, update all symlink functions
- `src/core/os.ts` - Add `matchGlob()` helper
- `src/index.ts` - Update status display

---

## Implementation Order

Recommended order based on dependencies:

1. **Machine-Specific Configs** - Type changes used by other features
2. **Full Lifecycle Hooks** - Type changes, independent of other features
3. **Interactive Conflict Resolution** - Improves UX for manual testing
4. **Binary Verification** - CI/release change, independent
5. **E2E Testing** - Tests all the above features

---

## Summary

| Feature | New Files | Modified Files |
|---------|-----------|----------------|
| E2E Testing | `tests/e2e/*`, `Dockerfile` | `package.json` |
| Interactive Conflicts | `src/core/prompt.ts` | `symlinks.ts`, `index.ts` |
| Binary Verification | - | `release.yml`, `update.ts`, `index.ts` |
| Lifecycle Hooks | - | `types/index.ts`, `sync.ts`, `push.ts`, `update.ts`, `backup.ts` |
| Machine Configs | - | `types/index.ts`, `symlinks.ts`, `os.ts`, `index.ts` |
