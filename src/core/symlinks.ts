/**
 * Symlink Management
 * Handles creation, removal, and status checking of symlinks
 */

import { mkdir, symlink, unlink, readlink, rename, lstat, stat } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import type { SymlinkState, InstallOptions, SymlinkEntry, SymlinkTarget, SymlinkCondition } from "../types";
import { logger } from "./logger";
import { getHomeDir, contractPath, validatePathWithinBase, getPlatform, matchGlob, getHostname } from "./os";
import { resolveConfigPath } from "./config";
import { isInteractive, promptConflict, type ConflictChoice } from "./prompt";

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

/**
 * Check if a file or symlink exists at the given path
 */
async function fileOrLinkExists(path: string): Promise<boolean> {
  try {
    await lstat(path);
    return true;
  } catch {
    return false;
  }
}

/**
 * Check if a path is a symlink pointing to the expected target
 */
async function isSymlinkTo(linkPath: string, expectedTarget: string): Promise<boolean> {
  try {
    const stats = await lstat(linkPath);
    if (!stats.isSymbolicLink()) return false;

    const actualTarget = await readlink(linkPath);
    // Handle both absolute paths and relative resolution
    const resolvedActual = resolve(dirname(linkPath), actualTarget);
    return resolvedActual === expectedTarget || actualTarget === expectedTarget;
  } catch {
    return false;
  }
}

/**
 * Create a timestamped backup of a file
 */
async function createBackup(filePath: string): Promise<string> {
  const timestamp = Date.now();
  const backupPath = `${filePath}.backup.${timestamp}`;
  await rename(filePath, backupPath);
  return backupPath;
}

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

  // Only set to linked if not already marked as backup
  if (state.status !== "backup") {
    state.status = "linked";
  }
  return state;
}

/**
 * Create symlinks for all configured files
 */
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

/**
 * Remove all managed symlinks
 */
export async function removeSymlinks(
  symlinks: Record<string, SymlinkTarget>,
  options: InstallOptions
): Promise<void> {
  const homeDir = getHomeDir();

  for (const [_, targetConfig] of Object.entries(symlinks)) {
    const { targetPath } = normalizeSymlinkTarget(targetConfig);
    const target = resolve(homeDir, targetPath);

    // Security: Prevent path traversal attacks
    validatePathWithinBase(target, homeDir, "Symlink target");

    try {
      const stats = await lstat(target);

      if (stats.isSymbolicLink()) {
        if (options.dryRun) {
          logger.dryRun(`Would remove symlink: ${contractPath(target)}`);
        } else {
          await unlink(target);
          logger.success(`Removed: ${contractPath(target)}`);
        }
      } else {
        logger.warn(`Skipping ${contractPath(target)} (not a symlink)`);
      }
    } catch {
      logger.skip(`${contractPath(target)} (not found)`);
    }
  }
}

/**
 * Get the current status of all symlinks
 */
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
    const { create } = shouldCreateSymlink(condition);

    // Security: Prevent path traversal attacks
    validatePathWithinBase(target, homeDir, "Symlink target");

    const state: SymlinkState = {
      source,
      target,
      status: "missing",
    };

    // If condition not met, show as skipped in status
    if (!create) {
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

/**
 * Convert symlink states to entries for the last run file
 */
export function statesToEntries(states: SymlinkState[]): SymlinkEntry[] {
  return states
    .filter(s => s.status === "linked")
    .map(s => ({
      source: s.source,
      target: s.target,
    }));
}
