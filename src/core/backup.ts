/**
 * Backup Management
 * Handles backup listing, restoration, cleanup, and rollback state
 */

import { readdir, stat, unlink, rename } from "node:fs/promises";
import { resolve, dirname, basename } from "node:path";
import type { BackupEntry, LastRunState, BackupConfig, InstallOptions } from "../types";
import { logger } from "./logger";
import { getHomeDir, contractPath } from "./os";

/** Pattern for backup files: filename.backup.timestamp */
const BACKUP_PATTERN = /^(.+)\.backup\.(\d+)$/;

/** Path to the last run state file */
const LAST_RUN_FILE = ".dotfiles-last-run.json";

/**
 * Get the path to the last run state file
 */
function getLastRunPath(): string {
  return resolve(getHomeDir(), LAST_RUN_FILE);
}

/**
 * Save the last run state
 */
export async function saveLastRunState(state: LastRunState): Promise<void> {
  const path = getLastRunPath();
  await Bun.write(path, JSON.stringify(state, null, 2));
  logger.debug(`Saved last run state to ${contractPath(path)}`, true);
}

/**
 * Load the last run state
 */
export async function loadLastRunState(): Promise<LastRunState | null> {
  const path = getLastRunPath();
  const file = Bun.file(path);

  if (!(await file.exists())) {
    return null;
  }

  try {
    const content = await file.text();
    return JSON.parse(content) as LastRunState;
  } catch (error) {
    logger.warn(`Failed to parse last run state: ${error}`);
    return null;
  }
}

/**
 * Find all backup files in a directory
 */
async function findBackupsInDir(dir: string): Promise<BackupEntry[]> {
  const backups: BackupEntry[] = [];

  try {
    const entries = await readdir(dir);

    for (const entry of entries) {
      const match = entry.match(BACKUP_PATTERN);
      if (match) {
        const [, originalName, timestampStr] = match;
        const timestamp = parseInt(timestampStr, 10);
        const backupPath = resolve(dir, entry);
        const originalPath = resolve(dir, originalName);

        backups.push({
          original: originalPath,
          backup: backupPath,
          timestamp,
        });
      }
    }
  } catch {
    // Directory doesn't exist or can't be read
  }

  return backups;
}

/**
 * Find all backup files across common dotfile locations
 */
export async function findAllBackups(): Promise<BackupEntry[]> {
  const homeDir = getHomeDir();
  const allBackups: BackupEntry[] = [];

  // Directories to search for backups
  const dirsToSearch = [
    homeDir,
    resolve(homeDir, ".config"),
    resolve(homeDir, ".config/starship"),
    resolve(homeDir, ".config/ghostty"),
    resolve(homeDir, ".config/kitty"),
    resolve(homeDir, ".claude"),
  ];

  for (const dir of dirsToSearch) {
    const backups = await findBackupsInDir(dir);
    allBackups.push(...backups);
  }

  // Sort by timestamp (newest first)
  allBackups.sort((a, b) => b.timestamp - a.timestamp);

  return allBackups;
}

/**
 * List all backup files
 */
export async function listBackups(): Promise<void> {
  const backups = await findAllBackups();

  if (backups.length === 0) {
    logger.info("No backup files found.");
    return;
  }

  logger.header("Backup Files");

  for (const backup of backups) {
    const date = new Date(backup.timestamp).toLocaleString();
    console.log(`  ${contractPath(backup.backup)}`);
    console.log(`    Original: ${contractPath(backup.original)}`);
    console.log(`    Created:  ${date}`);
    console.log();
  }

  logger.info(`Total: ${backups.length} backup(s)`);
}

/**
 * Restore a specific backup
 */
