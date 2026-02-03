/**
 * Interactive Prompts
 * Handle user interaction for conflict resolution
 */

import { createInterface } from "readline";
import { $ } from "bun";
import { logger } from "./logger";
import { contractPath } from "./os";

export interface ConflictChoice {
  action: "skip" | "backup" | "overwrite" | "abort";
  applyToAll?: boolean;
}

/**
 * Check if stdin is interactive (TTY)
 */
export function isInteractive(): boolean {
  return process.stdin.isTTY === true;
}

/**
 * Read a single character from stdin
 */
async function readChar(): Promise<string> {
  return new Promise((resolve, reject) => {
    const rl = createInterface({
      input: process.stdin,
      output: process.stdout,
    });

    const cleanup = () => {
      try {
        process.stdin.setRawMode?.(false);
      } catch {
        // Ignore setRawMode errors during cleanup
      }
      rl.close();
    };

    const onData = (data: Buffer) => {
      cleanup();
      resolve(data.toString().toLowerCase());
    };

    const onError = (err: Error) => {
      cleanup();
      reject(err);
    };

    const onClose = () => {
      cleanup();
      reject(new Error("stdin closed"));
    };

    try {
      process.stdin.setRawMode?.(true);
    } catch {
      // setRawMode not supported, continue without it
    }

    process.stdin.resume();
    process.stdin.once("data", onData);
    process.stdin.once("error", onError);
    process.stdin.once("close", onClose);
  });
}

/**
 * Show diff between two files
 */
export async function showDiff(existingPath: string, sourcePath: string): Promise<void> {
  const result = await $`diff -u ${existingPath} ${sourcePath}`.quiet().nothrow();

  if (result.exitCode === 0) {
    logger.info("Files are identical");
  } else {
    console.log("\n" + result.text());
  }
}

/**
 * Prompt user for conflict resolution
 */
export async function promptConflict(
  targetPath: string,
  sourcePath: string
): Promise<ConflictChoice> {
  console.log(`
\x1b[33m⚠ ${contractPath(targetPath)} already exists\x1b[0m

  [s] Skip this file
  [b] Backup & link
  [o] Overwrite (no backup)
  [d] Show diff
  [a] Abort
  ─────────────────────
  [S] Skip all remaining
  [B] Backup all remaining
`);

  process.stdout.write("Choice [s/b/o/d/a/S/B]: ");

  while (true) {
    const char = await readChar();
    console.log(char); // Echo the character

    switch (char) {
      case "s":
        return { action: "skip" };
      case "b":
        return { action: "backup" };
      case "o":
        return { action: "overwrite" };
      case "d":
        await showDiff(targetPath, sourcePath);
        process.stdout.write("\nChoice [s/b/o/d/a/S/B]: ");
        continue;
      case "a":
        return { action: "abort" };
      case "S": // Capital S = skip all
        return { action: "skip", applyToAll: true };
      case "B": // Capital B = backup all
        return { action: "backup", applyToAll: true };
      default:
        process.stdout.write("Invalid choice. [s/b/o/d/a/S/B]: ");
    }
  }
}
