/**
 * OS Detection and Package Manager Utilities
 * Handles cross-platform detection and package manager discovery
 */

import { $ } from "bun";
import type { Platform } from "../types";

/**
 * Get the current platform
 */
export function getPlatform(): Platform {
  const platform = process.platform;
  if (platform === "darwin") return "darwin";
  if (platform === "linux") return "linux";
  throw new Error(`Unsupported platform: ${platform}. Only macOS and Linux are supported.`);
}

/**
 * Check if a command exists on the system
 */
export async function commandExists(cmd: string): Promise<boolean> {
  try {
    const result = await $`which ${cmd}`.quiet().nothrow();
    return result.exitCode === 0;
  } catch {
    return false;
  }
}

/**
 * Get the home directory
 */
export function getHomeDir(): string {
  const home = process.env.HOME ?? Bun.env.HOME;
  if (!home) {
    throw new Error("Could not determine home directory. HOME environment variable not set.");
  }
  return home;
}

/**
 * Get the repository directory (where the dotfiles config lives)
 * Supports: environment variable, current dir, common locations, or import.meta.dir (dev mode)
 */
export function getRepoDir(): string {
  // 1. Check environment variable first (allows custom location)
  const envDir = process.env.PAW_REPO ?? process.env.DOTFILES_DIR;
  if (envDir) {
    return envDir;
  }

  // 2. Check current working directory (for CI and running from repo)
  const cwd = process.cwd();
  try {
    if (Bun.file(`${cwd}/dotfiles.config.ts`).size !== undefined) {
      return cwd;
    }
  } catch {
    // Not in repo directory
  }

  // 3. Check common dotfiles locations
  const home = getHomeDir();
  const commonPaths = [
    `${home}/Projects/dotfiles`,
    `${home}/projects/dotfiles`,
    `${home}/.dotfiles`,
    `${home}/dotfiles`,
    `${home}/.config/dotfiles`,
  ];

  for (const path of commonPaths) {
    try {
      // Check if this looks like our dotfiles repo (has dotfiles.config.ts)
      if (Bun.file(`${path}/dotfiles.config.ts`).size !== undefined) {
        return path;
      }
    } catch {
      // Path doesn't exist or isn't accessible, continue
    }
  }

  // 4. Fallback to import.meta.dir (works in dev mode when running from source)
  const metaDir = import.meta.dir;
  if (!metaDir.includes("$bunfs") && !metaDir.includes("bunfs")) {
    return metaDir.replace(/\/src\/core$/, "");
  }

  // 5. Last resort - assume first common path
  return commonPaths[0];
}

/**
 * Standard Homebrew paths to check
 */
const BREW_PATHS = {
  darwin: [
    "/opt/homebrew/bin/brew",      // Apple Silicon
    "/usr/local/bin/brew",          // Intel Mac
  ],
  linux: [
    "/home/linuxbrew/.linuxbrew/bin/brew",
    `${process.env.HOME}/.linuxbrew/bin/brew`,
  ],
} as const;

/**
 * Find the Homebrew executable path
 */
export async function findBrewPath(): Promise<string | null> {
  // First, try the which command
  try {
    const result = await $`which brew`.quiet().nothrow();
    if (result.exitCode === 0) {
      return result.text().trim();
    }
  } catch {
    // Continue to path checking
  }

  // Check standard paths based on platform
  const platform = getPlatform();
  const pathsToCheck = BREW_PATHS[platform];

  for (const brewPath of pathsToCheck) {
    try {
      const file = Bun.file(brewPath);
      if (await file.exists()) {
        return brewPath;
      }
    } catch {
      // Path doesn't exist, continue
    }
  }

  return null;
}

export interface PackageManagerInfo {
  /** Whether Homebrew is available */
  hasBrew: boolean;
  /** Path to the brew executable */
  brewPath: string | null;
  /** Whether apt is available (Linux only) */
  hasApt: boolean;
}

/**
 * Detect available package managers
 */
export async function detectPackageManagers(): Promise<PackageManagerInfo> {
  const [brewPath, hasApt] = await Promise.all([
    findBrewPath(),
    commandExists("apt"),
  ]);

  return {
    hasBrew: brewPath !== null,
    brewPath,
    hasApt,
  };
}

/**
 * Get the CPU architecture
 */
export function getArch(): string {
  const arch = process.arch;
  switch (arch) {
    case "arm64":
      return "arm64";
    case "x64":
      return "x64";
    default:
      return arch;
  }
}

/**
 * Get a human-readable system description
 */
export function getSystemInfo(): string {
  const platform = getPlatform();
  const arch = getArch();
  const platformName = platform === "darwin" ? "macOS" : "Linux";
  return `${platformName} (${arch})`;
}

/**
 * Expand ~ to the home directory in a path
 */
export function expandPath(path: string): string {
  if (path.startsWith("~/")) {
    return path.replace("~", getHomeDir());
  }
  return path;
}

/**
 * Contract the home directory to ~ in a path (for display)
 */
export function contractPath(path: string): string {
  const home = getHomeDir();
  if (path.startsWith(home)) {
    return path.replace(home, "~");
  }
  return path;
}

/**
 * Validate that a resolved path is within an allowed base directory.
 * Prevents path traversal attacks (e.g., "../../../etc/passwd").
 * @throws Error if path escapes the allowed directory
 */
export function validatePathWithinBase(resolvedPath: string, allowedBase: string, context: string): void {
  // Normalize both paths to handle trailing slashes
  const normalizedPath = resolvedPath.endsWith("/") ? resolvedPath.slice(0, -1) : resolvedPath;
  const normalizedBase = allowedBase.endsWith("/") ? allowedBase.slice(0, -1) : allowedBase;

  // Path must either equal the base or be a subdirectory of it
  if (normalizedPath !== normalizedBase && !normalizedPath.startsWith(normalizedBase + "/")) {
    throw new Error(`Security error: ${context} path escapes allowed directory.\n  Path: ${resolvedPath}\n  Allowed: ${allowedBase}`);
  }
}

/**
 * Validate that a package name is safe (alphanumeric, hyphens, underscores, slashes, @)
 * Prevents command injection via malicious package names.
 */
export function isValidPackageName(name: string): boolean {
  // Allow: letters, numbers, hyphens, underscores, slashes (for casks), @ (for scoped packages)
  return /^[@a-zA-Z0-9_\/-]+$/.test(name) && name.length > 0 && name.length < 256;
}

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
