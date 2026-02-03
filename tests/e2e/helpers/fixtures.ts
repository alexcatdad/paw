/**
 * Test Fixture Generation
 */

import { mkdtemp, writeFile, mkdir } from "fs/promises";
import { tmpdir } from "os";
import { join } from "path";
import type { Container } from "./docker";

export interface TestRepoOptions {
  symlinks?: Record<string, string>;
  packages?: { common?: string[] };
  hooks?: {
    preInstall?: string;
    postInstall?: string;
  };
  /** Files to create in container home dir (for conflict testing) */
  conflicts?: string[];
}

/**
 * Create a test dotfiles repo locally
 */
export async function createTestRepo(options: TestRepoOptions = {}): Promise<string> {
  const repoDir = await mkdtemp(join(tmpdir(), "paw-test-repo-"));

  // Create config directory
  await mkdir(join(repoDir, "config"), { recursive: true });

  // Create symlink source files
  const symlinks = options.symlinks ?? { "shell/zshrc": ".zshrc" };
  for (const [source] of Object.entries(symlinks)) {
    const sourcePath = join(repoDir, "config", source);
    await mkdir(join(sourcePath, ".."), { recursive: true });
    await writeFile(sourcePath, `# Test file: ${source}\n`);
  }

  // Create dotfiles.config.ts
  const configContent = `
import { defineConfig } from "paw";

export default defineConfig({
  symlinks: ${JSON.stringify(symlinks)},
  packages: ${JSON.stringify(options.packages ?? { common: [] })},
  templates: {},
  ignore: [],
  ${options.hooks ? `hooks: {
    ${options.hooks.preInstall ? `preInstall: async (ctx) => { ${options.hooks.preInstall} },` : ""}
    ${options.hooks.postInstall ? `postInstall: async (ctx) => { ${options.hooks.postInstall} },` : ""}
  },` : ""}
});
`;
  await writeFile(join(repoDir, "dotfiles.config.ts"), configContent);

  // Initialize git repo
  const { $ } = await import("bun");
  await $`git -C ${repoDir} init`.quiet();
  await $`git -C ${repoDir} add -A`.quiet();
  await $`git -C ${repoDir} commit -m "Initial commit"`.quiet();

  return repoDir;
}

/**
 * Set up test repo in container
 */
export async function setupTestRepoInContainer(
  container: Container,
  localRepoPath: string,
  options: TestRepoOptions = {}
): Promise<void> {
  // Copy repo to container
  await container.copyTo(localRepoPath, "/home/testuser/dotfiles");
  await container.run("chown -R testuser:testuser /home/testuser/dotfiles");

  // Create conflict files if specified
  for (const conflict of options.conflicts ?? []) {
    await container.run(`echo "existing content" > /home/testuser/${conflict}`);
  }
}
