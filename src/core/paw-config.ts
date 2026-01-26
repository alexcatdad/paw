/**
 * Paw Configuration Management
 * Handles ~/.config/paw/config.json for storing dotfiles repo location
 */

import { getHomeDir } from "./os";
import type { PawConfig } from "../types";

const CONFIG_DIR = `${getHomeDir()}/.config/paw`;
const CONFIG_FILE = `${CONFIG_DIR}/config.json`;

/**
 * Load paw configuration
 */
export async function loadPawConfig(): Promise<PawConfig | null> {
  try {
    const file = Bun.file(CONFIG_FILE);
    if (await file.exists()) {
      return await file.json();
    }
  } catch {
    // Config doesn't exist or is invalid
  }
  return null;
}

/**
 * Save paw configuration
 */
export async function savePawConfig(config: PawConfig): Promise<void> {
  const { mkdirSync } = await import("fs");
  mkdirSync(CONFIG_DIR, { recursive: true });
  await Bun.write(CONFIG_FILE, JSON.stringify(config, null, 2));
}

/**
 * Get the dotfiles repo path, checking paw config first, then fallback to getRepoDir()
 */
export async function getDotfilesPath(): Promise<string> {
  const config = await loadPawConfig();
  if (config?.dotfilesRepo) {
    // Expand ~ to home directory
    if (config.dotfilesRepo.startsWith("~/")) {
      return config.dotfilesRepo.replace("~", getHomeDir());
    }
    return config.dotfilesRepo;
  }
  // Fallback to legacy detection
  const { getRepoDir } = await import("./os");
  return getRepoDir();
}
