/**
 * E2E Tests: link command
 */

import { describe, test, expect, beforeAll, afterEach } from "bun:test";
import { buildTestImage, createContainer, installPawInContainer, type Container } from "./helpers/docker";
import { createTestRepo, setupTestRepoInContainer } from "./helpers/fixtures";
import { expectSymlink, expectSuccess } from "./helpers/assertions";
import { $ } from "bun";

// Build binary path
const BINARY_PATH = new URL("../../dist/paw-linux-x64", import.meta.url).pathname;

describe("paw link", () => {
  let container: Container;

  beforeAll(async () => {
    // Build test image (once)
    await buildTestImage();

    // Build paw binary for Linux
    await $`bun build src/index.ts --compile --target=bun-linux-x64 --outfile=dist/paw-linux-x64`.quiet();
  });

  afterEach(async () => {
    if (container) {
      await container.cleanup();
    }
  });

  test("creates symlinks from config", async () => {
    container = await createContainer();
    await installPawInContainer(container, BINARY_PATH);

    // Create test repo
    const repoPath = await createTestRepo({
      symlinks: {
        "shell/zshrc": ".zshrc",
        "git/gitconfig": ".gitconfig",
      },
    });
    await setupTestRepoInContainer(container, repoPath);

    // Run paw link
    const result = await container.run("cd /home/testuser/dotfiles && paw link");
    expectSuccess(result);

    // Verify symlinks
    await expectSymlink(container, "/home/testuser/.zshrc", "dotfiles/config/shell/zshrc");
    await expectSymlink(container, "/home/testuser/.gitconfig", "dotfiles/config/git/gitconfig");
  });

  test("skips existing symlinks", async () => {
    container = await createContainer();
    await installPawInContainer(container, BINARY_PATH);

    const repoPath = await createTestRepo({
      symlinks: { "shell/zshrc": ".zshrc" },
    });
    await setupTestRepoInContainer(container, repoPath);

    // Run paw link twice
    await container.run("cd /home/testuser/dotfiles && paw link");
    const result = await container.run("cd /home/testuser/dotfiles && paw link");

    expectSuccess(result);
    expect(result.stdout).toContain("already linked");
  });

  test("reports conflicts without --force", async () => {
    container = await createContainer();
    await installPawInContainer(container, BINARY_PATH);

    const repoPath = await createTestRepo({
      symlinks: { "shell/zshrc": ".zshrc" },
    });
    await setupTestRepoInContainer(container, repoPath, {
      conflicts: [".zshrc"],
    });

    // Run paw link without --force and with --no-interactive
    const result = await container.run("cd /home/testuser/dotfiles && paw link --no-interactive");

    expectSuccess(result);
    expect(result.stdout).toContain("exists");
  });

  test("creates backup with --force", async () => {
    container = await createContainer();
    await installPawInContainer(container, BINARY_PATH);

    const repoPath = await createTestRepo({
      symlinks: { "shell/zshrc": ".zshrc" },
    });
    await setupTestRepoInContainer(container, repoPath, {
      conflicts: [".zshrc"],
    });

    // Run paw link with --force
    const result = await container.run("cd /home/testuser/dotfiles && paw link --force");

    expectSuccess(result);
    expect(result.stdout).toContain("Backed up");

    // Verify backup exists
    const backupResult = await container.run("ls /home/testuser/.zshrc.backup.*");
    expectSuccess(backupResult);
  });

  test("dry-run shows what would be done", async () => {
    container = await createContainer();
    await installPawInContainer(container, BINARY_PATH);

    const repoPath = await createTestRepo({
      symlinks: { "shell/zshrc": ".zshrc" },
    });
    await setupTestRepoInContainer(container, repoPath);

    // Run paw link --dry-run
    const result = await container.run("cd /home/testuser/dotfiles && paw link --dry-run");

    expectSuccess(result);
    expect(result.stdout).toContain("dry-run");

    // Verify symlink was NOT created
    const checkResult = await container.run("test -L /home/testuser/.zshrc");
    expect(checkResult.exitCode).not.toBe(0);
  });
});
