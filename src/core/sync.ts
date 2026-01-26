/**
 * Sync Module
 * Orchestrates dotfiles synchronization: update check, git pull, link refresh
 */

import { $ } from "bun";
import { logger } from "./logger";
import { getRepoDir } from "./os";
import { checkForUpdate, performUpdate } from "./update";
import { loadConfig } from "./config";
import { createSymlinks, statesToEntries } from "./symlinks";
import { saveLastRunState } from "./backup";
import type { SyncResult, SyncOptions, BackupEntry } from "../types";

/**
 * Check if the repo is behind origin
 */
export async function getRepoStatus(): Promise<{
  behind: boolean;
  ahead: boolean;
  commits: number;
}> {
  const repoDir = getRepoDir();

  try {
    // Fetch from origin (quiet, no output)
    await $`git -C ${repoDir} fetch origin --quiet`.quiet().nothrow();

    // Get the status relative to upstream
    const result = await $`git -C ${repoDir} rev-list --left-right --count HEAD...@{upstream}`.quiet().nothrow();

    if (result.exitCode !== 0) {
      // No upstream tracking branch set
      return { behind: false, ahead: false, commits: 0 };
    }

    const [ahead, behind] = result.text().trim().split(/\s+/).map(Number);

    return {
      behind: behind > 0,
      ahead: ahead > 0,
      commits: behind,
    };
  } catch {
    return { behind: false, ahead: false, commits: 0 };
  }
}

/**
 * Get the current HEAD commit
 */
async function getCurrentHead(): Promise<string | null> {
  const repoDir = getRepoDir();
  try {
    const result = await $`git -C ${repoDir} rev-parse HEAD`.quiet().nothrow();
    if (result.exitCode === 0) {
      return result.text().trim();
    }
  } catch {
    // Ignore
  }
  return null;
}

/**
 * Pull the dotfiles repo
 * Returns the list of changed files
 */
export async function pullDotfilesRepo(options: SyncOptions): Promise<string[]> {
  const repoDir = getRepoDir();
  const oldHead = await getCurrentHead();

  if (options.dryRun) {
    if (!options.quiet) {
      logger.info("Would run: git pull --rebase");
    }
    return [];
  }

  try {
    // Check for local changes
    const statusResult = await $`git -C ${repoDir} status --porcelain`.quiet().nothrow();
    const hasLocalChanges = statusResult.text().trim().length > 0;

    if (hasLocalChanges) {
      // Stash local changes
      if (!options.quiet) {
        logger.info("Stashing local changes...");
      }
      await $`git -C ${repoDir} stash push -m "paw-sync-auto-stash"`.quiet();
    }

    // Pull with rebase
    const pullResult = await $`git -C ${repoDir} pull --rebase --quiet`.quiet().nothrow();

    if (pullResult.exitCode !== 0) {
      const stderr = pullResult.stderr.toString();

      // Check if it's a rebase conflict
      if (stderr.includes("conflict") || stderr.includes("CONFLICT")) {
        logger.error("Merge conflict detected. Aborting rebase...");
        await $`git -C ${repoDir} rebase --abort`.quiet().nothrow();

        if (hasLocalChanges) {
          await $`git -C ${repoDir} stash pop`.quiet().nothrow();
        }

        return [];
      }

      if (!options.quiet) {
        logger.warn(`Git pull failed: ${stderr}`);
      }
      return [];
    }

    // Pop stash if we had local changes
    if (hasLocalChanges) {
      if (!options.quiet) {
        logger.info("Restoring local changes...");
      }
      await $`git -C ${repoDir} stash pop`.quiet().nothrow();
    }

    // Get changed files between old and new HEAD
    const newHead = await getCurrentHead();
    if (oldHead && newHead && oldHead !== newHead) {
      const diffResult = await $`git -C ${repoDir} diff --name-only ${oldHead} ${newHead}`.quiet().nothrow();
      if (diffResult.exitCode === 0) {
        return diffResult.text().trim().split("\n").filter(Boolean);
      }
    }

    return [];
  } catch (error) {
    if (!options.quiet) {
      logger.error(`Pull failed: ${error}`);
    }
    return [];
  }
}

