/**
 * Repository Audit Module
 * Analyzes dotfiles repository structure, conventions, and completeness
 */

import { readdir, stat } from "node:fs/promises";
import { resolve, relative } from "node:path";
import type { AuditFinding, AuditResult, AuditOptions, DotfilesConfig } from "../types";
import { COMMON_CONFIGS, type CommonConfig } from "./audit-patterns";
import { getPlatform } from "./os";
import { logger } from "./logger";

/**
 * Recursively get all files in a directory
 */
async function getAllFiles(dir: string, baseDir: string = dir): Promise<string[]> {
  const files: string[] = [];

  try {
    const entries = await readdir(dir, { withFileTypes: true });

    for (const entry of entries) {
      const fullPath = resolve(dir, entry.name);
      const relativePath = relative(baseDir, fullPath);

      // Skip common non-dotfile directories
      if (entry.isDirectory()) {
        if (["node_modules", ".git", "dist", "build"].includes(entry.name)) {
          continue;
        }
        const subFiles = await getAllFiles(fullPath, baseDir);
        files.push(...subFiles);
      } else {
        files.push(relativePath);
      }
    }
  } catch {
    // Directory doesn't exist or can't be read
  }

  return files;
}

/**
 * Check if any of the file patterns exist in the file list
 */
function hasConfig(files: string[], config: CommonConfig): boolean {
  return config.fileNames.some(pattern => {
    // Normalize pattern for comparison
    const normalizedPattern = pattern.replace(/^\./, "");
    return files.some(file => {
      const normalizedFile = file.replace(/^\./, "");
      return normalizedFile === normalizedPattern ||
             normalizedFile.endsWith(`/${normalizedPattern}`) ||
             file === pattern ||
             file.endsWith(`/${pattern}`);
    });
  });
}

/**
 * Detect naming convention being used
 */
function detectNamingConvention(files: string[]): "noDotPrefix" | "keepDotPrefix" | "mixed" {
  const dotPrefixCount = files.filter(f => f.startsWith(".") && !f.startsWith(".git")).length;
  const noDotPrefixCount = files.filter(f => !f.startsWith(".") && !f.includes("/")).length;

  if (dotPrefixCount > 0 && noDotPrefixCount === 0) return "keepDotPrefix";
  if (noDotPrefixCount > 0 && dotPrefixCount === 0) return "noDotPrefix";
  return "mixed";
}

/**
 * Check if config file exists and is valid
 */
async function checkConfigFile(repoPath: string): Promise<AuditFinding[]> {
  const findings: AuditFinding[] = [];
  const configPath = resolve(repoPath, "dotfiles.config.ts");

  try {
    await stat(configPath);
    findings.push({
      severity: "info",
      category: "structure",
      message: "Found dotfiles.config.ts configuration file",
      path: "dotfiles.config.ts",
    });
  } catch {
    findings.push({
      severity: "error",
      category: "structure",
      message: "Missing dotfiles.config.ts configuration file",
      path: "dotfiles.config.ts",
      suggestion: "Create dotfiles.config.ts to define symlinks, packages, and templates",
    });
  }

  return findings;
}

/**
 * Check for common missing configurations
 */
function checkMissingConfigs(files: string[], config?: DotfilesConfig): AuditFinding[] {
  const findings: AuditFinding[] = [];
  const platform = getPlatform();

  // Get symlink targets if config exists
  const symlinkTargets = config ? Object.values(config.symlinks) : [];
  const allFiles = [...files, ...symlinkTargets];

  for (const commonConfig of COMMON_CONFIGS) {
    // Skip platform-specific configs that don't apply
    if (commonConfig.platform && commonConfig.platform !== "all" && commonConfig.platform !== platform) {
      continue;
    }

    const hasIt = hasConfig(allFiles, commonConfig);

    if (!hasIt) {
      let severity: AuditFinding["severity"];
      if (commonConfig.priority === 1) {
        severity = "warning";
      } else if (commonConfig.priority === 2) {
        severity = "suggestion";
      } else {
        severity = "info";
      }

      findings.push({
        severity,
        category: "missing",
        message: `Missing ${commonConfig.name}: ${commonConfig.description}`,
        suggestion: `Consider adding: ${commonConfig.fileNames[0]}`,
      });
    }
  }

  return findings;
}

/**
 * Check naming convention consistency
 */
function checkNamingConventions(files: string[]): AuditFinding[] {
  const findings: AuditFinding[] = [];
  const convention = detectNamingConvention(files);

  if (convention === "mixed") {
    findings.push({
      severity: "warning",
      category: "naming",
      message: "Mixed naming conventions detected (some files with dots, some without)",
      suggestion: "Consider standardizing on one naming convention for clarity",
    });
  } else {
    findings.push({
      severity: "info",
      category: "naming",
      message: `Using '${convention}' naming convention`,
    });
  }

  return findings;
}

