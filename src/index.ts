#!/usr/bin/env bun
/**
 * Dotfiles CLI
 * Personal paw manager with TypeScript/Bun
 */

import { parseArgs } from "util";
import { loadConfig } from "./core/config";
import { getPlatform, getSystemInfo, getHomeDir, getRepoDir, commandExists, contractPath } from "./core/os";
import { createSymlinks, removeSymlinks, getSymlinkStatus, statesToEntries } from "./core/symlinks";
import { generateTemplates } from "./core/templates";
import { installPackages, checkPackages } from "./core/packages";
import { listBackups, restoreBackup, cleanBackups, rollback, saveLastRunState, loadLastRunState } from "./core/backup";
import { addSuggestions, SSH_SUGGESTIONS } from "./core/suggestions";
import { logger } from "./core/logger";
import { performUpdate } from "./core/update";
import { runSync, printSyncSummary } from "./core/sync";
import type { InstallOptions, BackupEntry, SyncOptions } from "./types";
import pkg from "../package.json";

const VERSION = pkg.version;

/**
 * Print help message
 */
function printHelp(): void {
  console.log(`
${"\x1b[1m"}paw${"\x1b[0m"} v${VERSION} - Personal dotfiles manager 🐱

${"\x1b[1m"}USAGE${"\x1b[0m"}
  paw <command> [options]

${"\x1b[1m"}COMMANDS${"\x1b[0m"}
  install          Full setup: install packages and create symlinks
  link             Create symlinks only (skip package installation)
  unlink           Remove all managed symlinks
  status           Show current symlink and package status
  sync             Pull dotfiles repo and refresh links if needed
  update           Update the paw binary (self-update)
  rollback         Restore backups and remove symlinks from last run
  backup list      List all backup files
  backup restore   Restore a specific backup file
  backup clean     Remove old backups based on retention policy
  doctor           Check dotfiles health and diagnose issues

${"\x1b[1m"}OPTIONS${"\x1b[0m"}
  -n, --dry-run        Show what would be done without making changes
  -f, --force          Overwrite existing files (creates backups)
  -v, --verbose        Show detailed output
  --skip-packages      Skip package installation (install command only)
  -q, --quiet          Suppress output (sync command)
  --skip-update        Skip paw binary update check (sync command)
  --auto-update        Auto-update paw without prompting (sync command)
  -h, --help           Show this help message
  --version            Show version number

${"\x1b[1m"}EXAMPLES${"\x1b[0m"}
  paw install              # Full installation
  paw install --dry-run    # Preview installation
  paw link --force         # Force symlinks with backup
  paw status               # Check current state
  paw sync                 # Pull repo and refresh config
  paw sync --quiet         # Silent sync (for shell startup)
  paw update               # Update paw binary
  paw rollback             # Undo last install/link
  paw backup clean         # Clean old backups

${"\x1b[1m"}FIRST TIME SETUP${"\x1b[0m"}
  curl -fsSL https://raw.githubusercontent.com/alexcatdad/dotfiles/main/install.sh | bash
`);
}

/**
 * Print version
 */
function printVersion(): void {
  console.log(`paw v${VERSION}`);
}

/**
 * Install command - full setup
 */
