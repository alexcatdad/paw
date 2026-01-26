/**
 * Config Suggestions
 * Instead of symlinking/overwriting, prepend best practice comments to existing configs
 */

import { readFile, writeFile, stat } from "node:fs/promises";
import { resolve, dirname } from "node:path";
import { mkdir } from "node:fs/promises";
import { getHomeDir, contractPath } from "./os";
import { logger } from "./logger";

const SUGGESTION_MARKER = "# ═══ paw suggestions";
const SUGGESTION_END = "# ═══ end paw suggestions";

interface SuggestionConfig {
  /** Target file path relative to home */
  target: string;
  /** Comment character(s) for this file type */
  comment: string;
  /** Suggested content (will be commented out) */
  suggestions: string;
  /** Header text explaining the suggestions */
  header?: string;
}

/**
 * Check if file already has our suggestions
 */
async function hasSuggestions(filePath: string): Promise<boolean> {
  try {
    const content = await readFile(filePath, "utf-8");
    return content.includes(SUGGESTION_MARKER);
  } catch {
    return false;
  }
}

/**
 * Build the suggestion block
 */
function buildSuggestionBlock(config: SuggestionConfig): string {
  const c = config.comment;
  const lines: string[] = [];

  lines.push(`${c} ═══ paw suggestions ═══════════════════════════════════════════════════════`);
  lines.push(`${c} Uncomment the settings you want to use. Managed by paw dotfiles.`);
  lines.push(`${c} To update: paw install --force (re-adds this block if removed)`);
  lines.push(`${c} ─────────────────────────────────────────────────────────────────────────────`);

  if (config.header) {
    lines.push(`${c}`);
    for (const headerLine of config.header.split("\n")) {
      lines.push(`${c} ${headerLine}`);
    }
  }

  lines.push(`${c}`);

  // Add suggestions as comments
  for (const line of config.suggestions.split("\n")) {
    if (line.trim() === "") {
      lines.push(`${c}`);
    } else {
      // Prefix with comment char (user uncomments what they want)
      lines.push(`${c} ${line}`);
    }
  }

  lines.push(`${c}`);
  lines.push(`${c} ═══ end paw suggestions ════════════════════════════════════════════════════`);
  lines.push("");

  return lines.join("\n");
}

/**
 * Add suggestions to a config file
 */
export async function addSuggestions(
  config: SuggestionConfig,
  options: { dryRun?: boolean; force?: boolean }
): Promise<boolean> {
  const homeDir = getHomeDir();
  const filePath = resolve(homeDir, config.target);
  const displayPath = contractPath(filePath);

  // Check if already has suggestions
  if (!options.force && await hasSuggestions(filePath)) {
    logger.skip(`${displayPath} (suggestions already present)`);
    return false;
  }

  const suggestionBlock = buildSuggestionBlock(config);

  // Check if file exists
  let existingContent = "";
  let fileExists = false;
  try {
    await stat(filePath);
    existingContent = await readFile(filePath, "utf-8");
    fileExists = true;

    // Remove old suggestions if forcing
    if (options.force && existingContent.includes(SUGGESTION_MARKER)) {
      const startIdx = existingContent.indexOf(SUGGESTION_MARKER);
      const endMarker = SUGGESTION_END;
      const endIdx = existingContent.indexOf(endMarker);
      if (startIdx !== -1 && endIdx !== -1) {
        // Find the end of the end marker line
        let endOfLine = existingContent.indexOf("\n", endIdx);
        if (endOfLine === -1) endOfLine = existingContent.length;
        // Remove old block (including trailing newlines)
        let removeEnd = endOfLine + 1;
        while (removeEnd < existingContent.length && existingContent[removeEnd] === "\n") {
          removeEnd++;
        }
        existingContent = existingContent.slice(0, startIdx) + existingContent.slice(removeEnd);
      }
    }
  } catch {
    // File doesn't exist
  }

  if (options.dryRun) {
    if (fileExists) {
      logger.dryRun(`Would prepend suggestions to ${displayPath}`);
    } else {
      logger.dryRun(`Would create ${displayPath} with suggestions`);
    }
    return true;
  }

  // Ensure directory exists
  await mkdir(dirname(filePath), { recursive: true });

  // Prepend suggestions to existing content (or create new file)
  const newContent = suggestionBlock + existingContent;
  await writeFile(filePath, newContent);

  if (fileExists) {
    logger.success(`Added suggestions to ${displayPath}`);
  } else {
    logger.success(`Created ${displayPath} with suggestions`);
  }

  return true;
}

/**
 * SSH config suggestions
 */
export const SSH_SUGGESTIONS: SuggestionConfig = {
  target: ".ssh/config",
  comment: "#",
  header: `Best practices for SSH configuration.
See: https://www.ssh.com/academy/ssh/config`,
  suggestions: `
─────────────────────────────────────────────────────────────────────────────
Global Defaults
─────────────────────────────────────────────────────────────────────────────
Host *
  AddKeysToAgent yes
  IdentitiesOnly yes
  UseKeychain yes                    # macOS only
  ServerAliveInterval 60
  ServerAliveCountMax 3

  # Connection multiplexing (faster subsequent connections)
  ControlMaster auto
  ControlPath ~/.ssh/sockets/%r@%h-%p
  ControlPersist 600

─────────────────────────────────────────────────────────────────────────────
GitHub
─────────────────────────────────────────────────────────────────────────────
Host github.com
  HostName github.com
  User git
  IdentityFile ~/.ssh/id_ed25519

─────────────────────────────────────────────────────────────────────────────
Example: Server with jump host
─────────────────────────────────────────────────────────────────────────────
Host production
  HostName 10.0.0.50
  User deploy
  ProxyJump bastion.example.com
  IdentityFile ~/.ssh/deploy_key
`.trim(),
};

/**
 * Git config suggestions (for gitconfig.local)
 */
export const GIT_LOCAL_SUGGESTIONS: SuggestionConfig = {
  target: ".gitconfig.local",
  comment: "#",
  header: `Machine-specific git configuration.
This file is included by ~/.gitconfig`,
  suggestions: `
[user]
    name = Your Name
    email = your.email@example.com

# ─────────────────────────────────────────────────────────────────────────────
# Credential Helpers
# ─────────────────────────────────────────────────────────────────────────────
# macOS Keychain:
# [credential]
#     helper = osxkeychain

# GitHub CLI (recommended):
# [credential "https://github.com"]
#     helper =
#     helper = !/opt/homebrew/bin/gh auth git-credential

# ─────────────────────────────────────────────────────────────────────────────
# Commit Signing
# ─────────────────────────────────────────────────────────────────────────────
# [commit]
#     gpgsign = true
# [user]
#     signingkey = YOUR_GPG_KEY_ID
`.trim(),
};
