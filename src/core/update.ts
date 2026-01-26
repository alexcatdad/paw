/**
 * Self-Update Module
 * Handles version checking and binary updates using GitHub releases via gh CLI
 */

import { $ } from "bun";
import { logger } from "./logger";
import { getPlatform, getArch, getHomeDir, commandExists } from "./os";
import type { UpdateState, GitHubRelease, InstallOptions } from "../types";
import pkg from "../../package.json";

const REPO = "alexcatdad/paw";
const STATE_FILE = `${getHomeDir()}/.paw-state.json`;
const CHECK_INTERVAL_MS = 24 * 60 * 60 * 1000; // 24 hours

/**
 * Load update state from disk
 */
export async function loadUpdateState(): Promise<UpdateState | null> {
  try {
    const file = Bun.file(STATE_FILE);
    if (await file.exists()) {
      return await file.json();
    }
  } catch {
    // State file doesn't exist or is invalid
  }
  return null;
}

/**
 * Save update state to disk
 */
export async function saveUpdateState(state: UpdateState): Promise<void> {
  await Bun.write(STATE_FILE, JSON.stringify(state, null, 2));
}

/**
 * Check if we should skip the version check (rate limiting)
 */
export async function shouldSkipCheck(): Promise<boolean> {
  const state = await loadUpdateState();
  if (!state) return false;

  const lastCheck = new Date(state.lastCheck).getTime();
  const now = Date.now();
  return now - lastCheck < CHECK_INTERVAL_MS;
}

/**
 * Get the latest release from GitHub using gh CLI
 */
export async function getLatestRelease(): Promise<GitHubRelease | null> {
  // Check if gh is available
  if (!(await commandExists("gh"))) {
    logger.warn("gh CLI not found. Install with: brew install gh");
    return null;
  }

  try {
    const result = await $`gh release view latest --repo ${REPO} --json tagName,assets`.quiet().nothrow();

    if (result.exitCode !== 0) {
      // Could be offline, no releases, or not authenticated
      const stderr = result.stderr.toString();
      if (stderr.includes("release not found")) {
        logger.debug?.("No releases found");
      } else if (stderr.includes("authentication")) {
        logger.warn("gh not authenticated. Run: gh auth login");
      }
      return null;
    }

    const data = JSON.parse(result.text());
    return {
      tag_name: data.tagName,
      assets: data.assets.map((a: { name: string; url: string }) => ({
        name: a.name,
        browser_download_url: a.url,
      })),
    };
  } catch (error) {
    logger.debug?.(`Failed to fetch release: ${error}`);
    return null;
  }
}

/**
 * Compare two semantic versions
 * Returns: -1 if a < b, 0 if a == b, 1 if a > b
 */
export function compareVersions(a: string, b: string): number {
  // Strip leading 'v' if present
  const cleanA = a.replace(/^v/, "");
  const cleanB = b.replace(/^v/, "");

  const partsA = cleanA.split(".").map(Number);
  const partsB = cleanB.split(".").map(Number);

  for (let i = 0; i < Math.max(partsA.length, partsB.length); i++) {
    const numA = partsA[i] ?? 0;
    const numB = partsB[i] ?? 0;

    if (numA < numB) return -1;
    if (numA > numB) return 1;
  }

  return 0;
}

/**
 * Get the expected binary name for the current platform
 */
export function getBinaryName(): string {
  const platform = getPlatform();
  const arch = getArch();
  return `paw-${platform}-${arch}`;
}

/**
 * Get the path where paw is installed
 */
export async function getPawBinaryPath(): Promise<string | null> {
  try {
    const result = await $`which paw`.quiet().nothrow();
    if (result.exitCode === 0) {
      return result.text().trim();
    }
  } catch {
    // paw not in PATH
  }

  // Check default location
  const defaultPath = `${getHomeDir()}/.local/bin/paw`;
  const file = Bun.file(defaultPath);
  if (await file.exists()) {
    return defaultPath;
  }

  return null;
}

/**
 * Download the binary for the current platform
 */
export async function downloadBinary(targetDir: string, options: InstallOptions): Promise<string | null> {
  const binaryName = getBinaryName();

  if (options.dryRun) {
    logger.info(`Would download: ${binaryName} to ${targetDir}`);
    return `${targetDir}/${binaryName}`;
  }

  try {
    const result = await $`gh release download latest --repo ${REPO} --pattern ${binaryName} --dir ${targetDir}`.quiet().nothrow();

    if (result.exitCode !== 0) {
      const stderr = result.stderr.toString();
      logger.error(`Failed to download binary: ${stderr}`);
      return null;
    }

    return `${targetDir}/${binaryName}`;
  } catch (error) {
    logger.error(`Download failed: ${error}`);
    return null;
  }
}

