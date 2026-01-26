/**
 * Dotfiles Type Definitions
 */

export type Platform = "darwin" | "linux";

export interface PackageConfig {
  /** Packages to install on all platforms via Homebrew */
  common: string[];
  /** macOS-specific packages (Homebrew) */
  darwin?: string[];
  /** Linux-specific packages */
  linux?: {
    /** Packages to install via apt (prerequisites) */
    apt?: string[];
    /** Packages to install via Linuxbrew */
    brew?: string[];
  };
}

export interface BackupConfig {
  /** Whether to create backups when overwriting files */
  enabled: boolean;
  /** Maximum age of backups in days */
  maxAge: number;
  /** Maximum number of backups per file */
  maxCount: number;
}

export interface HookContext {
  platform: Platform;
  homeDir: string;
  repoDir: string;
  dryRun: boolean;
  shell: (cmd: string) => Promise<void>;
  commandExists: (cmd: string) => Promise<boolean>;
}

export interface DotfilesConfig {
  /** Symlink mappings: source (relative to config/) -> target (relative to $HOME) */
  symlinks: Record<string, string>;
  /** Packages to install by platform */
  packages: PackageConfig;
  /** Template files for machine-specific config: source -> target */
  templates: Record<string, string>;
  /** Files/patterns to never overwrite (gitignored, machine-specific) */
  ignore: string[];
  /** Backup configuration */
  backup?: BackupConfig;
  /** Lifecycle hooks */
  hooks?: {
    preInstall?: (ctx: HookContext) => Promise<void>;
    postInstall?: (ctx: HookContext) => Promise<void>;
    preLink?: (ctx: HookContext) => Promise<void>;
    postLink?: (ctx: HookContext) => Promise<void>;
  };
}

export type SymlinkStatus = "linked" | "missing" | "conflict" | "backup" | "source-missing";

export interface SymlinkState {
  /** Source file path (in repo) */
  source: string;
  /** Target file path (in home directory) */
  target: string;
  /** Current status of the symlink */
  status: SymlinkStatus;
  /** Path to backup file if one was created */
  backupPath?: string;
}

export interface InstallOptions {
  /** Show what would be done without making changes */
  dryRun: boolean;
  /** Overwrite existing files (creates backups) */
  force: boolean;
  /** Show detailed output */
  verbose: boolean;
  /** Skip package installation */
  skipPackages: boolean;
}

export interface BackupEntry {
  /** Original file path */
  original: string;
  /** Backup file path */
  backup: string;
  /** Timestamp of backup creation */
  timestamp: number;
}

export interface SymlinkEntry {
  /** Source file path (in repo) */
  source: string;
  /** Target file path (in home directory) */
  target: string;
}

export interface LastRunState {
  /** ISO timestamp of last run */
  timestamp: string;
  /** Command that was run */
  command: string;
  /** Backups created during this run */
  backups: BackupEntry[];
  /** Symlinks created during this run */
  symlinks: SymlinkEntry[];
}

// ============================================================================
// Update & Sync Types
// ============================================================================

export interface UpdateState {
  /** ISO timestamp of last version check */
  lastCheck: string;
  /** Latest version found on GitHub */
  latestVersion: string;
  /** Current installed version */
  currentVersion: string;
}

export interface GitHubRelease {
  /** Release tag name (e.g., "v1.2.0") */
  tag_name: string;
  /** Release assets (binaries, etc.) */
  assets: GitHubAsset[];
}

export interface GitHubAsset {
  /** Asset filename */
  name: string;
  /** Download URL */
  browser_download_url: string;
}

export interface SyncResult {
  /** Whether paw binary was updated */
  pawUpdated: boolean;
  /** Whether dotfiles repo was updated (git pull) */
  repoUpdated: boolean;
  /** Whether symlinks were refreshed */
  linksRefreshed: boolean;
}

export interface SyncOptions extends InstallOptions {
  /** Suppress output (for background sync) */
  quiet: boolean;
  /** Skip paw binary update check */
  skipUpdate: boolean;
  /** Automatically update paw without prompting */
  autoUpdate: boolean;
}
