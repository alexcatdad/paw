/**
 * Push Command
 * Stage, commit, and push dotfiles changes
 */

import { $ } from "bun";
import { logger } from "./logger";
import { getDotfilesPath } from "./paw-config";
import type { InstallOptions } from "../types";

/**
 * Push dotfiles changes to remote
 */
export async function runPush(message: string | undefined, options: InstallOptions): Promise<boolean> {
  const repoPath = await getDotfilesPath();

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
  const changedFiles = changes.split("\n").length;
  logger.info(`${changedFiles} file(s) changed`);

  if (options.verbose) {
    logger.newline();
    console.log(changes);
    logger.newline();
  }

  if (options.dryRun) {
    logger.info("Would stage, commit, and push changes");
    return true;
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

  // Push
  logger.info("Pushing to remote...");
  const pushResult = await $`git -C ${repoPath} push`.quiet().nothrow();
  if (pushResult.exitCode !== 0) {
    logger.error("Failed to push. You may need to pull first: paw sync");
    logger.error(pushResult.stderr.toString());
    return false;
  }

  logger.success(`Pushed: ${commitMessage}`);
  return true;
}
