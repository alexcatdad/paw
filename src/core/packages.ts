/**
 * Package Installation
 * Handles installing packages via Homebrew (macOS) and apt/Linuxbrew (Linux)
 */

import { $ } from "bun";
import type { PackageConfig, Platform, InstallOptions } from "../types";
import { detectPackageManagers, commandExists, getPlatform } from "./os";
import { logger } from "./logger";

/**
 * Install Homebrew if not present
 */
async function installHomebrew(platform: Platform, options: InstallOptions): Promise<string> {
  if (options.dryRun) {
    logger.dryRun("Would install Homebrew");
    return platform === "darwin" ? "/opt/homebrew/bin/brew" : "/home/linuxbrew/.linuxbrew/bin/brew";
  }

  logger.info("Homebrew not found. Installing...");
  logger.info("This may take a few minutes and require your password.");

  try {
    // The official Homebrew install script
    await $`/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"`;
  } catch (error) {
    logger.error(`Failed to install Homebrew: ${error}`);
    throw new Error("Homebrew installation failed");
  }

  // Return the expected path based on platform
  if (platform === "darwin") {
    // Check both Apple Silicon and Intel paths
    const armPath = "/opt/homebrew/bin/brew";
    const intelPath = "/usr/local/bin/brew";

    if (await Bun.file(armPath).exists()) return armPath;
    if (await Bun.file(intelPath).exists()) return intelPath;

    throw new Error("Homebrew installed but brew not found in expected locations");
  } else {
    return "/home/linuxbrew/.linuxbrew/bin/brew";
  }
}

/**
 * Check if a font is installed on macOS (regardless of how it was installed)
 */
async function isFontInstalled(fontName: string): Promise<boolean> {
  // Extract the font family name from cask name (e.g., "font-fira-code-nerd-font" -> "fira")
  const searchTerms = fontName
    .replace(/^font-/, "")
    .replace(/-nerd-font$/, "")
    .replace(/-/g, " ");

  try {
    // Use system_profiler to check installed fonts (more reliable than fc-list on macOS)
    const result = await $`system_profiler SPFontsDataType`.quiet().nothrow();
    if (result.exitCode === 0) {
      const output = result.stdout.toString().toLowerCase();
      // Check if any variant of the font name is present
      const terms = searchTerms.toLowerCase().split(" ");
      return terms.every(term => output.includes(term)) && output.includes("nerd");
    }
  } catch {
    // Fall through to false
  }

  return false;
}

/**
 * Check if a Homebrew package is installed (formula or cask)
 */
async function isBrewPackageInstalled(pkg: string, brewPath: string): Promise<boolean> {
  try {
    // Special handling for fonts - check if font is actually installed on system
    if (pkg.startsWith("font-") && pkg.includes("nerd-font")) {
      if (await isFontInstalled(pkg)) {
        return true;
      }
    }

    // Check formulae first
    const formulaResult = await $`${brewPath} list --formula ${pkg}`.quiet().nothrow();
    if (formulaResult.exitCode === 0) {
      return true;
    }

    // Check casks (for GUI apps, fonts, etc.)
    const caskResult = await $`${brewPath} list --cask ${pkg}`.quiet().nothrow();
    if (caskResult.exitCode === 0) {
      return true;
    }

    return false;
  } catch {
    return false;
  }
}

/**
 * Install a package via Homebrew
 */
async function installBrewPackage(
  pkg: string,
  brewPath: string,
  options: InstallOptions
): Promise<boolean> {
  // Check if already installed
  if (await isBrewPackageInstalled(pkg, brewPath)) {
    logger.skip(`${pkg} (already installed)`);
    return true;
  }

  if (options.dryRun) {
    logger.dryRun(`Would install: ${pkg}`);
    return true;
  }

  logger.package(`Installing ${pkg}...`);

  try {
    // Try regular formula first
    const result = await $`${brewPath} install ${pkg}`.quiet().nothrow();

    if (result.exitCode === 0) {
      logger.success(`Installed ${pkg}`);
      return true;
    }

    // Try as a cask (for GUI applications on macOS)
    const caskResult = await $`${brewPath} install --cask ${pkg}`.quiet().nothrow();

    if (caskResult.exitCode === 0) {
      logger.success(`Installed ${pkg} (cask)`);
      return true;
    }

    logger.error(`Failed to install ${pkg}`);
    return false;
  } catch (error) {
    logger.error(`Failed to install ${pkg}: ${error}`);
    return false;
  }
}

/**
 * Install a package via apt
 */
async function installAptPackage(
  pkg: string,
  options: InstallOptions
): Promise<boolean> {
  // Check if already installed
  try {
    const result = await $`dpkg -s ${pkg}`.quiet().nothrow();
    if (result.exitCode === 0) {
      logger.skip(`${pkg} (already installed)`);
      return true;
    }
  } catch {
    // Not installed, continue
  }

  if (options.dryRun) {
    logger.dryRun(`Would install via apt: ${pkg}`);
    return true;
  }

  logger.package(`Installing ${pkg} via apt...`);

  try {
    await $`sudo apt install -y ${pkg}`.quiet();
    logger.success(`Installed ${pkg}`);
    return true;
  } catch (error) {
    logger.error(`Failed to install ${pkg}: ${error}`);
    return false;
  }
}

/**
 * Ensure Homebrew is available, installing if necessary
 */
