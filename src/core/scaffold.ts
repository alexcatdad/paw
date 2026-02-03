/**
 * Scaffold Module
 * Generates missing configs based on audit recommendations
 */

import { mkdir, writeFile, stat } from "node:fs/promises";
import { resolve, dirname } from "node:path";
import { logger } from "./logger";
import { COMMON_CONFIGS, type CommonConfig } from "./audit-patterns";
import { getPlatform } from "./os";

interface ScaffoldOptions {
  dryRun: boolean;
  force: boolean;
}

/**
 * Template content for common configs
 */
const TEMPLATES: Record<string, string> = {
  "shell/zshrc": `# Zsh Configuration
# Managed by paw dotfiles

# ─────────────────────────────────────────────────────────────────────────────
# Environment
# ─────────────────────────────────────────────────────────────────────────────
export EDITOR="vim"
export VISUAL="$EDITOR"

# ─────────────────────────────────────────────────────────────────────────────
# History
# ─────────────────────────────────────────────────────────────────────────────
HISTSIZE=10000
SAVEHIST=10000
HISTFILE=~/.zsh_history
setopt SHARE_HISTORY
setopt HIST_IGNORE_DUPS

# ─────────────────────────────────────────────────────────────────────────────
# Aliases
# ─────────────────────────────────────────────────────────────────────────────
alias ll="ls -la"
alias ..="cd .."
alias ...="cd ../.."

# ─────────────────────────────────────────────────────────────────────────────
# Load local config if exists
# ─────────────────────────────────────────────────────────────────────────────
[[ -f ~/.zshrc.local ]] && source ~/.zshrc.local
`,

  "git/gitconfig": `# Git Configuration
# Managed by paw dotfiles

[init]
    defaultBranch = main

[core]
    editor = vim
    excludesfile = ~/.gitignore_global

[alias]
    st = status
    co = checkout
    br = branch
    ci = commit
    lg = log --oneline --graph --decorate

[pull]
    rebase = false

[push]
    default = current

# Include local config for machine-specific settings
[include]
    path = ~/.gitconfig.local
`,

  "git/gitignore_global": `# Global gitignore
# Managed by paw dotfiles

# OS
.DS_Store
Thumbs.db

# Editors
*.swp
*.swo
*~
.idea/
.vscode/

# Environment
.env
.env.local
`,
};

/**
 * Get the recommended file path for a config
 */
function getRecommendedPath(config: CommonConfig): string {
  const fileName = config.fileNames[0];

  // Map common configs to organized structure
  if (fileName.includes("zsh") || fileName.includes("bash")) {
    return `shell/${fileName.replace(/^\./, "")}`;
  }
  if (fileName.includes("git")) {
    return `git/${fileName.replace(/^\./, "")}`;
  }
  if (fileName.includes("vim") || fileName.includes("nvim")) {
    return `vim/${fileName.replace(/^\./, "")}`;
  }

  // Default: use config directory
  return `config/${fileName.replace(/^\./, "")}`;
}

/**
 * Scaffold missing configs
 */
export async function scaffoldConfigs(
  repoPath: string,
  configNames: string[],
  options: ScaffoldOptions
): Promise<number> {
  const platform = getPlatform();
  let created = 0;

  for (const configName of configNames) {
    const config = COMMON_CONFIGS.find(c =>
      c.name.toLowerCase() === configName.toLowerCase()
    );

    if (!config) {
      logger.warn(`Unknown config: ${configName}`);
      continue;
    }

    // Skip if platform doesn't match
    if (config.platform && config.platform !== "all" && config.platform !== platform) {
      logger.skip(`${config.name} (not for ${platform})`);
      continue;
    }

    const filePath = getRecommendedPath(config);
    const fullPath = resolve(repoPath, filePath);

    // Check if already exists
    try {
      await stat(fullPath);
      if (!options.force) {
        logger.skip(`${filePath} (already exists)`);
        continue;
      }
    } catch {
      // Doesn't exist, we can create it
    }

    // Get template or create placeholder
    const template = TEMPLATES[filePath] || `# ${config.name}\n# ${config.description}\n# TODO: Add your configuration here\n`;

    if (options.dryRun) {
      logger.dryRun(`Would create ${filePath}`);
    } else {
      await mkdir(dirname(fullPath), { recursive: true });
      await writeFile(fullPath, template);
      logger.success(`Created ${filePath}`);
    }

    created++;
  }

  return created;
}

/**
 * List available configs to scaffold
 */
export function listAvailableConfigs(): void {
  const platform = getPlatform();

  logger.subheader("Essential (Priority 1)");
  for (const config of COMMON_CONFIGS.filter(c => c.priority === 1)) {
    if (config.platform && config.platform !== "all" && config.platform !== platform) continue;
    console.log(`  ${config.name.padEnd(20)} ${config.description}`);
  }

  logger.subheader("Recommended (Priority 2)");
  for (const config of COMMON_CONFIGS.filter(c => c.priority === 2)) {
    if (config.platform && config.platform !== "all" && config.platform !== platform) continue;
    console.log(`  ${config.name.padEnd(20)} ${config.description}`);
  }

  logger.subheader("Optional (Priority 3)");
  for (const config of COMMON_CONFIGS.filter(c => c.priority === 3)) {
    if (config.platform && config.platform !== "all" && config.platform !== platform) continue;
    console.log(`  ${config.name.padEnd(20)} ${config.description}`);
  }
}
