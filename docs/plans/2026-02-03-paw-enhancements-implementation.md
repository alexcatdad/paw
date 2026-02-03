# Paw Enhancements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add machine-specific configs, full lifecycle hooks, interactive conflict resolution, binary verification, and E2E testing to paw.

**Architecture:** Five independent features built in dependency order. Types first (machine configs, hooks), then UX (interactive prompts), then security (verification), then testing (E2E). Each feature is self-contained with its own tests.

**Tech Stack:** TypeScript, Bun, Docker (for E2E tests), GitHub Actions attestations

---

## Task 1: Machine-Specific Configs - Types

**Files:**
- Modify: `src/types/index.ts`

**Step 1: Add SymlinkCondition and SymlinkTarget types**

In `src/types/index.ts`, add after the `Platform` type (around line 5):

```typescript
export interface SymlinkCondition {
  /** Glob pattern matched against os.hostname() */
  hostname?: string;
  /** Target platform */
  platform?: Platform;
}

export type SymlinkTarget = string | {
  target: string;
  when: SymlinkCondition;
};
```

**Step 2: Update DotfilesConfig symlinks type**

Change line 41 from:
```typescript
symlinks: Record<string, string>;
```
to:
```typescript
symlinks: Record<string, SymlinkTarget>;
```

**Step 3: Run typecheck**