async function ensureHomebrew(
  platform: Platform,
  options: InstallOptions
): Promise<string> {
  const { hasBrew, brewPath } = await detectPackageManagers();

  if (hasBrew && brewPath) {
    logger.debug(`Found Homebrew at: ${brewPath}`, options.verbose);
    return brewPath;
  }

  return await installHomebrew(platform, options);
}

/**
 * Install all configured packages
 */
export async function installPackages(
  config: PackageConfig,
  options: InstallOptions
): Promise<{ installed: string[]; failed: string[]; skipped: string[] }> {
  const platform = getPlatform();
  const { hasApt } = await detectPackageManagers();

  const installed: string[] = [];
  const failed: string[] = [];
  const skipped: string[] = [];

  // On Linux, install apt prerequisites first
  if (platform === "linux" && hasApt && config.linux?.apt?.length) {
    logger.subheader("Installing apt prerequisites");

    for (const pkg of config.linux.apt) {
      const success = await installAptPackage(pkg, options);
      if (success) {
        installed.push(pkg);
      } else {
        failed.push(pkg);
      }
    }
  }

  // Ensure Homebrew is available
  logger.subheader("Setting up Homebrew");
  const brewPath = await ensureHomebrew(platform, options);

  // Update Homebrew (skip in dry run)
  if (!options.dryRun) {
    logger.info("Updating Homebrew...");
    try {
      await $`${brewPath} update`.quiet();
    } catch {
      logger.warn("Failed to update Homebrew, continuing anyway...");
    }
  }

  // Install common packages
  logger.subheader("Installing common packages");
  for (const pkg of config.common) {
    const success = await installBrewPackage(pkg, brewPath, options);
    if (success) {
      installed.push(pkg);
    } else {
      failed.push(pkg);
    }
  }

  // Install platform-specific packages
  if (platform === "darwin" && config.darwin?.length) {
    logger.subheader("Installing macOS packages");
    for (const pkg of config.darwin) {
      const success = await installBrewPackage(pkg, brewPath, options);
      if (success) {
        installed.push(pkg);
      } else {
        failed.push(pkg);
      }
    }
  }

  if (platform === "linux" && config.linux?.brew?.length) {
    logger.subheader("Installing Linux packages");
    for (const pkg of config.linux.brew) {
      const success = await installBrewPackage(pkg, brewPath, options);
      if (success) {
        installed.push(pkg);
      } else {
        failed.push(pkg);
      }
    }
  }

  // Install Nerd Fonts on Linux (not available via Homebrew casks)
  if (platform === "linux") {
    logger.subheader("Installing fonts");
    await installLinuxFonts(options);
  }

  return { installed, failed, skipped };
}

/**
 * Install Nerd Fonts on Linux (not available via Homebrew casks)
 */
async function installLinuxFonts(options: InstallOptions): Promise<void> {
  const fontDir = `${process.env.HOME}/.local/share/fonts`;
  const fontName = "FiraCode";
  const fontZip = `${fontName}.zip`;
  const fontUrl = `https://github.com/ryanoasis/nerd-fonts/releases/latest/download/${fontZip}`;

  // Check if font already exists
  try {
    const result = await $`fc-list`.quiet().nothrow();
    if (result.exitCode === 0 && result.stdout.toString().toLowerCase().includes("firacode nerd")) {
      logger.skip("FiraCode Nerd Font (already installed)");
      return;
    }
  } catch {
    // fc-list not available or font not found, continue with install
  }

  if (options.dryRun) {
    logger.dryRun("Would install FiraCode Nerd Font");
    return;
  }

  logger.package("Installing FiraCode Nerd Font...");

  try {
    // Create font directory
    await $`mkdir -p ${fontDir}`;

    // Download and extract font
    const tempDir = `/tmp/nerd-fonts-${Date.now()}`;
    await $`mkdir -p ${tempDir}`;
    await $`curl -fsSL ${fontUrl} -o ${tempDir}/${fontZip}`;
    await $`unzip -q ${tempDir}/${fontZip} -d ${tempDir}`;

    // Copy font files (only .ttf files)
    // Use shell to handle the glob properly
    await $`sh -c 'cp ${tempDir}/*.ttf ${fontDir}/ 2>/dev/null || true'`;

    // Refresh font cache
    await $`fc-cache -f ${fontDir}`.quiet().nothrow();

    // Cleanup
    await $`rm -rf ${tempDir}`;

    logger.success("Installed FiraCode Nerd Font");
  } catch (error) {
    logger.warn(`Failed to install FiraCode Nerd Font: ${error}`);
    logger.info("You can manually install from: https://www.nerdfonts.com/font-downloads");
  }
}

/**
 * Check which packages are installed
 */
export async function checkPackages(
  config: PackageConfig
): Promise<{ installed: string[]; missing: string[] }> {
  const installed: string[] = [];
  const missing: string[] = [];

  // Check common packages
  for (const pkg of config.common) {
    if (await commandExists(pkg)) {
      installed.push(pkg);
    } else {
      missing.push(pkg);
    }
  }

  // Check platform-specific packages
  const platform = getPlatform();

  if (platform === "darwin" && config.darwin) {
    for (const pkg of config.darwin) {
      if (await commandExists(pkg)) {
        installed.push(pkg);
      } else {
        missing.push(pkg);
      }
    }
  }

  if (platform === "linux") {
    if (config.linux?.brew) {
      for (const pkg of config.linux.brew) {
        if (await commandExists(pkg)) {
          installed.push(pkg);
        } else {
          missing.push(pkg);
        }
      }
    }
  }

  return { installed, missing };
}
