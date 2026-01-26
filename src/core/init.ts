/**
 * Init Command
 * Clone dotfiles repo and configure paw
 */

import { $ } from "bun";
import { logger } from "./logger";
import { getHomeDir } from "./os";
import { savePawConfig, loadPawConfig } from "./paw-config";
import type { InstallOptions } from "../types";

/**
 * Normalize a git URL for comparison
 * Handles SSH (git@github.com:user/repo) vs HTTPS (https://github.com/user/repo)
 */
function normalizeGitUrl(url: string): string {
  let normalized = url.trim();

  // Remove trailing .git
  normalized = normalized.replace(/\.git$/, "");

  // Convert SSH to HTTPS format for comparison
  // git@github.com:user/repo -> github.com/user/repo
  const sshMatch = normalized.match(/^git@([^:]+):(.+)$/);
  if (sshMatch) {
    normalized = `${sshMatch[1]}/${sshMatch[2]}`;
  }

  // Extract domain/path from HTTPS
  // https://github.com/user/repo -> github.com/user/repo
  const httpsMatch = normalized.match(/^https?:\/\/(.+)$/);
  if (httpsMatch) {
    normalized = httpsMatch[1];
  }

  return normalized.toLowerCase();
}

/**
 * Validate that a string looks like a git repository URL
 */
function isValidGitUrl(url: string): boolean {
  // HTTPS format
  if (/^https?:\/\/[^/]+\/.+/.test(url)) {
    return true;
  }
  // SSH format
  if (/^git@[^:]+:.+/.test(url)) {
    return true;
  }
  return false;
}

/**
 * Initialize paw with a dotfiles repository
 */
export async function runInit(repoUrl: string, options: InstallOptions & { path?: string }): Promise<boolean> {
  logger.header("Paw Init");

  // Validate repoUrl format
  if (!isValidGitUrl(repoUrl)) {
    logger.error("Invalid repository URL format");
    logger.info("Expected: https://github.com/user/repo or git@github.com:user/repo");
    return false;
  }

  // Check if already initialized
  const existingConfig = await loadPawConfig();
  if (existingConfig && !options.force) {
    logger.warn(`Already initialized with: ${existingConfig.repoUrl}`);
    logger.info(`Dotfiles at: ${existingConfig.dotfilesRepo}`);
    logger.info("Use --force to reinitialize");
    return false;
  }

  // Determine clone path
  const homeDir = getHomeDir();
  const clonePath = options.path ?? `${homeDir}/dotfiles`;

  logger.table({
    "Repository": repoUrl,
    "Clone to": clonePath,
    "Dry run": options.dryRun ? "Yes" : "No",
  });
  logger.newline();

  if (options.dryRun) {
    logger.info("Would clone repository and run paw install");
    return true;
  }

  // Check if path already exists
  const { existsSync } = await import("fs");

  if (existsSync(clonePath)) {
    // Check if it's a git repo with the same remote
    const remoteResult = await $`git -C ${clonePath} remote get-url origin`.quiet().nothrow();
    if (remoteResult.exitCode === 0) {
      const existingRemote = remoteResult.text().trim();
      // Use normalized comparison to handle SSH vs HTTPS differences
      if (normalizeGitUrl(existingRemote) === normalizeGitUrl(repoUrl)) {
        logger.info("Repository already cloned, pulling latest...");
        const pullResult = await $`git -C ${clonePath} pull --rebase`.quiet().nothrow();
        if (pullResult.exitCode !== 0) {
          logger.warn("Pull failed, continuing with existing state");
          const stderr = pullResult.stderr.toString().trim();
          if (stderr) {
            logger.debug(stderr, options.verbose);
          }
        }
      } else {
        logger.error(`Directory exists with different remote: ${existingRemote}`);
        return false;
      }
    } else {
      logger.error(`Directory exists but is not a git repository: ${clonePath}`);
      return false;
    }
  } else {
    // Clone the repository
    logger.info("Cloning repository...");
    const cloneResult = await $`git clone ${repoUrl} ${clonePath}`.nothrow();
    if (cloneResult.exitCode !== 0) {
      logger.error("Failed to clone repository");
      logger.error(cloneResult.stderr.toString());
      return false;
    }
    logger.success("Repository cloned");
  }

  // Save paw configuration
  logger.info("Saving paw configuration...");
  await savePawConfig({
    dotfilesRepo: clonePath.replace(homeDir, "~"),
    repoUrl,
  });
  logger.success(`Config saved to ~/.config/paw/config.json`);

  // Set the repo dir environment variable for this session
  process.env.PAW_REPO = clonePath;

  return true;
}