/**
 * Perform the self-update
 */
export async function performUpdate(options: InstallOptions): Promise<boolean> {
  const currentVersion = pkg.version;
  const binaryPath = await getPawBinaryPath();

  if (!binaryPath) {
    logger.error("Could not find paw binary. Are you running from source?");
    logger.info("For source installs, update with: git pull && bun install");
    return false;
  }

  // Check if this is a compiled binary (not running from bun)
  const isCompiled = !binaryPath.endsWith(".ts") && !process.argv[0]?.includes("bun");
  if (!isCompiled && !options.force) {
    logger.warn("Running from source, not a compiled binary.");
    logger.info("To update: git pull && bun install");
    return false;
  }

  logger.info(`Current version: v${currentVersion}`);
  logger.info(`Binary location: ${binaryPath}`);

  // Fetch latest release
  const release = await getLatestRelease();
  if (!release) {
    logger.error("Could not fetch latest release");
    return false;
  }

  const latestVersion = release.tag_name.replace(/^v/, "");
  logger.info(`Latest version: v${latestVersion}`);

  // Compare versions
  const comparison = compareVersions(currentVersion, latestVersion);
  if (comparison >= 0) {
    logger.success("Already up to date!");
    await saveUpdateState({
      lastCheck: new Date().toISOString(),
      latestVersion,
      currentVersion,
    });
    return true;
  }

  logger.info(`Update available: v${currentVersion} → v${latestVersion}`);

  if (options.dryRun) {
    logger.info("Dry run - no changes made");
    return true;
  }

  // Check if we have the right binary in the release
  const binaryName = getBinaryName();
  const hasAsset = release.assets.some((a) => a.name === binaryName);
  if (!hasAsset) {
    logger.error(`No binary found for your platform: ${binaryName}`);
    logger.info(`Available: ${release.assets.map((a) => a.name).join(", ")}`);
    return false;
  }

  // Create temp directory for download
  const tmpDir = `/tmp/paw-update-${Date.now()}`;
  await $`mkdir -p ${tmpDir}`.quiet();

  try {
    // Download new binary
    logger.info("Downloading new binary...");
    const downloadedPath = await downloadBinary(tmpDir, options);
    if (!downloadedPath) {
      return false;
    }

    // Backup current binary
    const backupPath = `${binaryPath}.backup`;
    logger.info(`Backing up current binary to ${backupPath}`);
    await $`cp ${binaryPath} ${backupPath}`.quiet();

    // Install new binary
    logger.info("Installing new binary...");
    await $`mv ${downloadedPath} ${binaryPath}`.quiet();
    await $`chmod +x ${binaryPath}`.quiet();

    // Verify new binary works
    logger.info("Verifying installation...");
    const verifyResult = await $`${binaryPath} --version`.quiet().nothrow();

    if (verifyResult.exitCode !== 0) {
      // Restore backup
      logger.error("Verification failed, restoring backup...");
      await $`mv ${backupPath} ${binaryPath}`.quiet();
      return false;
    }

    // Remove backup
    await $`rm -f ${backupPath}`.quiet();

    // Update state
    await saveUpdateState({
      lastCheck: new Date().toISOString(),
      latestVersion,
      currentVersion: latestVersion,
    });

    logger.success(`Updated to v${latestVersion}!`);
    return true;
  } finally {
    // Cleanup temp directory
    await $`rm -rf ${tmpDir}`.quiet().nothrow();
  }
}

/**
 * Check for updates (used by sync command)
 * Returns the latest version if an update is available, null otherwise
 */
export async function checkForUpdate(options: { force?: boolean } = {}): Promise<string | null> {
  // Skip if we checked recently (unless forced)
  if (!options.force && (await shouldSkipCheck())) {
    return null;
  }

  const release = await getLatestRelease();
  if (!release) {
    return null;
  }

  const currentVersion = pkg.version;
  const latestVersion = release.tag_name.replace(/^v/, "");

  // Update state
  await saveUpdateState({
    lastCheck: new Date().toISOString(),
    latestVersion,
    currentVersion,
  });

  // Return latest version if update is available
  if (compareVersions(currentVersion, latestVersion) < 0) {
    return latestVersion;
  }

  return null;
}
