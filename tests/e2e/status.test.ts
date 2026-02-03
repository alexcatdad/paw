/**
 * E2E Tests: status and doctor commands
 */

import { describe, test, expect, beforeAll, afterEach } from "bun:test";
import { buildTestImage, createContainer, installPawInContainer, type Container } from "./helpers/docker";
import { createTestRepo, setupTestRepoInContainer } from "./helpers/fixtures";
import { expectSuccess } from "./helpers/assertions";
import { $ } from "bun";

const BINARY_PATH = new URL("../../dist/paw-linux-x64", import.meta.url).pathname;

describe("paw status", () => {
  let container: Container;

  beforeAll(async () => {
    await buildTestImage();
    await $`bun build src/index.ts --compile --target=bun-linux-x64 --outfile=dist/paw-linux-x64`.quiet();
  });

  afterEach(async () => {
    if (container) await container.cleanup();
  });

  test("shows symlink status", async () => {
    container = await createContainer();
    await installPawInContainer(container, BINARY_PATH);

    const repoPath = await createTestRepo({
      symlinks: {
        "shell/zshrc": ".zshrc",
        "git/gitconfig": ".gitconfig",
      },
    });
    await setupTestRepoInContainer(container, repoPath);

    // Link one file
    await container.run("cd /home/testuser/dotfiles && paw link");

    // Check status
    const result = await container.run("cd /home/testuser/dotfiles && paw status");

    expectSuccess(result);
    expect(result.stdout).toContain("Symlinks");
    expect(result.stdout).toContain(".zshrc");
    expect(result.stdout).toContain("linked");
  });

  test("shows conflicts", async () => {
    container = await createContainer();
    await installPawInContainer(container, BINARY_PATH);

    const repoPath = await createTestRepo({
      symlinks: { "shell/zshrc": ".zshrc" },
    });
    await setupTestRepoInContainer(container, repoPath, {
      conflicts: [".zshrc"],
    });

    const result = await container.run("cd /home/testuser/dotfiles && paw status");

    expectSuccess(result);
    expect(result.stdout).toContain("conflict");
  });
});

describe("paw doctor", () => {
  let container: Container;

  beforeAll(async () => {
    await buildTestImage();
  });

  afterEach(async () => {
    if (container) await container.cleanup();
  });

  test("reports system info and checks", async () => {
    container = await createContainer();
    await installPawInContainer(container, BINARY_PATH);

    const repoPath = await createTestRepo({
      symlinks: { "shell/zshrc": ".zshrc" },
    });
    await setupTestRepoInContainer(container, repoPath);

    const result = await container.run("cd /home/testuser/dotfiles && paw doctor");

    expectSuccess(result);
    expect(result.stdout).toContain("System");
    expect(result.stdout).toContain("linux");
    expect(result.stdout).toContain("Required Tools");
    expect(result.stdout).toContain("git");
  });
});