export async function restoreBackup(
  backupPath: string,
  options: InstallOptions
): Promise<boolean> {
  const expandedPath = backupPath.startsWith("~")
    ? resolve(getHomeDir(), backupPath.slice(2))
    : resolve(backupPath);

  // Parse backup filename to get original name
  const filename = basename(expandedPath);
  const match = filename.match(BACKUP_PATTERN);

  if (!match) {
    logger.error(`Not a valid backup file: ${backupPath}`);
    return false;
  }

  const [, originalName] = match;
  const originalPath = resolve(dirname(expandedPath), originalName);

  // Check if backup exists
  try {
    await stat(expandedPath);
  } catch {
    logger.error(`Backup file not found: ${contractPath(expandedPath)}`);
    return false;
  }

  if (options.dryRun) {
    logger.dryRun(`Would restore ${contractPath(expandedPath)} to ${contractPath(originalPath)}`);
    return true;
  }

  // Remove current file if it exists
  try {
    await unlink(originalPath);
  } catch {
    // File doesn't exist, that's fine
  }

  // Restore backup
  await rename(expandedPath, originalPath);
  logger.success(`Restored ${contractPath(originalPath)} from backup`);

  return true;
}

/**
 * Clean old backups based on retention policy
 */
export async function cleanBackups(
  config: BackupConfig,
  options: InstallOptions
): Promise<number> {
  const backups = await findAllBackups();
  const now = Date.now();
  const maxAgeMs = config.maxAge * 24 * 60 * 60 * 1000;

  // Group backups by original file
  const byOriginal = new Map<string, BackupEntry[]>();
  for (const backup of backups) {
    const existing = byOriginal.get(backup.original) ?? [];
    existing.push(backup);
    byOriginal.set(backup.original, existing);
  }

  let removed = 0;

  for (const [original, fileBackups] of byOriginal) {
    // Sort by timestamp (newest first)
    fileBackups.sort((a, b) => b.timestamp - a.timestamp);

    for (let i = 0; i < fileBackups.length; i++) {
      const backup = fileBackups[i];
      const age = now - backup.timestamp;
      const isOld = age > maxAgeMs;
      const exceedsCount = i >= config.maxCount;

      if (isOld || exceedsCount) {
        if (options.dryRun) {
          logger.dryRun(`Would remove: ${contractPath(backup.backup)}`);
        } else {
          try {
            await unlink(backup.backup);
            logger.success(`Removed: ${contractPath(backup.backup)}`);
            removed++;
          } catch (error) {
            logger.error(`Failed to remove ${contractPath(backup.backup)}: ${error}`);
          }
        }
      }
    }
  }

  return removed;
}

/**
 * Rollback to the state before the last install/link
 */
export async function rollback(options: InstallOptions): Promise<boolean> {
  const lastRun = await loadLastRunState();

  if (!lastRun) {
    logger.error("No previous run state found. Cannot rollback.");
    return false;
  }

  logger.header("Rolling Back");
  logger.info(`Last run: ${lastRun.command} at ${lastRun.timestamp}`);

  // Remove symlinks that were created
  logger.subheader("Removing symlinks");
  for (const entry of lastRun.symlinks) {
    if (options.dryRun) {
      logger.dryRun(`Would remove symlink: ${contractPath(entry.target)}`);
    } else {
      try {
        await unlink(entry.target);
        logger.success(`Removed: ${contractPath(entry.target)}`);
      } catch {
        logger.skip(`${contractPath(entry.target)} (not found)`);
      }
    }
  }

  // Restore backups
  if (lastRun.backups.length > 0) {
    logger.subheader("Restoring backups");
    for (const backup of lastRun.backups) {
      if (options.dryRun) {
        logger.dryRun(`Would restore: ${contractPath(backup.original)}`);
      } else {
        try {
          await rename(backup.backup, backup.original);
          logger.success(`Restored: ${contractPath(backup.original)}`);
        } catch (error) {
          logger.error(`Failed to restore ${contractPath(backup.original)}: ${error}`);
        }
      }
    }
  }

  // Remove the last run state file
  if (!options.dryRun) {
    try {
      await unlink(getLastRunPath());
    } catch {
      // File doesn't exist, that's fine
    }
  }

  logger.newline();
  logger.success("Rollback complete!");
  return true;
}