async function installCommand(options: InstallOptions): Promise<void> {
  logger.header("Dotfiles Install");
  logger.table({
    "System": getSystemInfo(),
    "Home": getHomeDir(),
    "Repo": getRepoDir(),
    "Dry Run": options.dryRun ? "Yes" : "No",
  });
  logger.newline();

  const config = await loadConfig();
  const backups: BackupEntry[] = [];

  // Run pre-install hook
  if (config.hooks?.preInstall) {
    logger.subheader("Running pre-install hook");
    await config.hooks.preInstall({
      platform: getPlatform(),
      homeDir: getHomeDir(),
      repoDir: getRepoDir(),
      dryRun: options.dryRun,
      shell: async (cmd) => {
        const { $ } = await import("bun");
        await $`sh -c ${cmd}`;
      },
      commandExists,
    });
  }

  // Install packages
  if (!options.skipPackages) {
    logger.header("Installing Packages");
    const result = await installPackages(config.packages, options);
    logger.newline();
    logger.info(`Installed: ${result.installed.length}, Failed: ${result.failed.length}`);
  } else {
    logger.info("Skipping package installation (--skip-packages)");
  }

  // Create symlinks
  logger.header("Creating Symlinks");
  const states = await createSymlinks(config.symlinks, options);

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

  // Generate template files
  if (Object.keys(config.templates).length > 0) {
    logger.header("Generating Template Files");
    await generateTemplates(config.templates, options);
  }

  // Add best-practice suggestions to config files (non-destructive)
  logger.header("Config Suggestions");
  await addSuggestions(SSH_SUGGESTIONS, { dryRun: options.dryRun, force: options.force });

  // Run post-install hook
  if (config.hooks?.postInstall) {
    logger.subheader("Running post-install hook");
    await config.hooks.postInstall({
      platform: getPlatform(),
      homeDir: getHomeDir(),
      repoDir: getRepoDir(),
      dryRun: options.dryRun,
      shell: async (cmd) => {
        const { $ } = await import("bun");
        await $`sh -c ${cmd}`;
      },
      commandExists,
    });
  }

  // Save last run state (for rollback)
  if (!options.dryRun) {
    await saveLastRunState({
      timestamp: new Date().toISOString(),
      command: "install",
      backups,
      symlinks: statesToEntries(states),
    });
  }

  // Summary
  logger.header("Summary");
  const linked = states.filter(s => s.status === "linked").length;
  const conflicts = states.filter(s => s.status === "conflict").length;
  const missing = states.filter(s => s.status === "source-missing").length;

  logger.table({
    "Symlinks created": String(linked),
    "Conflicts": String(conflicts),
    "Missing sources": String(missing),
    "Backups created": String(backups.length),
  });

  logger.newline();
  if (options.dryRun) {
    logger.info("This was a dry run. No changes were made.");
  } else {
    logger.success("Installation complete! Restart your shell or run: source ~/.zshrc");
  }
}

/**
 * Link command - symlinks only
 */
async function linkCommand(options: InstallOptions): Promise<void> {
  logger.header("Creating Symlinks");

  const config = await loadConfig();
  const backups: BackupEntry[] = [];

  // Run pre-link hook
  if (config.hooks?.preLink) {
    await config.hooks.preLink({
      platform: getPlatform(),
      homeDir: getHomeDir(),
      repoDir: getRepoDir(),
      dryRun: options.dryRun,
      shell: async (cmd) => {
        const { $ } = await import("bun");
        await $`sh -c ${cmd}`;
      },
      commandExists,
    });
  }

  const states = await createSymlinks(config.symlinks, options);

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

  // Generate templates
  if (Object.keys(config.templates).length > 0) {
    logger.newline();
    logger.subheader("Generating Template Files");
    await generateTemplates(config.templates, options);
  }

  // Run post-link hook
  if (config.hooks?.postLink) {
    await config.hooks.postLink({
      platform: getPlatform(),
      homeDir: getHomeDir(),
      repoDir: getRepoDir(),
      dryRun: options.dryRun,
      shell: async (cmd) => {
        const { $ } = await import("bun");
        await $`sh -c ${cmd}`;
      },
      commandExists,
    });
  }

  // Save last run state
  if (!options.dryRun) {
    await saveLastRunState({
      timestamp: new Date().toISOString(),
      command: "link",
      backups,
      symlinks: statesToEntries(states),
    });
  }

  logger.newline();
  const linked = states.filter(s => s.status === "linked").length;
  logger.success(`Created ${linked} symlink(s)`);
}

/**
 * Unlink command - remove symlinks
 */
async function unlinkCommand(options: InstallOptions): Promise<void> {
  logger.header("Removing Symlinks");

  const config = await loadConfig();
  await removeSymlinks(config.symlinks, options);

  logger.newline();
  logger.success("Symlinks removed");
}

/**
 * Status command - show current state
 */
