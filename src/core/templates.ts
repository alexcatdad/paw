/**
 * Template File Generation
 * Generates .local files from templates for machine-specific configuration
 */

import { mkdir, stat } from "node:fs/promises";
import { resolve, dirname } from "node:path";
import type { InstallOptions } from "../types";
import { logger } from "./logger";
import { getHomeDir, contractPath } from "./os";
import { resolveConfigPath } from "./config";

/**
 * Generate a single template file
 * Never overwrites existing files - .local files are user-owned
 */
async function generateTemplate(
  templatePath: string,
  targetPath: string,
  options: InstallOptions
): Promise<boolean> {
  // Check if target already exists - never overwrite .local files
  try {
    await stat(targetPath);
    logger.skip(`${contractPath(targetPath)} (already exists - not overwriting .local file)`);
    return false;
  } catch {
    // Target doesn't exist, we can create it
  }

  // Check if template exists
  const templateFile = Bun.file(templatePath);
  if (!(await templateFile.exists())) {
    logger.error(`Template not found: ${contractPath(templatePath)}`);
    return false;
  }

  if (options.dryRun) {
    logger.dryRun(`Would create: ${contractPath(targetPath)} from template`);
    return true;
  }

  // Read template content
  const templateContent = await templateFile.text();

  // Create parent directory
  await mkdir(dirname(targetPath), { recursive: true });

  // Write template content to target
  await Bun.write(targetPath, templateContent);
  logger.success(`Created: ${contractPath(targetPath)} (from template)`);

  return true;
}

/**
 * Generate all template files
 */
export async function generateTemplates(
  templates: Record<string, string>,
  options: InstallOptions
): Promise<number> {
  const homeDir = getHomeDir();
  let created = 0;

  for (const [templateRel, targetRel] of Object.entries(templates)) {
    const templatePath = resolveConfigPath(templateRel);
    const targetPath = resolve(homeDir, targetRel);

    const wasCreated = await generateTemplate(templatePath, targetPath, options);
    if (wasCreated) created++;
  }

  return created;
}

/**
 * Check which template targets already exist
 */
export async function checkTemplateStatus(
  templates: Record<string, string>
): Promise<{ existing: string[]; missing: string[] }> {
  const homeDir = getHomeDir();
  const existing: string[] = [];
  const missing: string[] = [];

  for (const [_, targetRel] of Object.entries(templates)) {
    const targetPath = resolve(homeDir, targetRel);

    try {
      await stat(targetPath);
      existing.push(targetPath);
    } catch {
      missing.push(targetPath);
    }
  }

  return { existing, missing };
}
