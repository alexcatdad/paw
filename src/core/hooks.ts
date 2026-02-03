/**
 * Hook Runner
 * Centralized hook execution with consistent context creation
 */

import { $ } from "bun";
import { getPlatform, getHomeDir, getRepoDir, commandExists } from "./os";
import { logger } from "./logger";
import type { HookContext, DotfilesConfig } from "../types";

/**
 * Create base hook context
 */
export function createHookContext(dryRun: boolean): HookContext {
  return {
    platform: getPlatform(),
    homeDir: getHomeDir(),
    repoDir: getRepoDir(),
    dryRun,
    shell: async (cmd: string) => {
      const { $ } = await import("bun");
      await $`sh -c ${cmd}`;
    },
    commandExists,
  };
}

/**
 * Run a hook if it exists
 */
export async function runHook<T extends HookContext>(
  config: DotfilesConfig,
  hookName: keyof NonNullable<DotfilesConfig["hooks"]>,
  context: T,
  label?: string
): Promise<void> {
  const hook = config.hooks?.[hookName];
  if (!hook) return;

  const displayName = label ?? hookName;
  logger.subheader(`Running ${displayName} hook`);

  try {
    await (hook as (ctx: T) => Promise<void>)(context);
  } catch (error) {
    logger.error(`Hook ${displayName} failed: ${error}`);
    throw error;
  }
}