/**
 * Check if symlinks should be refreshed based on changed files
 */
export function shouldRefreshLinks(changedFiles: string[]): boolean {
  // Refresh if config/ directory or dotfiles.config.ts changed
  return changedFiles.some(
    (file) => file.startsWith("config/") || file === "dotfiles.config.ts"
  );
}

/**
 * Run the full sync operation
 */
export async function runSync(options: SyncOptions): Promise<SyncResult> {
  const result: SyncResult = {
    pawUpdated: false,
    repoUpdated: false,
    linksRefreshed: false,
  };

  // Step 1: Check for paw updates (unless skipped)
  if (!options.skipUpdate) {
    const latestVersion = await checkForUpdate({ force: options.dryRun });

    if (latestVersion) {
      if (!options.quiet) {
        logger.info(`paw update available: v${latestVersion}`);
      }

      if (options.autoUpdate) {
        if (!options.quiet) {
          logger.subheader("Updating paw");
        }
        result.pawUpdated = await performUpdate(options);
      } else if (!options.quiet) {
        logger.info("Run 'paw update' to install the latest version");
      }
    }
  }

  // Step 2: Check repo status
  if (!options.quiet) {
    logger.subheader("Checking dotfiles repo");
  }

  const repoStatus = await getRepoStatus();

  if (!repoStatus.behind) {
    if (!options.quiet) {
      logger.success("Dotfiles repo is up to date");
    }
    return result;
  }

  if (!options.quiet) {
    logger.info(`${repoStatus.commits} commit(s) behind origin`);
  }

  // Step 3: Pull changes
  if (!options.quiet) {
    logger.info("Pulling changes...");
  }

  const changedFiles = await pullDotfilesRepo(options);

  if (changedFiles.length === 0 && !options.dryRun) {
    if (!options.quiet) {
      logger.info("No files changed");
    }
    return result;
  }

  result.repoUpdated = true;

  if (!options.quiet && changedFiles.length > 0) {
    logger.info(`Updated ${changedFiles.length} file(s)`);
  }

  // Step 4: Refresh symlinks if needed
  if (shouldRefreshLinks(changedFiles)) {
    if (!options.quiet) {
      logger.subheader("Refreshing symlinks");
    }

    const config = await loadConfig();
    const backups: BackupEntry[] = [];

    const states = await createSymlinks(config.symlinks, {
      ...options,
      force: true, // Force refresh since config changed
    });

    // Collect backup info
    for (const state of states) {
      if (state.backupPath) {
        backups.push({
          original: state.target,
          backup: state.backupPath,
          timestamp: Date.now(),
        });
      }
    }

    // Save state for rollback
    if (!options.dryRun) {
      await saveLastRunState({
        timestamp: new Date().toISOString(),
        command: "sync",
        backups,
        symlinks: statesToEntries(states),
      });
    }

    result.linksRefreshed = true;

    const linked = states.filter((s) => s.status === "linked").length;
    if (!options.quiet) {
      logger.info(`${linked} symlink(s) refreshed`);
    }
  }

  return result;
}

/**
 * Print sync summary
 */
export function printSyncSummary(result: SyncResult): void {
  if (!result.pawUpdated && !result.repoUpdated && !result.linksRefreshed) {
    logger.success("Everything is up to date!");
    return;
  }

  logger.newline();
  logger.subheader("Sync Summary");

  const items: string[] = [];
  if (result.pawUpdated) items.push("paw binary updated");
  if (result.repoUpdated) items.push("dotfiles pulled");
  if (result.linksRefreshed) items.push("symlinks refreshed");

  logger.table({
    "Actions completed": items.join(", ") || "none",
  });

  logger.newline();
  logger.success("Sync complete!");
}
