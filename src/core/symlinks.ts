/**
 * Symlink Management
 * Handles creation, removal, and status checking of symlinks
 */

import { mkdir, symlink, unlink, readlink, rename, lstat, stat } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import type { SymlinkState, InstallOptions, SymlinkEntry } from "../types";
import { logger } from "./logger";
import { getHomeDir, contractPath } from "./os";
import { resolveConfigPath } from "./config";

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
  options: InstallOptions
): Promise<SymlinkState> {
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
    return state;
  }

  // Check if target already exists
  const targetExists = await fileOrLinkExists(target);

  if (targetExists) {
    // Check if it's already the correct symlink
    const isCorrectLink = await isSymlinkTo(target, source);

    if (isCorrectLink) {
      logger.skip(`${contractPath(target)} (already linked)`);
      state.status = "linked";
      return state;
    }

    // Handle conflict
    if (!options.force) {
      logger.warn(`Conflict: ${contractPath(target)} exists (use --force to backup and replace)`);
      state.status = "conflict";
      return state;
    }

    // Create backup
    if (options.dryRun) {
      logger.dryRun(`Would backup ${contractPath(target)}`);
    } else {
      const backupPath = await createBackup(target);
      logger.info(`Backed up ${contractPath(target)} to ${contractPath(backupPath)}`);
      state.backupPath = backupPath;
    }
    state.status = "backup";
  }

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

/**
 * Create symlinks for all configured files
 */
export async function createSymlinks(
  symlinks: Record<string, string>,
  options: InstallOptions
): Promise<SymlinkState[]> {
  const homeDir = getHomeDir();
  const states: SymlinkState[] = [];

  for (const [sourceRel, targetRel] of Object.entries(symlinks)) {
    const source = resolveConfigPath(sourceRel);
    const target = resolve(homeDir, targetRel);

    const state = await createSymlink(source, target, options);
    states.push(state);
  }

  return states;
}

/**
 * Remove all managed symlinks
 */
export async function removeSymlinks(
  symlinks: Record<string, string>,
  options: InstallOptions
): Promise<void> {
  const homeDir = getHomeDir();

  for (const [_, targetRel] of Object.entries(symlinks)) {
    const target = resolve(homeDir, targetRel);

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
  symlinks: Record<string, string>
): Promise<SymlinkState[]> {
  const homeDir = getHomeDir();
  const states: SymlinkState[] = [];

  for (const [sourceRel, targetRel] of Object.entries(symlinks)) {
    const source = resolveConfigPath(sourceRel);
    const target = resolve(homeDir, targetRel);

    const state: SymlinkState = {
      source,
      target,
      status: "missing",
    };

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