async function statusCommand(options: InstallOptions): Promise<void> {
  logger.header("Dotfiles Status");
  logger.table({
    "System": getSystemInfo(),
    "Home": getHomeDir(),
    "Repo": getRepoDir(),
  });

  const config = await loadConfig();

  // Check symlinks
  logger.subheader("Symlinks");
  const states = await getSymlinkStatus(config.symlinks);

  for (const state of states) {
    const icon = state.status === "linked" ? "\x1b[32m✓\x1b[0m" :
                 state.status === "conflict" ? "\x1b[33m⚠\x1b[0m" :
                 state.status === "source-missing" ? "\x1b[31m✗\x1b[0m" :
                 "\x1b[90m○\x1b[0m";
    const statusText = state.status === "linked" ? "linked" :
                       state.status === "conflict" ? "conflict" :
                       state.status === "source-missing" ? "source missing" :
                       "not linked";

    console.log(`  ${icon} ${contractPath(state.target)} (${statusText})`);
  }

  // Check packages
  logger.subheader("Packages");
  const { installed, missing } = await checkPackages(config.packages);

  if (missing.length > 0) {
    logger.warn(`Missing packages: ${missing.join(", ")}`);
  }
  logger.info(`Installed: ${installed.length}, Missing: ${missing.length}`);

  // Check last run
  const lastRun = await loadLastRunState();
  if (lastRun) {
    logger.subheader("Last Run");
    logger.table({
      "Command": lastRun.command,
      "Time": lastRun.timestamp,
      "Symlinks": String(lastRun.symlinks.length),
      "Backups": String(lastRun.backups.length),
    });
  }
}

/**
 * Backup command dispatcher
 */
async function backupCommand(subcommand: string, args: string[], options: InstallOptions): Promise<void> {
  const config = await loadConfig();

  switch (subcommand) {
    case "list":
      await listBackups();
      break;

    case "restore":
      if (args.length === 0) {
        logger.error("Usage: paw backup restore <backup-file>");
        process.exit(1);
      }
      await restoreBackup(args[0], options);
      break;

    case "clean":
      logger.header("Cleaning Backups");
      const removed = await cleanBackups(config.backup!, options);
      logger.newline();
      logger.info(`Removed ${removed} backup(s)`);
      break;

    default:
      logger.error(`Unknown backup subcommand: ${subcommand}`);
      logger.info("Available: list, restore, clean");
      process.exit(1);
  }
}

/**
 * Rollback command
 */
async function rollbackCommand(options: InstallOptions): Promise<void> {
  await rollback(options);
}

/**
 * Update command - self-update
 */
async function updateCommand(options: InstallOptions): Promise<void> {
  logger.header("Self Update");
  await performUpdate(options);
}

/**
 * Sync command - pull repo and refresh links
 */
async function syncCommand(options: SyncOptions): Promise<void> {
  if (!options.quiet) {
    logger.header("Dotfiles Sync");
  }

  const result = await runSync(options);

  if (!options.quiet) {
    printSyncSummary(result);
  }
}

/**
 * Doctor command - check paw health
 */