/**
 * Check directory structure
 */
async function checkStructure(repoPath: string, files: string[]): Promise<AuditFinding[]> {
  const findings: AuditFinding[] = [];

  // Check for config directory
  const hasConfigDir = files.some(f => f.startsWith("config/"));
  const hasShellDir = files.some(f => f.startsWith("shell/"));
  const hasGitDir = files.some(f => f.startsWith("git/"));

  if (hasConfigDir || hasShellDir || hasGitDir) {
    findings.push({
      severity: "info",
      category: "structure",
      message: "Using organized directory structure",
    });
  } else {
    const rootFiles = files.filter(f => !f.includes("/"));
    if (rootFiles.length > 10) {
      findings.push({
        severity: "suggestion",
        category: "structure",
        message: `Many files at root level (${rootFiles.length}). Consider organizing into directories.`,
        suggestion: "Group related configs: shell/, git/, vim/, config/",
      });
    }
  }

  return findings;
}

/**
 * Calculate completeness score
 */
function calculateScore(findings: AuditFinding[]): number {
  let score = 100;

  for (const finding of findings) {
    switch (finding.severity) {
      case "error":
        score -= 20;
        break;
      case "warning":
        score -= 10;
        break;
      case "suggestion":
        score -= 3;
        break;
      // info doesn't affect score
    }
  }

  return Math.max(0, Math.min(100, score));
}

/**
 * Run a full audit on the dotfiles repository
 */
export async function runAudit(repoPath: string, config?: DotfilesConfig, options: AuditOptions = { verbose: false, json: false }): Promise<AuditResult> {
  const findings: AuditFinding[] = [];

  // Get all files in repo
  const files = await getAllFiles(repoPath);

  // Run all checks
  findings.push(...await checkConfigFile(repoPath));
  findings.push(...checkMissingConfigs(files, config));
  findings.push(...checkNamingConventions(files));
  findings.push(...await checkStructure(repoPath, files));

  // Calculate summary
  const summary = {
    errors: findings.filter(f => f.severity === "error").length,
    warnings: findings.filter(f => f.severity === "warning").length,
    info: findings.filter(f => f.severity === "info").length,
    suggestions: findings.filter(f => f.severity === "suggestion").length,
  };

  const score = calculateScore(findings);

  return {
    timestamp: new Date().toISOString(),
    repoPath,
    findings,
    summary,
    score,
  };
}

/**
 * Print audit results to console
 */
export function printAuditResults(result: AuditResult, options: AuditOptions): void {
  if (options.json) {
    console.log(JSON.stringify(result, null, 2));
    return;
  }

  logger.header("Dotfiles Audit");
  logger.table({
    "Repository": result.repoPath,
    "Score": `${result.score}/100`,
  });

  // Group findings by category
  const categories = ["structure", "naming", "missing", "convention"] as const;

  for (const category of categories) {
    const categoryFindings = result.findings.filter(f => f.category === category);
    if (categoryFindings.length === 0) continue;

    // Filter by minSeverity if set
    const severityOrder = ["error", "warning", "suggestion", "info"] as const;
    const minIndex = options.minSeverity ? severityOrder.indexOf(options.minSeverity) : severityOrder.length;
    const filteredFindings = categoryFindings.filter(f =>
      severityOrder.indexOf(f.severity) <= minIndex
    );

    if (filteredFindings.length === 0) continue;

    logger.subheader(category.charAt(0).toUpperCase() + category.slice(1));

    for (const finding of filteredFindings) {
      switch (finding.severity) {
        case "error":
          logger.error(finding.message);
          break;
        case "warning":
          logger.warn(finding.message);
          break;
        case "suggestion":
          logger.info(`${finding.message}`);
          break;
        case "info":
          logger.success(finding.message);
          break;
      }

      if (finding.suggestion && options.verbose) {
        console.log(`     ${finding.suggestion}`);
      }
    }
  }

  // Summary
  logger.subheader("Summary");
  logger.table({
    "Errors": String(result.summary.errors),
    "Warnings": String(result.summary.warnings),
    "Suggestions": String(result.summary.suggestions),
    "Info": String(result.summary.info),
  });

  logger.newline();
  if (result.score >= 80) {
    logger.success(`Score: ${result.score}/100 - Great dotfiles setup!`);
  } else if (result.score >= 60) {
    logger.info(`Score: ${result.score}/100 - Good setup with room for improvement`);
  } else {
    logger.warn(`Score: ${result.score}/100 - Consider addressing the issues above`);
  }
}