Run: `bun run typecheck`
Expected: Type errors in symlinks.ts (expected - we haven't updated it yet)

**Step 4: Commit types**

```bash
git add src/types/index.ts
git commit -m "feat(types): add SymlinkCondition and SymlinkTarget for machine-specific configs"
```

---

## Task 2: Machine-Specific Configs - Glob Matcher

**Files:**
- Modify: `src/core/os.ts`

**Step 1: Add matchGlob helper function**

At the end of `src/core/os.ts`, add:

```typescript
/**
 * Match a value against a glob pattern (supports * and ? wildcards)
 */
export function matchGlob(value: string, pattern: string): boolean {
  const regex = new RegExp(
    "^" + pattern.replace(/[.+^${}()|[\]\\]/g, "\\$&").replace(/\*/g, ".*").replace(/\?/g, ".") + "$"
  );
  return regex.test(value);
}

/**
 * Get the system hostname
 */
export function getHostname(): string {
  return require("os").hostname();
}
```

**Step 2: Run typecheck**

Run: `bun run typecheck`
Expected: PASS (no new errors from this change)

**Step 3: Commit**

```bash
git add src/core/os.ts
git commit -m "feat(os): add matchGlob helper and getHostname for machine-specific configs"
```

---

## Task 3: Machine-Specific Configs - Symlink Logic

**Files:**
- Modify: `src/core/symlinks.ts`

**Step 1: Add imports and helper function**

At the top of `src/core/symlinks.ts`, update the imports:

```typescript
import type { SymlinkState, InstallOptions, SymlinkEntry, SymlinkTarget, SymlinkCondition } from "../types";
```

And add import for os helpers:
```typescript
import { getHomeDir, contractPath, validatePathWithinBase, getPlatform, matchGlob, getHostname } from "./os";
```

Add this helper function after the imports:

```typescript
/**
 * Check if a symlink should be created based on conditions
 */
function shouldCreateSymlink(condition: SymlinkCondition | undefined): { create: boolean; reason?: string } {
  if (!condition) {
    return { create: true };
  }

  if (condition.platform && getPlatform() !== condition.platform) {
    return { create: false, reason: `platform ${condition.platform} ≠ ${getPlatform()}` };
  }

  if (condition.hostname) {
    const currentHostname = getHostname();
    if (!matchGlob(currentHostname, condition.hostname)) {
      return { create: false, reason: `hostname ${condition.hostname} ≠ ${currentHostname}` };
    }
  }

  return { create: true };
}

/**
 * Normalize symlink target to extract target path and condition
 */
function normalizeSymlinkTarget(target: SymlinkTarget): { targetPath: string; condition?: SymlinkCondition } {
  if (typeof target === "string") {
    return { targetPath: target };
  }
  return { targetPath: target.target, condition: target.when };
}
```

**Step 2: Update createSymlinks function**

Replace the `createSymlinks` function:

```typescript
/**
 * Create symlinks for all configured files
 */
export async function createSymlinks(
  symlinks: Record<string, SymlinkTarget>,
  options: InstallOptions
): Promise<SymlinkState[]> {
  const homeDir = getHomeDir();
  const states: SymlinkState[] = [];

  for (const [sourceRel, targetConfig] of Object.entries(symlinks)) {
    const { targetPath, condition } = normalizeSymlinkTarget(targetConfig);
    const source = resolveConfigPath(sourceRel);
    const target = resolve(homeDir, targetPath);

    // Check if symlink should be created based on conditions
    const { create, reason } = shouldCreateSymlink(condition);
    if (!create) {
      logger.skip(`${contractPath(target)} (skipped: ${reason})`);
      states.push({
        source,
        target,
        status: "missing", // Use missing for skipped conditional symlinks
      });
      continue;
    }

    // Security: Prevent path traversal attacks
    validatePathWithinBase(target, homeDir, "Symlink target");

    const state = await createSymlink(source, target, options);
    states.push(state);
  }

  return states;
}
```

**Step 3: Update removeSymlinks function signature**

```typescript
export async function removeSymlinks(
  symlinks: Record<string, SymlinkTarget>,
  options: InstallOptions
): Promise<void> {
  const homeDir = getHomeDir();

  for (const [_, targetConfig] of Object.entries(symlinks)) {
    const { targetPath } = normalizeSymlinkTarget(targetConfig);
    const target = resolve(homeDir, targetPath);
    // ... rest unchanged
```

**Step 4: Update getSymlinkStatus function**

```typescript
export async function getSymlinkStatus(
  symlinks: Record<string, SymlinkTarget>
): Promise<SymlinkState[]> {
  const homeDir = getHomeDir();
  const states: SymlinkState[] = [];

  for (const [sourceRel, targetConfig] of Object.entries(symlinks)) {
    const { targetPath, condition } = normalizeSymlinkTarget(targetConfig);
    const source = resolveConfigPath(sourceRel);
    const target = resolve(homeDir, targetPath);

    // Check conditions for status display
    const { create, reason } = shouldCreateSymlink(condition);

    // Security: Prevent path traversal attacks
    validatePathWithinBase(target, homeDir, "Symlink target");

    const state: SymlinkState = {
      source,
      target,
      status: "missing",
    };

    // If condition not met, show as skipped in status
    if (!create) {
      // We'll use a custom display in index.ts for this
      states.push(state);
      continue;
    }

    // Check if source exists
    try {
      await stat(source);
    } catch {
      state.status = "source-missing";
      states.push(state);
      continue;
    }

    // Check target status
    if (await isSymlinkTo(target, source)) {
      state.status = "linked";
    } else if (await fileOrLinkExists(target)) {
      state.status = "conflict";
    } else {
      state.status = "missing";
    }

    states.push(state);
  }

  return states;
}
```

**Step 5: Run typecheck**

Run: `bun run typecheck`
Expected: PASS

**Step 6: Manual test**

Run: `bun run dev status`
Expected: Should work with existing string-only symlinks

**Step 7: Commit**

```bash
git add src/core/symlinks.ts
git commit -m "feat(symlinks): support conditional symlinks with hostname/platform matching"
```

---

## Task 4: Full Lifecycle Hooks - Types

**Files:**
- Modify: `src/types/index.ts`

**Step 1: Add extended hook context types**

After the `HookContext` interface (around line 37), add:

```typescript
export interface SyncHookContext extends HookContext {
  /** Files changed during sync */
  filesChanged: string[];
  /** Whether symlinks were refreshed */
  linksRefreshed: boolean;
}

export interface PushHookContext extends HookContext {
  /** Commit hash after push */
  commitHash: string;
  /** Files that were committed */
  filesCommitted: string[];
}

export interface UpdateHookContext extends HookContext {
  /** Version before update */
  previousVersion: string;
  /** Version after update */
  newVersion: string;
}
```

**Step 2: Update hooks in DotfilesConfig**

Replace the `hooks` property in `DotfilesConfig`:

```typescript
  /** Lifecycle hooks */
  hooks?: {
    // Install lifecycle
    preInstall?: (ctx: HookContext) => Promise<void>;
    postInstall?: (ctx: HookContext) => Promise<void>;
    // Link lifecycle
    preLink?: (ctx: HookContext) => Promise<void>;
    postLink?: (ctx: HookContext) => Promise<void>;
    // Sync lifecycle
    preSync?: (ctx: HookContext) => Promise<void>;
    postSync?: (ctx: SyncHookContext) => Promise<void>;
    // Push lifecycle
    prePush?: (ctx: HookContext) => Promise<void>;
    postPush?: (ctx: PushHookContext) => Promise<void>;
    // Update lifecycle
    preUpdate?: (ctx: HookContext) => Promise<void>;
    postUpdate?: (ctx: UpdateHookContext) => Promise<void>;
    // Rollback lifecycle
    preRollback?: (ctx: HookContext) => Promise<void>;
    postRollback?: (ctx: HookContext) => Promise<void>;
  };
```

**Step 3: Run typecheck**

Run: `bun run typecheck`
Expected: PASS

**Step 4: Commit**

```bash
git add src/types/index.ts
git commit -m "feat(types): add full lifecycle hook types (sync, push, update, rollback)"
```

---

## Task 5: Full Lifecycle Hooks - Hook Runner Helper

**Files:**
- Create: `src/core/hooks.ts`

**Step 1: Create hooks helper module**

```typescript
/**
 * Hook Runner
 * Centralized hook execution with consistent context creation
 */

import { $ } from "bun";
import { getPlatform, getHomeDir, getRepoDir, commandExists } from "./os";
import { logger } from "./logger";
import type { HookContext, DotfilesConfig } from "../types";

/**
 * Create base hook context
 */
export function createHookContext(dryRun: boolean): HookContext {
  return {
    platform: getPlatform(),
    homeDir: getHomeDir(),
    repoDir: getRepoDir(),
    dryRun,
    shell: async (cmd: string) => {
      const { $ } = await import("bun");
      await $`sh -c ${cmd}`;
    },
    commandExists,
  };
}

/**
 * Run a hook if it exists
 */
export async function runHook<T extends HookContext>(
  config: DotfilesConfig,
  hookName: keyof NonNullable<DotfilesConfig["hooks"]>,
  context: T,
  label?: string
): Promise<void> {
  const hook = config.hooks?.[hookName];
  if (!hook) return;

  const displayName = label ?? hookName;
  logger.subheader(`Running ${displayName} hook`);

  try {
    await (hook as (ctx: T) => Promise<void>)(context);
  } catch (error) {
    logger.error(`Hook ${displayName} failed: ${error}`);
    throw error;
  }
}
```

**Step 2: Run typecheck**

Run: `bun run typecheck`
Expected: PASS

**Step 3: Commit**

```bash
git add src/core/hooks.ts
git commit -m "feat(hooks): add centralized hook runner helper"
```

---

## Task 6: Full Lifecycle Hooks - Sync Hooks

**Files:**
- Modify: `src/core/sync.ts`

**Step 1: Add imports**

Add to imports at top:

```typescript
import { createHookContext, runHook } from "./hooks";
import type { SyncHookContext } from "../types";
```

**Step 2: Update runSync to call hooks**

In the `runSync` function, after loading the config and before the sync logic, add preSync hook:

After `const result: SyncResult = { ... }`, add:

```typescript
  // Load config for hooks
  let config;
  try {
    config = await loadConfig();
  } catch {
    // Config might not exist yet, that's ok
  }

  // Run preSync hook
  if (config?.hooks?.preSync) {
    await runHook(config, "preSync", createHookContext(options.dryRun));
  }
```

At the end of `runSync`, before `return result;`, add:

```typescript
  // Run postSync hook
  if (config?.hooks?.postSync) {
    const syncContext: SyncHookContext = {
      ...createHookContext(options.dryRun),
      filesChanged: changedFiles,
      linksRefreshed: result.linksRefreshed,
    };
    await runHook(config, "postSync", syncContext);
  }
```

**Step 3: Run typecheck**

Run: `bun run typecheck`
Expected: PASS

**Step 4: Commit**

```bash
git add src/core/sync.ts
git commit -m "feat(sync): add preSync/postSync lifecycle hooks"
```

---

## Task 7: Full Lifecycle Hooks - Push Hooks

**Files:**
- Modify: `src/core/push.ts`

**Step 1: Add imports**

```typescript
import { loadConfig } from "./config";
import { createHookContext, runHook } from "./hooks";
import type { PushHookContext } from "../types";
import { $ } from "bun";
```

**Step 2: Update runPush to include hooks**

Replace the entire `runPush` function:

```typescript
/**
 * Push dotfiles changes to remote
 */
export async function runPush(message: string | undefined, options: InstallOptions): Promise<boolean> {
  const repoPath = await getDotfilesPath();

  // Load config for hooks
  let config;
  try {
    config = await loadConfig();
  } catch {
    // Config might not exist, that's ok
  }

  // Check for changes
  const statusResult = await $`git -C ${repoPath} status --porcelain`.quiet().nothrow();
  if (statusResult.exitCode !== 0) {
    logger.error("Not a git repository or git error");
    logger.error(statusResult.stderr.toString());
    return false;
  }

  const changes = statusResult.text().trim();
  if (!changes) {
    logger.info("No changes to push");
    return true;
  }

  // Show what will be committed
  const changedFiles = changes.split("\n");
  logger.info(`${changedFiles.length} file(s) changed`);

  if (options.verbose) {
    logger.newline();
    console.log(changes);
    logger.newline();
  }

  if (options.dryRun) {
    logger.info("Would stage, commit, and push changes");
    return true;
  }

  // Run prePush hook
  if (config?.hooks?.prePush) {
    await runHook(config, "prePush", createHookContext(options.dryRun));
  }

  // Generate commit message if not provided
  const commitMessage = message ?? `Update dotfiles (${new Date().toISOString().split("T")[0]})`;

  // Stage all changes
  logger.info("Staging changes...");
  const addResult = await $`git -C ${repoPath} add -A`.quiet().nothrow();
  if (addResult.exitCode !== 0) {
    logger.error("Failed to stage changes");
    logger.error(addResult.stderr.toString());
    return false;
  }

  // Commit
  logger.info("Committing...");
  const commitResult = await $`git -C ${repoPath} commit -m ${commitMessage}`.quiet().nothrow();
  if (commitResult.exitCode !== 0) {
    const stderr = commitResult.stderr.toString();
    if (stderr.includes("nothing to commit")) {
      logger.info("Nothing to commit");
      return true;
    }
    logger.error(`Failed to commit: ${stderr}`);
    return false;
  }

  // Get commit hash
  const hashResult = await $`git -C ${repoPath} rev-parse HEAD`.quiet().nothrow();
  const commitHash = hashResult.exitCode === 0 ? hashResult.text().trim() : "unknown";

  // Push
  logger.info("Pushing to remote...");
  const pushResult = await $`git -C ${repoPath} push`.quiet().nothrow();
  if (pushResult.exitCode !== 0) {
    logger.error("Failed to push. You may need to pull first: paw sync");
    logger.error(pushResult.stderr.toString());
    return false;
  }

  logger.success(`Pushed: ${commitMessage}`);

  // Run postPush hook
  if (config?.hooks?.postPush) {
    const pushContext: PushHookContext = {
      ...createHookContext(options.dryRun),
      commitHash,
      filesCommitted: changedFiles.map(line => line.slice(3)), // Remove status prefix
    };
    await runHook(config, "postPush", pushContext);
  }

  return true;
}
```

**Step 3: Run typecheck**

Run: `bun run typecheck`
Expected: PASS

**Step 4: Commit**

```bash
git add src/core/push.ts
git commit -m "feat(push): add prePush/postPush lifecycle hooks"
```

---

## Task 8: Full Lifecycle Hooks - Update Hooks

**Files:**
- Modify: `src/core/update.ts`

**Step 1: Add imports**

```typescript
import { loadConfig } from "./config";
import { createHookContext, runHook } from "./hooks";
import type { UpdateHookContext } from "../types";
```

**Step 2: Update performUpdate to include hooks**

In `performUpdate`, after `logger.info(\`Update available: v${currentVersion} → v${latestVersion}\`);` and before the download logic, add:

```typescript
  // Load config for hooks
  let config;
  try {
    config = await loadConfig();
  } catch {
    // Config might not exist, that's ok
  }

  // Run preUpdate hook
  if (config?.hooks?.preUpdate) {
    await runHook(config, "preUpdate", createHookContext(options.dryRun));
  }
```

At the end of `performUpdate`, before `logger.success(\`Updated to v${latestVersion}!\`);`, wrap the success path with postUpdate hook:

```typescript
    // Run postUpdate hook
    if (config?.hooks?.postUpdate) {
      const updateContext: UpdateHookContext = {
        ...createHookContext(options.dryRun),
        previousVersion: currentVersion,
        newVersion: latestVersion,
      };
      await runHook(config, "postUpdate", updateContext);
    }

    logger.success(`Updated to v${latestVersion}!`);
```

**Step 3: Run typecheck**

Run: `bun run typecheck`
Expected: PASS

**Step 4: Commit**

```bash
git add src/core/update.ts
git commit -m "feat(update): add preUpdate/postUpdate lifecycle hooks"
```

---

## Task 9: Full Lifecycle Hooks - Rollback Hooks

**Files:**
- Modify: `src/core/backup.ts`

**Step 1: Add imports**

```typescript
import { loadConfig } from "./config";
import { createHookContext, runHook } from "./hooks";
```

**Step 2: Update rollback to include hooks**

In the `rollback` function, after loading lastRun state and checking if it exists, add:

```typescript
  // Load config for hooks
  let config;
  try {
    config = await loadConfig();
  } catch {
    // Config might not exist, that's ok
  }

  // Run preRollback hook
  if (config?.hooks?.preRollback) {
    await runHook(config, "preRollback", createHookContext(options.dryRun));
  }
```

At the end of `rollback`, before `logger.success("Rollback complete!");`, add:

```typescript
  // Run postRollback hook
  if (config?.hooks?.postRollback) {
    await runHook(config, "postRollback", createHookContext(options.dryRun));
  }
```

**Step 3: Run typecheck**

Run: `bun run typecheck`
Expected: PASS

**Step 4: Commit**

```bash
git add src/core/backup.ts
git commit -m "feat(backup): add preRollback/postRollback lifecycle hooks"
```

---

## Task 10: Interactive Conflict Resolution - Prompt Module

**Files:**
- Create: `src/core/prompt.ts`

**Step 1: Create the prompt module**

```typescript
/**
 * Interactive Prompts
 * Handle user interaction for conflict resolution
 */

import { createInterface } from "readline";
import { $ } from "bun";
import { logger } from "./logger";
import { contractPath } from "./os";

export interface ConflictChoice {
  action: "skip" | "backup" | "overwrite" | "abort";
  applyToAll?: boolean;
}

/**
 * Check if stdin is interactive (TTY)
 */
export function isInteractive(): boolean {
  return process.stdin.isTTY === true;
}

/**
 * Read a single character from stdin
 */
async function readChar(): Promise<string> {
  return new Promise((resolve) => {
    const rl = createInterface({
      input: process.stdin,
      output: process.stdout,
    });

    process.stdin.setRawMode?.(true);
    process.stdin.resume();
    process.stdin.once("data", (data) => {
      process.stdin.setRawMode?.(false);
      rl.close();
      resolve(data.toString().toLowerCase());
    });
  });
}

/**
 * Show diff between two files
 */
export async function showDiff(existingPath: string, sourcePath: string): Promise<void> {
  const result = await $`diff -u ${existingPath} ${sourcePath}`.quiet().nothrow();

  if (result.exitCode === 0) {
    logger.info("Files are identical");
  } else {
    console.log("\n" + result.text());
  }
}

/**
 * Prompt user for conflict resolution
 */
export async function promptConflict(
  targetPath: string,
  sourcePath: string
): Promise<ConflictChoice> {
  console.log(`
\x1b[33m⚠ ${contractPath(targetPath)} already exists\x1b[0m

  [s] Skip this file
  [b] Backup & link
  [o] Overwrite (no backup)
  [d] Show diff
  [a] Abort
  ─────────────────────
  [S] Skip all remaining
  [B] Backup all remaining
`);

  process.stdout.write("Choice [s/b/o/d/a/S/B]: ");

  while (true) {
    const char = await readChar();
    console.log(char); // Echo the character

    switch (char) {
      case "s":
        return { action: "skip" };
      case "b":
        return { action: "backup" };
      case "o":
        return { action: "overwrite" };
      case "d":
        await showDiff(targetPath, sourcePath);
        process.stdout.write("\nChoice [s/b/o/d/a/S/B]: ");
        continue;
      case "a":
        return { action: "abort" };
      case "S": // Capital S = skip all
        return { action: "skip", applyToAll: true };
      case "B": // Capital B = backup all
        return { action: "backup", applyToAll: true };
      default:
        process.stdout.write("Invalid choice. [s/b/o/d/a/S/B]: ");
    }
  }
}
```

**Step 2: Run typecheck**

Run: `bun run typecheck`
Expected: PASS

**Step 3: Commit**

```bash
git add src/core/prompt.ts
git commit -m "feat(prompt): add interactive conflict resolution prompts"
```

---

## Task 11: Interactive Conflict Resolution - Integrate with Symlinks

**Files:**
- Modify: `src/core/symlinks.ts`
- Modify: `src/types/index.ts`

**Step 1: Add interactive option to InstallOptions**

In `src/types/index.ts`, update `InstallOptions`:

```typescript
export interface InstallOptions {
  /** Show what would be done without making changes */
  dryRun: boolean;
  /** Overwrite existing files (creates backups) */
  force: boolean;
  /** Show detailed output */
  verbose: boolean;
  /** Skip package installation */
  skipPackages: boolean;
  /** Disable interactive prompts */
  noInteractive?: boolean;
}
```

**Step 2: Update symlinks.ts to use prompts**

Add import at top of `src/core/symlinks.ts`:

```typescript
import { isInteractive, promptConflict, type ConflictChoice } from "./prompt";
```

Replace the `createSymlink` function:

```typescript
/**
 * Create a single symlink with backup handling
 */
async function createSymlink(
  source: string,
  target: string,
  options: InstallOptions,
  pendingChoice?: ConflictChoice
): Promise<{ state: SymlinkState; nextChoice?: ConflictChoice }> {
  const state: SymlinkState = {
    source,
    target,
    status: "missing",
  };

  // Check if source file exists
  try {
    await stat(source);
  } catch {
    logger.error(`Source file not found: ${contractPath(source)}`);
    state.status = "source-missing";
    return { state };
  }

  // Check if target already exists
  const targetExists = await fileOrLinkExists(target);

  if (targetExists) {
    // Check if it's already the correct symlink
    const isCorrectLink = await isSymlinkTo(target, source);

    if (isCorrectLink) {
      logger.skip(`${contractPath(target)} (already linked)`);
      state.status = "linked";
      return { state };
    }

    // Handle conflict
    let choice: ConflictChoice | undefined = pendingChoice;

    if (!choice) {
      if (options.force) {
        choice = { action: "backup" };
      } else if (options.noInteractive || !isInteractive()) {
        logger.warn(`Conflict: ${contractPath(target)} exists (use --force to backup and replace)`);
        state.status = "conflict";
        return { state };
      } else {
        // Interactive prompt
        choice = await promptConflict(target, source);
      }
    }

    // Handle the choice
    switch (choice.action) {
      case "abort":
        throw new Error("Aborted by user");

      case "skip":
        logger.skip(`${contractPath(target)} (skipped by user)`);
        state.status = "conflict";
        return { state, nextChoice: choice.applyToAll ? choice : undefined };

      case "overwrite":
        if (options.dryRun) {
          logger.dryRun(`Would overwrite ${contractPath(target)} (no backup)`);
        } else {
          await unlink(target);
          logger.info(`Removed ${contractPath(target)} (no backup)`);
        }
        break;

      case "backup":
        if (options.dryRun) {
          logger.dryRun(`Would backup ${contractPath(target)}`);
        } else {
          const backupPath = await createBackup(target);
          logger.info(`Backed up ${contractPath(target)} to ${contractPath(backupPath)}`);
          state.backupPath = backupPath;
        }
        state.status = "backup";
        break;
    }

    // Pass along applyToAll choice
    if (choice.applyToAll) {
      return { state: await finishSymlink(source, target, state, options), nextChoice: choice };
    }
  }

  return { state: await finishSymlink(source, target, state, options) };
}

/**
 * Complete symlink creation after conflict handling
 */
async function finishSymlink(
  source: string,
  target: string,
  state: SymlinkState,
  options: InstallOptions
): Promise<SymlinkState> {
  // Create parent directories
  const targetDir = dirname(target);
  if (options.dryRun) {
    logger.dryRun(`Would create directory: ${contractPath(targetDir)}`);
    logger.dryRun(`Would symlink: ${contractPath(target)} → ${contractPath(source)}`);
  } else {
    await mkdir(targetDir, { recursive: true });
    await symlink(source, target);
    logger.link(contractPath(source), contractPath(target));
  }

  state.status = "linked";
  return state;
}
```

**Step 3: Update createSymlinks to handle pending choices**

```typescript
export async function createSymlinks(
  symlinks: Record<string, SymlinkTarget>,
  options: InstallOptions
): Promise<SymlinkState[]> {
  const homeDir = getHomeDir();
  const states: SymlinkState[] = [];
  let pendingChoice: ConflictChoice | undefined;

  for (const [sourceRel, targetConfig] of Object.entries(symlinks)) {
    const { targetPath, condition } = normalizeSymlinkTarget(targetConfig);
    const source = resolveConfigPath(sourceRel);
    const target = resolve(homeDir, targetPath);

    // Check if symlink should be created based on conditions
    const { create, reason } = shouldCreateSymlink(condition);
    if (!create) {
      logger.skip(`${contractPath(target)} (skipped: ${reason})`);
      states.push({
        source,
        target,
        status: "missing",
      });
      continue;
    }

    // Security: Prevent path traversal attacks
    validatePathWithinBase(target, homeDir, "Symlink target");

    const { state, nextChoice } = await createSymlink(source, target, options, pendingChoice);
    states.push(state);

    if (nextChoice) {
      pendingChoice = nextChoice;
    }
  }

  return states;
}
```

**Step 4: Add unlink import**

Make sure `unlink` is imported at the top:

```typescript
import { mkdir, symlink, unlink, readlink, rename, lstat, stat } from "node:fs/promises";
```

**Step 5: Run typecheck**

Run: `bun run typecheck`
Expected: PASS

**Step 6: Commit**

```bash
git add src/core/symlinks.ts src/types/index.ts
git commit -m "feat(symlinks): integrate interactive conflict resolution"
```

---

## Task 12: Interactive Conflict Resolution - CLI Flag

**Files:**
- Modify: `src/index.ts`

**Step 1: Add --no-interactive flag**

In the `parseArgs` options, add:

```typescript
"no-interactive": { type: "boolean", default: false },
```

**Step 2: Pass to InstallOptions**

Update the options object:

```typescript
const options: InstallOptions = {
  dryRun: values["dry-run"] as boolean,
  force: values.force as boolean,
  verbose: values.verbose as boolean,
  skipPackages: values["skip-packages"] as boolean,
  noInteractive: values["no-interactive"] as boolean,
};
```

**Step 3: Update help text**

Add to OPTIONS section:

```
  --no-interactive   Disable interactive prompts (skip conflicts)
```

**Step 4: Run typecheck**

Run: `bun run typecheck`
Expected: PASS

**Step 5: Commit**

```bash
git add src/index.ts
git commit -m "feat(cli): add --no-interactive flag for non-TTY environments"
```

---

## Task 13: Binary Verification - Update Workflow

**Files:**
- Modify: `.github/workflows/release.yml`

**Step 1: Add attestation permissions and step**

```yaml
name: Release

on:
  release:
    types: [published]

jobs:
  upload-binaries:
    name: Upload Release Binaries
    runs-on: ubuntu-latest
    # Only run for version tags, not the 'latest' pre-release
    if: startsWith(github.event.release.tag_name, 'v') && github.event.release.tag_name != 'latest'
    permissions:
      contents: write
      id-token: write
      attestations: write
    steps:
      - uses: actions/checkout@v4
        with:
          ref: ${{ github.event.release.tag_name }}

      - name: Setup Bun
        uses: oven-sh/setup-bun@v1

      - name: Install dependencies
        run: bun install

      - name: Type check
        run: bun run typecheck

      - name: Build all targets
        run: |
          mkdir -p dist
          bun build src/index.ts --compile --target=bun-darwin-arm64 --outfile=dist/paw-darwin-arm64
          bun build src/index.ts --compile --target=bun-darwin-x64 --outfile=dist/paw-darwin-x64
          bun build src/index.ts --compile --target=bun-linux-x64 --outfile=dist/paw-linux-x64
          bun build src/index.ts --compile --target=bun-linux-arm64 --outfile=dist/paw-linux-arm64

      - name: Attest build provenance
        uses: actions/attest-build-provenance@v2
        with:
          subject-path: 'dist/paw-*'

      - name: Upload binaries to release
        uses: softprops/action-gh-release@v1
        with:
          tag_name: ${{ github.event.release.tag_name }}
          files: |
            dist/paw-darwin-arm64
            dist/paw-darwin-x64
            dist/paw-linux-x64
            dist/paw-linux-arm64
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

**Step 2: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "feat(ci): add GitHub attestation for binary verification"
```

---

## Task 14: Binary Verification - Update Module

**Files:**
- Modify: `src/core/update.ts`
- Modify: `src/index.ts`

**Step 1: Update downloadBinary to verify attestation**

Replace `downloadBinary` function in `src/core/update.ts`:

```typescript
/**
 * Download the binary for the current platform with optional verification
 */
export async function downloadBinary(
  targetDir: string,
  options: InstallOptions & { skipVerify?: boolean }
): Promise<string | null> {
  const binaryName = getBinaryName();

  if (options.dryRun) {
    logger.info(`Would download: ${binaryName} to ${targetDir}`);
    return `${targetDir}/${binaryName}`;
  }

  try {
    // Check if gh supports --verify-attestation
    const helpResult = await $`gh release download --help`.quiet().nothrow();
    const supportsVerify = helpResult.text().includes("verify-attestation");

    if (supportsVerify && !options.skipVerify) {
      logger.info("Downloading with attestation verification...");
      const result = await $`gh release download latest --repo ${REPO} --pattern ${binaryName} --dir ${targetDir} --verify-attestation`.quiet().nothrow();

      if (result.exitCode !== 0) {
        const stderr = result.stderr.toString();

        if (stderr.includes("attestation")) {
          logger.error("Binary verification failed - attestation invalid or missing");
          logger.info("Use --skip-verify to bypass verification (not recommended)");
          return null;
        }

        logger.error(`Failed to download binary: ${stderr}`);
        return null;
      }

      logger.success("Binary attestation verified");
    } else {
      if (!options.skipVerify && !supportsVerify) {
        logger.warn("gh CLI doesn't support attestation verification. Consider updating gh.");
      }

      const result = await $`gh release download latest --repo ${REPO} --pattern ${binaryName} --dir ${targetDir}`.quiet().nothrow();

      if (result.exitCode !== 0) {
        logger.error(`Failed to download binary: ${result.stderr.toString()}`);
        return null;
      }
    }

    return `${targetDir}/${binaryName}`;
  } catch (error) {
    logger.error(`Download failed: ${error}`);
    return null;
  }
}
```

**Step 2: Update performUpdate signature**

Update `performUpdate` to accept skipVerify:

```typescript
export async function performUpdate(options: InstallOptions & { skipVerify?: boolean }): Promise<boolean> {
```

And update the call to `downloadBinary`:

```typescript
const downloadedPath = await downloadBinary(tmpDir, options);
```

**Step 3: Add --skip-verify flag to CLI**

In `src/index.ts`, add to parseArgs options:

```typescript
"skip-verify": { type: "boolean", default: false },
```

Update the update command case:

```typescript
case "update": {
  const updateOptions = {
    ...options,
    skipVerify: values["skip-verify"] as boolean,
  };
  await updateCommand(updateOptions);
  break;
}
```

Update help text:

```
  --skip-verify        Skip binary attestation verification (update command)
```

**Step 4: Run typecheck**

Run: `bun run typecheck`
Expected: PASS

**Step 5: Commit**

```bash
git add src/core/update.ts src/index.ts
git commit -m "feat(update): add binary attestation verification with --skip-verify escape hatch"
```

---

## Task 15: E2E Testing - Docker Setup

**Files:**
- Create: `tests/e2e/docker/Dockerfile`

**Step 1: Create Dockerfile**

```dockerfile
FROM oven/bun:1 AS base

# Install git and common tools
RUN apt-get update && apt-get install -y \
    git \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create test user with home directory
RUN useradd -m -s /bin/bash testuser
USER testuser
WORKDIR /home/testuser

# Initialize git config (required for commits)
RUN git config --global user.email "test@example.com" && \
    git config --global user.name "Test User" && \
    git config --global init.defaultBranch main

# Create bin directory for paw
RUN mkdir -p /home/testuser/.local/bin
ENV PATH="/home/testuser/.local/bin:${PATH}"
```

**Step 2: Commit**

```bash
mkdir -p tests/e2e/docker
git add tests/e2e/docker/Dockerfile
git commit -m "test(e2e): add Docker setup for isolated testing"
```

---

## Task 16: E2E Testing - Test Helpers

**Files:**
- Create: `tests/e2e/helpers/docker.ts`
- Create: `tests/e2e/helpers/fixtures.ts`
- Create: `tests/e2e/helpers/assertions.ts`

**Step 1: Create docker.ts**

```typescript
/**
 * Docker Container Management for E2E Tests
 */

import { $ } from "bun";
import { randomUUID } from "crypto";

const IMAGE_NAME = "paw-e2e-test";

export interface Container {
  id: string;
  run: (cmd: string) => Promise<{ stdout: string; stderr: string; exitCode: number }>;
  copyTo: (localPath: string, containerPath: string) => Promise<void>;
  cleanup: () => Promise<void>;
}

/**
 * Build the test Docker image (run once before tests)
 */
export async function buildTestImage(): Promise<void> {
  const dockerfilePath = new URL("../docker/Dockerfile", import.meta.url).pathname;
  await $`docker build -t ${IMAGE_NAME} -f ${dockerfilePath} .`.quiet();
}

/**
 * Create a new container for testing
 */
export async function createContainer(): Promise<Container> {
  const id = `paw-test-${randomUUID().slice(0, 8)}`;

  // Start container in detached mode
  await $`docker run -d --name ${id} ${IMAGE_NAME} sleep infinity`.quiet();

  const container: Container = {
    id,

    async run(cmd: string) {
      const result = await $`docker exec ${id} bash -c ${cmd}`.quiet().nothrow();
      return {
        stdout: result.stdout.toString(),
        stderr: result.stderr.toString(),
        exitCode: result.exitCode,
      };
    },

    async copyTo(localPath: string, containerPath: string) {
      await $`docker cp ${localPath} ${id}:${containerPath}`.quiet();
    },

    async cleanup() {
      await $`docker rm -f ${id}`.quiet().nothrow();
    },
  };

  return container;
}

/**
 * Copy the built paw binary into a container
 */
export async function installPawInContainer(container: Container, binaryPath: string): Promise<void> {
  await container.copyTo(binaryPath, "/home/testuser/.local/bin/paw");
  await container.run("chmod +x /home/testuser/.local/bin/paw");
}
```

**Step 2: Create fixtures.ts**

```typescript
/**
 * Test Fixture Generation
 */

import { mkdtemp, writeFile, mkdir } from "fs/promises";
import { tmpdir } from "os";
import { join } from "path";
import type { Container } from "./docker";

export interface TestRepoOptions {
  symlinks?: Record<string, string>;
  packages?: { common?: string[] };
  hooks?: {
    preInstall?: string;
    postInstall?: string;
  };
  /** Files to create in container home dir (for conflict testing) */
  conflicts?: string[];
}

/**
 * Create a test dotfiles repo locally
 */
export async function createTestRepo(options: TestRepoOptions = {}): Promise<string> {
  const repoDir = await mkdtemp(join(tmpdir(), "paw-test-repo-"));

  // Create config directory
  await mkdir(join(repoDir, "config"), { recursive: true });

  // Create symlink source files
  const symlinks = options.symlinks ?? { "shell/zshrc": ".zshrc" };
  for (const [source] of Object.entries(symlinks)) {
    const sourcePath = join(repoDir, "config", source);
    await mkdir(join(sourcePath, ".."), { recursive: true });
    await writeFile(sourcePath, `# Test file: ${source}\n`);
  }

  // Create dotfiles.config.ts
  const configContent = `
import { defineConfig } from "paw";

export default defineConfig({
  symlinks: ${JSON.stringify(symlinks)},
  packages: ${JSON.stringify(options.packages ?? { common: [] })},
  templates: {},
  ignore: [],
  ${options.hooks ? `hooks: {
    ${options.hooks.preInstall ? `preInstall: async (ctx) => { ${options.hooks.preInstall} },` : ""}
    ${options.hooks.postInstall ? `postInstall: async (ctx) => { ${options.hooks.postInstall} },` : ""}
  },` : ""}
});
`;
  await writeFile(join(repoDir, "dotfiles.config.ts"), configContent);

  // Initialize git repo
  const { $ } = await import("bun");
  await $`git -C ${repoDir} init`.quiet();
  await $`git -C ${repoDir} add -A`.quiet();
  await $`git -C ${repoDir} commit -m "Initial commit"`.quiet();

  return repoDir;
}

/**
 * Set up test repo in container
 */
export async function setupTestRepoInContainer(
  container: Container,
  localRepoPath: string,
  options: TestRepoOptions = {}
): Promise<void> {
  // Copy repo to container
  await container.copyTo(localRepoPath, "/home/testuser/dotfiles");
  await container.run("chown -R testuser:testuser /home/testuser/dotfiles");

  // Create conflict files if specified
  for (const conflict of options.conflicts ?? []) {
    await container.run(`echo "existing content" > /home/testuser/${conflict}`);
  }
}
```

**Step 3: Create assertions.ts**

```typescript
/**
 * Custom Test Assertions
 */

import { expect } from "bun:test";
import type { Container } from "./docker";

/**
 * Assert a symlink exists and points to expected target
 */
export async function expectSymlink(
  container: Container,
  linkPath: string,
  expectedTarget: string
): Promise<void> {
  const result = await container.run(`readlink ${linkPath}`);
  expect(result.exitCode).toBe(0);
  expect(result.stdout.trim()).toContain(expectedTarget);
}

/**
 * Assert a file exists
 */
export async function expectFileExists(container: Container, path: string): Promise<void> {
  const result = await container.run(`test -e ${path}`);
  expect(result.exitCode).toBe(0);
}

/**
 * Assert a file does not exist
 */
export async function expectFileNotExists(container: Container, path: string): Promise<void> {
  const result = await container.run(`test -e ${path}`);
  expect(result.exitCode).not.toBe(0);
}

/**
 * Assert command output contains text
 */
export function expectOutputContains(output: string, text: string): void {
  expect(output).toContain(text);
}

/**
 * Assert command succeeded
 */
export function expectSuccess(result: { exitCode: number }): void {
  expect(result.exitCode).toBe(0);
}

/**
 * Assert command failed
 */
export function expectFailure(result: { exitCode: number }): void {
  expect(result.exitCode).not.toBe(0);
}
```

**Step 4: Commit**

```bash
mkdir -p tests/e2e/helpers
git add tests/e2e/helpers/
git commit -m "test(e2e): add test helpers for Docker, fixtures, and assertions"
```

---

## Task 17: E2E Testing - Link Command Tests

**Files:**
- Create: `tests/e2e/link.test.ts`

**Step 1: Create link tests**

```typescript
/**
 * E2E Tests: link command
 */

import { describe, test, expect, beforeAll, afterEach } from "bun:test";
import { buildTestImage, createContainer, installPawInContainer, type Container } from "./helpers/docker";
import { createTestRepo, setupTestRepoInContainer } from "./helpers/fixtures";
import { expectSymlink, expectSuccess } from "./helpers/assertions";
import { $ } from "bun";

// Build binary path
const BINARY_PATH = new URL("../../dist/paw-linux-x64", import.meta.url).pathname;

describe("paw link", () => {
  let container: Container;

  beforeAll(async () => {
    // Build test image (once)
    await buildTestImage();

    // Build paw binary for Linux
    await $`bun build src/index.ts --compile --target=bun-linux-x64 --outfile=dist/paw-linux-x64`.quiet();
  });

  afterEach(async () => {
    if (container) {
      await container.cleanup();
    }
  });

  test("creates symlinks from config", async () => {
    container = await createContainer();
    await installPawInContainer(container, BINARY_PATH);

    // Create test repo
    const repoPath = await createTestRepo({
      symlinks: {
        "shell/zshrc": ".zshrc",
        "git/gitconfig": ".gitconfig",
      },
    });
    await setupTestRepoInContainer(container, repoPath);

    // Run paw link
    const result = await container.run("cd /home/testuser/dotfiles && paw link");
    expectSuccess(result);

    // Verify symlinks
    await expectSymlink(container, "/home/testuser/.zshrc", "dotfiles/config/shell/zshrc");
    await expectSymlink(container, "/home/testuser/.gitconfig", "dotfiles/config/git/gitconfig");
  });

  test("skips existing symlinks", async () => {
    container = await createContainer();
    await installPawInContainer(container, BINARY_PATH);

    const repoPath = await createTestRepo({
      symlinks: { "shell/zshrc": ".zshrc" },
    });
    await setupTestRepoInContainer(container, repoPath);

    // Run paw link twice
    await container.run("cd /home/testuser/dotfiles && paw link");
    const result = await container.run("cd /home/testuser/dotfiles && paw link");

    expectSuccess(result);
    expect(result.stdout).toContain("already linked");
  });

  test("reports conflicts without --force", async () => {
    container = await createContainer();
    await installPawInContainer(container, BINARY_PATH);

    const repoPath = await createTestRepo({
      symlinks: { "shell/zshrc": ".zshrc" },
    });
    await setupTestRepoInContainer(container, repoPath, {
      conflicts: [".zshrc"],
    });

    // Run paw link without --force and with --no-interactive
    const result = await container.run("cd /home/testuser/dotfiles && paw link --no-interactive");

    expectSuccess(result);
    expect(result.stdout).toContain("exists");
  });

  test("creates backup with --force", async () => {
    container = await createContainer();
    await installPawInContainer(container, BINARY_PATH);

    const repoPath = await createTestRepo({
      symlinks: { "shell/zshrc": ".zshrc" },
    });
    await setupTestRepoInContainer(container, repoPath, {
      conflicts: [".zshrc"],
    });

    // Run paw link with --force
    const result = await container.run("cd /home/testuser/dotfiles && paw link --force");

    expectSuccess(result);
    expect(result.stdout).toContain("Backed up");

    // Verify backup exists
    const backupResult = await container.run("ls /home/testuser/.zshrc.backup.*");
    expectSuccess(backupResult);
  });

  test("dry-run shows what would be done", async () => {
    container = await createContainer();
    await installPawInContainer(container, BINARY_PATH);

    const repoPath = await createTestRepo({
      symlinks: { "shell/zshrc": ".zshrc" },
    });
    await setupTestRepoInContainer(container, repoPath);

    // Run paw link --dry-run
    const result = await container.run("cd /home/testuser/dotfiles && paw link --dry-run");

    expectSuccess(result);
    expect(result.stdout).toContain("dry-run");

    // Verify symlink was NOT created
    const checkResult = await container.run("test -L /home/testuser/.zshrc");
    expect(checkResult.exitCode).not.toBe(0);
  });
});
```

**Step 2: Commit**

```bash
git add tests/e2e/link.test.ts
git commit -m "test(e2e): add link command tests"
```

---

## Task 18: E2E Testing - Install and Rollback Tests

**Files:**
- Create: `tests/e2e/install.test.ts`
- Create: `tests/e2e/rollback.test.ts`

**Step 1: Create install tests**

```typescript
/**
 * E2E Tests: install command
 */

import { describe, test, expect, beforeAll, afterEach } from "bun:test";
import { buildTestImage, createContainer, installPawInContainer, type Container } from "./helpers/docker";
import { createTestRepo, setupTestRepoInContainer } from "./helpers/fixtures";
import { expectSymlink, expectSuccess } from "./helpers/assertions";
import { $ } from "bun";

const BINARY_PATH = new URL("../../dist/paw-linux-x64", import.meta.url).pathname;

describe("paw install", () => {
  let container: Container;

  beforeAll(async () => {
    await buildTestImage();
    await $`bun build src/index.ts --compile --target=bun-linux-x64 --outfile=dist/paw-linux-x64`.quiet();
  });

  afterEach(async () => {
    if (container) await container.cleanup();
  });

  test("installs packages and creates symlinks", async () => {
    container = await createContainer();
    await installPawInContainer(container, BINARY_PATH);

    const repoPath = await createTestRepo({
      symlinks: { "shell/zshrc": ".zshrc" },
      packages: { common: [] }, // Skip packages in test for speed
    });
    await setupTestRepoInContainer(container, repoPath);

    const result = await container.run("cd /home/testuser/dotfiles && paw install --skip-packages");

    expectSuccess(result);
    expect(result.stdout).toContain("Installation complete");
    await expectSymlink(container, "/home/testuser/.zshrc", "dotfiles/config/shell/zshrc");
  });

  test("runs pre/post install hooks", async () => {
    container = await createContainer();
    await installPawInContainer(container, BINARY_PATH);

    const repoPath = await createTestRepo({
      symlinks: { "shell/zshrc": ".zshrc" },
      hooks: {
        preInstall: 'console.log("PRE_INSTALL_HOOK_RAN")',
        postInstall: 'console.log("POST_INSTALL_HOOK_RAN")',
      },
    });
    await setupTestRepoInContainer(container, repoPath);

    const result = await container.run("cd /home/testuser/dotfiles && paw install --skip-packages");

    expectSuccess(result);
    expect(result.stdout).toContain("PRE_INSTALL_HOOK_RAN");
    expect(result.stdout).toContain("POST_INSTALL_HOOK_RAN");
  });
});
```

**Step 2: Create rollback tests**

```typescript
/**
 * E2E Tests: rollback command
 */

import { describe, test, expect, beforeAll, afterEach } from "bun:test";
import { buildTestImage, createContainer, installPawInContainer, type Container } from "./helpers/docker";
import { createTestRepo, setupTestRepoInContainer } from "./helpers/fixtures";
import { expectSuccess, expectFileExists } from "./helpers/assertions";
import { $ } from "bun";

const BINARY_PATH = new URL("../../dist/paw-linux-x64", import.meta.url).pathname;

describe("paw rollback", () => {
  let container: Container;

  beforeAll(async () => {
    await buildTestImage();
    await $`bun build src/index.ts --compile --target=bun-linux-x64 --outfile=dist/paw-linux-x64`.quiet();
  });

  afterEach(async () => {
    if (container) await container.cleanup();
  });

  test("removes symlinks and restores backups", async () => {
    container = await createContainer();
    await installPawInContainer(container, BINARY_PATH);

    const repoPath = await createTestRepo({
      symlinks: { "shell/zshrc": ".zshrc" },
    });
    await setupTestRepoInContainer(container, repoPath, {
      conflicts: [".zshrc"],
    });

    // Install with force (creates backup)
    await container.run("cd /home/testuser/dotfiles && paw install --skip-packages --force");

    // Verify symlink exists
    await expectFileExists(container, "/home/testuser/.zshrc");

    // Rollback
    const result = await container.run("cd /home/testuser/dotfiles && paw rollback");

    expectSuccess(result);
    expect(result.stdout).toContain("Rollback complete");

    // Verify original file is restored (not a symlink)
    const typeResult = await container.run("test -L /home/testuser/.zshrc");
    expect(typeResult.exitCode).not.toBe(0); // Should NOT be a symlink

    const contentResult = await container.run("cat /home/testuser/.zshrc");
    expect(contentResult.stdout).toContain("existing content");
  });

  test("fails gracefully with no previous state", async () => {
    container = await createContainer();
    await installPawInContainer(container, BINARY_PATH);

    const repoPath = await createTestRepo({
      symlinks: { "shell/zshrc": ".zshrc" },
    });
    await setupTestRepoInContainer(container, repoPath);

    // Try rollback without previous install
    const result = await container.run("cd /home/testuser/dotfiles && paw rollback");

    expect(result.exitCode).not.toBe(0);
    expect(result.stdout + result.stderr).toContain("No previous run state");
  });
});
```

**Step 3: Commit**

```bash
git add tests/e2e/install.test.ts tests/e2e/rollback.test.ts
git commit -m "test(e2e): add install and rollback command tests"
```

---

## Task 19: E2E Testing - Status and Doctor Tests

**Files:**
- Create: `tests/e2e/status.test.ts`

**Step 1: Create status and doctor tests**

```typescript
/**
 * E2E Tests: status and doctor commands
 */

import { describe, test, expect, beforeAll, afterEach } from "bun:test";
import { buildTestImage, createContainer, installPawInContainer, type Container } from "./helpers/docker";
import { createTestRepo, setupTestRepoInContainer } from "./helpers/fixtures";
import { expectSuccess } from "./helpers/assertions";
import { $ } from "bun";

const BINARY_PATH = new URL("../../dist/paw-linux-x64", import.meta.url).pathname;

describe("paw status", () => {
  let container: Container;

  beforeAll(async () => {
    await buildTestImage();
    await $`bun build src/index.ts --compile --target=bun-linux-x64 --outfile=dist/paw-linux-x64`.quiet();
  });

  afterEach(async () => {
    if (container) await container.cleanup();
  });

  test("shows symlink status", async () => {
    container = await createContainer();
    await installPawInContainer(container, BINARY_PATH);

    const repoPath = await createTestRepo({
      symlinks: {
        "shell/zshrc": ".zshrc",
        "git/gitconfig": ".gitconfig",
      },
    });
    await setupTestRepoInContainer(container, repoPath);

    // Link one file
    await container.run("cd /home/testuser/dotfiles && paw link");

    // Check status
    const result = await container.run("cd /home/testuser/dotfiles && paw status");

    expectSuccess(result);
    expect(result.stdout).toContain("Symlinks");
    expect(result.stdout).toContain(".zshrc");
    expect(result.stdout).toContain("linked");
  });

  test("shows conflicts", async () => {
    container = await createContainer();
    await installPawInContainer(container, BINARY_PATH);

    const repoPath = await createTestRepo({
      symlinks: { "shell/zshrc": ".zshrc" },
    });
    await setupTestRepoInContainer(container, repoPath, {
      conflicts: [".zshrc"],
    });

    const result = await container.run("cd /home/testuser/dotfiles && paw status");

    expectSuccess(result);
    expect(result.stdout).toContain("conflict");
  });
});

describe("paw doctor", () => {
  let container: Container;

  beforeAll(async () => {
    await buildTestImage();
  });

  afterEach(async () => {
    if (container) await container.cleanup();
  });

  test("reports system info and checks", async () => {
    container = await createContainer();
    await installPawInContainer(container, BINARY_PATH);

    const repoPath = await createTestRepo({
      symlinks: { "shell/zshrc": ".zshrc" },
    });
    await setupTestRepoInContainer(container, repoPath);

    const result = await container.run("cd /home/testuser/dotfiles && paw doctor");

    expectSuccess(result);
    expect(result.stdout).toContain("System");
    expect(result.stdout).toContain("linux");
    expect(result.stdout).toContain("Required Tools");
    expect(result.stdout).toContain("git");
  });
});
```

**Step 2: Commit**

```bash
git add tests/e2e/status.test.ts
git commit -m "test(e2e): add status and doctor command tests"
```

---

## Task 20: E2E Testing - Machine-Specific Config Tests

**Files:**
- Create: `tests/e2e/machine-specific.test.ts`

**Step 1: Create machine-specific tests**

```typescript
/**
 * E2E Tests: Machine-specific configurations
 */

import { describe, test, expect, beforeAll, afterEach } from "bun:test";
import { buildTestImage, createContainer, installPawInContainer, type Container } from "./helpers/docker";
import { expectSuccess, expectFileNotExists } from "./helpers/assertions";
import { mkdtemp, writeFile, mkdir } from "fs/promises";
import { tmpdir } from "os";
import { join } from "path";
import { $ } from "bun";

const BINARY_PATH = new URL("../../dist/paw-linux-x64", import.meta.url).pathname;

describe("machine-specific configs", () => {
  let container: Container;

  beforeAll(async () => {
    await buildTestImage();
    await $`bun build src/index.ts --compile --target=bun-linux-x64 --outfile=dist/paw-linux-x64`.quiet();
  });

  afterEach(async () => {
    if (container) await container.cleanup();
  });

  test("skips symlinks that don't match platform condition", async () => {
    container = await createContainer();
    await installPawInContainer(container, BINARY_PATH);

    // Create repo with darwin-only symlink (container is linux)
    const repoDir = await mkdtemp(join(tmpdir(), "paw-test-"));
    await mkdir(join(repoDir, "config/macos"), { recursive: true });
    await writeFile(join(repoDir, "config/macos/settings"), "# macOS settings");

    const configContent = `
import { defineConfig } from "paw";
export default defineConfig({
  symlinks: {
    "macos/settings": {
      target: ".macos-settings",
      when: { platform: "darwin" }
    }
  },
  packages: { common: [] },
  templates: {},
  ignore: [],
});
`;
    await writeFile(join(repoDir, "dotfiles.config.ts"), configContent);
    await $`git -C ${repoDir} init && git -C ${repoDir} add -A && git -C ${repoDir} commit -m "init"`.quiet();

    await container.copyTo(repoDir, "/home/testuser/dotfiles");
    await container.run("chown -R testuser:testuser /home/testuser/dotfiles");

    const result = await container.run("cd /home/testuser/dotfiles && paw link");

    expectSuccess(result);
    expect(result.stdout).toContain("skipped");
    expect(result.stdout).toContain("platform darwin");

    // File should not exist
    await expectFileNotExists(container, "/home/testuser/.macos-settings");
  });

  test("creates symlinks that match platform condition", async () => {
    container = await createContainer();
    await installPawInContainer(container, BINARY_PATH);

    // Create repo with linux-only symlink
    const repoDir = await mkdtemp(join(tmpdir(), "paw-test-"));
    await mkdir(join(repoDir, "config/linux"), { recursive: true });
    await writeFile(join(repoDir, "config/linux/settings"), "# Linux settings");

    const configContent = `
import { defineConfig } from "paw";
export default defineConfig({
  symlinks: {
    "linux/settings": {
      target: ".linux-settings",
      when: { platform: "linux" }
    }
  },
  packages: { common: [] },
  templates: {},
  ignore: [],
});
`;
    await writeFile(join(repoDir, "dotfiles.config.ts"), configContent);
    await $`git -C ${repoDir} init && git -C ${repoDir} add -A && git -C ${repoDir} commit -m "init"`.quiet();

    await container.copyTo(repoDir, "/home/testuser/dotfiles");
    await container.run("chown -R testuser:testuser /home/testuser/dotfiles");

    const result = await container.run("cd /home/testuser/dotfiles && paw link");

    expectSuccess(result);
    expect(result.stdout).not.toContain("skipped");

    // File should exist as symlink
    const checkResult = await container.run("test -L /home/testuser/.linux-settings");
    expect(checkResult.exitCode).toBe(0);
  });
});
```

**Step 2: Commit**

```bash
git add tests/e2e/machine-specific.test.ts
git commit -m "test(e2e): add machine-specific config tests"
```

---

## Task 21: E2E Testing - Package.json Scripts

**Files:**
- Modify: `package.json`

**Step 1: Add test scripts**

Add to scripts section:

```json
"test:e2e": "bun test tests/e2e/",
"test:e2e:build": "docker build -t paw-e2e-test -f tests/e2e/docker/Dockerfile .",
"test": "bun test"
```

**Step 2: Run tests**

Run: `bun run test:e2e:build && bun run test:e2e`
Expected: All tests pass

**Step 3: Commit**

```bash
git add package.json
git commit -m "chore: add E2E test scripts to package.json"
```

---

## Task 22: Final Integration Test

**Step 1: Run full typecheck**

Run: `bun run typecheck`
Expected: PASS

**Step 2: Run all E2E tests**

Run: `bun run test:e2e`
Expected: All tests pass

**Step 3: Manual verification**

Run: `bun run dev status`
Expected: Shows current symlink status

Run: `bun run dev link --dry-run`
Expected: Shows what would be done

**Step 4: Final commit**

```bash
git add -A
git commit -m "feat: complete paw enhancements (machine configs, hooks, interactive, verification, e2e)"
```

---

## Summary

| Task | Feature | Files Changed |
|------|---------|---------------|
| 1-3 | Machine-Specific Configs | types, os, symlinks |
| 4-9 | Full Lifecycle Hooks | types, hooks, sync, push, update, backup |
| 10-12 | Interactive Conflicts | prompt, symlinks, index |
| 13-14 | Binary Verification | release.yml, update, index |
| 15-21 | E2E Testing | docker, helpers, tests |
| 22 | Integration | - |

Total: 22 tasks, ~15 new/modified files