async function doctorCommand(options: InstallOptions): Promise<void> {
  logger.header("Dotfiles Doctor");

  const config = await loadConfig();
  let issues = 0;

  // System Info
  logger.subheader("System");
  logger.table({
    "Platform": getPlatform(),
    "Home": getHomeDir(),
    "Repo": getRepoDir(),
    "Shell": process.env.SHELL ?? "unknown",
  });

  // Check Symlinks
  logger.subheader("Symlinks");
  const symlinkStatus = await getSymlinkStatus(config.symlinks);
  const linked = symlinkStatus.filter(s => s.status === "linked").length;
  const conflicts = symlinkStatus.filter(s => s.status === "conflict");
  const missing = symlinkStatus.filter(s => s.status === "missing" || s.status === "source-missing");

  if (conflicts.length > 0) {
    logger.warn(`${conflicts.length} conflict(s):`);
    conflicts.forEach(s => logger.info(`  - ${contractPath(s.target)}`));
    issues += conflicts.length;
  }
  if (missing.length > 0) {
    logger.warn(`${missing.length} missing:`);
    missing.forEach(s => logger.info(`  - ${contractPath(s.target)}`));
    issues += missing.length;
  }
  logger.info(`${linked}/${symlinkStatus.length} symlinks active`);

  // Check Required Tools
  logger.subheader("Required Tools");
  const requiredTools = ["git", "zsh", "curl", "nano", "ssh", "tar", "gzip"];
  for (const tool of requiredTools) {
    if (await commandExists(tool)) {
      logger.success(tool);
    } else {
      logger.error(`${tool} - NOT FOUND`);
      issues++;
    }
  }

  // Check Optional Tools (from packages)
  logger.subheader("Optional Tools");
  const optionalTools = config.packages.common;
  const installedTools: string[] = [];
  const missingTools: string[] = [];

  for (const tool of optionalTools) {
    if (await commandExists(tool)) {
      installedTools.push(tool);
    } else {
      missingTools.push(tool);
    }
  }

  logger.info(`${installedTools.length}/${optionalTools.length} packages installed`);
  if (missingTools.length > 0 && options.verbose) {
    logger.warn(`Missing: ${missingTools.join(", ")}`);
  }

  // Check Shell Configuration
  logger.subheader("Shell Config");
  const homeDir = getHomeDir();
  const zshrcPath = `${homeDir}/.zshrc`;
  const zshrcFile = Bun.file(zshrcPath);

  if (await zshrcFile.exists()) {
    const zshrcContent = await zshrcFile.text();
    const checks = [
      { name: "Zinit", check: zshrcContent.includes("zinit") },
      { name: "Starship", check: zshrcContent.includes("starship") },
      { name: "Zoxide", check: zshrcContent.includes("zoxide") },
      { name: "FZF", check: zshrcContent.includes("fzf") },
    ];
    for (const { name, check } of checks) {
      if (check) {
        logger.success(name);
      } else {
        logger.warn(`${name} - not configured`);
      }
    }
  } else {
    logger.error(".zshrc not found");
    issues++;
  }

  // Check SSH config
  logger.subheader("SSH");
  const sshConfigPath = `${homeDir}/.ssh/config`;
  const sshSocketsPath = `${homeDir}/.ssh/sockets`;
  const sshConfigFile = Bun.file(sshConfigPath);

  if (await sshConfigFile.exists()) {
    logger.success("SSH config exists");
  } else {
    logger.info("SSH config not found (optional)");
  }

  // Check for SSH sockets directory (for connection multiplexing)
  const { existsSync } = await import("fs");
  if (!existsSync(sshSocketsPath)) {
    logger.info("SSH sockets directory missing - run: mkdir -p ~/.ssh/sockets");
  }

  // Summary
  logger.newline();
  if (issues === 0) {
    logger.success("All checks passed!");
  } else {
    logger.warn(`Found ${issues} issue(s). Run 'paw install --force' to fix.`);
  }
}

/**
 * Main entry point
 */
async function main(): Promise<void> {
  const { values, positionals } = parseArgs({
    args: Bun.argv.slice(2),
    options: {
      "dry-run": { type: "boolean", short: "n", default: false },
      "force": { type: "boolean", short: "f", default: false },
      "verbose": { type: "boolean", short: "v", default: false },
      "skip-packages": { type: "boolean", default: false },
      "quiet": { type: "boolean", short: "q", default: false },
      "skip-update": { type: "boolean", default: false },
      "auto-update": { type: "boolean", default: false },
      "help": { type: "boolean", short: "h", default: false },
      "version": { type: "boolean", default: false },
    },
    allowPositionals: true,
    strict: false,
  });

  // Handle --help and --version
  if (values.help) {
    printHelp();
    process.exit(0);
  }

  if (values.version) {
    printVersion();
    process.exit(0);
  }

  const command = positionals[0] ?? "status";
  const subArgs = positionals.slice(1);

  const options: InstallOptions = {
    dryRun: values["dry-run"] as boolean,
    force: values.force as boolean,
    verbose: values.verbose as boolean,
    skipPackages: values["skip-packages"] as boolean,
  };

  try {
    switch (command) {
      case "install":
        await installCommand(options);
        break;

      case "link":
        await linkCommand(options);
        break;

      case "unlink":
        await unlinkCommand(options);
        break;

      case "status":
        await statusCommand(options);
        break;

      case "rollback":
        await rollbackCommand(options);
        break;

      case "backup":
        await backupCommand(subArgs[0] ?? "list", subArgs.slice(1), options);
        break;

      case "sync": {
        const syncOptions: SyncOptions = {
          ...options,
          quiet: values.quiet as boolean,
          skipUpdate: values["skip-update"] as boolean,
          autoUpdate: values["auto-update"] as boolean,
        };
        await syncCommand(syncOptions);
        break;
      }

      case "update":
        await updateCommand(options);
        break;

      case "doctor":
        await doctorCommand(options);
        break;

      case "help":
        printHelp();
        break;

      default:
        logger.error(`Unknown command: ${command}`);
        printHelp();
        process.exit(1);
    }
  } catch (error) {
    logger.error(`Command failed: ${error}`);
    if (options.verbose) {
      console.error(error);
    }
    process.exit(1);
  }
}

main();
