/**
 * Configuration Loader
 * Loads and validates the dotfiles.config.ts file
 */

import { resolve } from "node:path";
import type { DotfilesConfig, BackupConfig } from "../types";
import { getRepoDir } from "./os";
import { logger } from "./logger";

/**
 * Default backup configuration
 */
const DEFAULT_BACKUP_CONFIG: BackupConfig = {
  enabled: true,
  maxAge: 30,
  maxCount: 5,
};

/**
 * Helper function for creating a typed config
 * Used in dotfiles.config.ts for type safety
 */
export function defineConfig(config: DotfilesConfig): DotfilesConfig {
  return {
    ...config,
    backup: {
      ...DEFAULT_BACKUP_CONFIG,
      ...config.backup,
    },
  };
}

/**
 * Load the dotfiles configuration from the repo
 */
export async function loadConfig(): Promise<DotfilesConfig> {
  const repoDir = getRepoDir();
  const configPath = resolve(repoDir, "dotfiles.config.ts");

  try {
    // Dynamic import of the config file
    const configModule = await import(configPath);
    const config = configModule.default as DotfilesConfig;

    // Validate required fields
    if (!config.symlinks || typeof config.symlinks !== "object") {
      throw new Error("Config must have a 'symlinks' object");
    }

    if (!config.packages || typeof config.packages !== "object") {
      throw new Error("Config must have a 'packages' object");
    }

    if (!Array.isArray(config.packages.common)) {
      throw new Error("Config packages.common must be an array");
    }

    // Apply defaults
    return {
      ...config,
      templates: config.templates ?? {},
      ignore: config.ignore ?? [],
      backup: {
        ...DEFAULT_BACKUP_CONFIG,
        ...config.backup,
      },
    };
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "ERR_MODULE_NOT_FOUND") {
      logger.error(`Configuration file not found: ${configPath}`);
      logger.info("Please create a dotfiles.config.ts file in the repository root.");
      throw new Error("Configuration file not found");
    }
    throw error;
  }
}

/**
 * Get the path to the config directory
 */
export function getConfigDir(): string {
  return resolve(getRepoDir(), "config");
}

/**
 * Resolve a config file path relative to the config directory
 */
export function resolveConfigPath(relativePath: string): string {
  return resolve(getConfigDir(), relativePath);
}
