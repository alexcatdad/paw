/**
 * E2E Tests: rollback command
 */

import { describe, test, expect, beforeAll, afterEach } from "bun:test";
import { buildTestImage, createContainer, installPawInContainer, type Container } from "./helpers/docker";
import { createTestRepo, setupTestRepoInContainer } from "./helpers/fixtures";
import { expectSuccess, expectFileExists } from "./helpers/assertions";
import { $ } from "bun";

const BINARY_PATH = new URL("../../dist/paw-linux-x64", import.meta.url).pathname;

describe("paw rollback", () => {
  let container: Container;

  beforeAll(async () => {
    await buildTestImage();
    await $`bun build src/index.ts --compile --target=bun-linux-x64 --outfile=dist/paw-linux-x64`.quiet();
  });

  afterEach(async () => {
    if (container) await container.cleanup();
  });

  test("removes symlinks and restores backups", async () => {
    container = await createContainer();
    await installPawInContainer(container, BINARY_PATH);

    const repoPath = await createTestRepo({
      symlinks: { "shell/zshrc": ".zshrc" },
    });
    await setupTestRepoInContainer(container, repoPath, {
      conflicts: [".zshrc"],
    });

    // Install with force (creates backup)
    await container.run("cd /home/testuser/dotfiles && paw install --skip-packages --force");

    // Verify symlink exists
    await expectFileExists(container, "/home/testuser/.zshrc");

    // Rollback
    const result = await container.run("cd /home/testuser/dotfiles && paw rollback");

    expectSuccess(result);
    expect(result.stdout).toContain("Rollback complete");

    // Verify original file is restored (not a symlink)
    const typeResult = await container.run("test -L /home/testuser/.zshrc");
    expect(typeResult.exitCode).not.toBe(0); // Should NOT be a symlink

    const contentResult = await container.run("cat /home/testuser/.zshrc");
    expect(contentResult.stdout).toContain("existing content");
  });

  test("fails gracefully with no previous state", async () => {
    container = await createContainer();
    await installPawInContainer(container, BINARY_PATH);

    const repoPath = await createTestRepo({
      symlinks: { "shell/zshrc": ".zshrc" },
    });
    await setupTestRepoInContainer(container, repoPath);

    // Try rollback without previous install
    const result = await container.run("cd /home/testuser/dotfiles && paw rollback");

    expect(result.exitCode).not.toBe(0);
    expect(result.stdout + result.stderr).toContain("No previous run state");
  });
});
