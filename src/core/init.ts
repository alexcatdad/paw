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
 * Initialize paw with a dotfiles repository
 */
export async function runInit(repoUrl: string, options: InstallOptions & { path?: string }): Promise<boolean> {
  logger.header("Paw Init");

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
      if (existingRemote === repoUrl || existingRemote === `${repoUrl}.git`) {
        logger.info("Repository already cloned, pulling latest...");
        await $`git -C ${clonePath} pull --rebase`.quiet().nothrow();
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
