/**
 * E2E Tests: install command
 */

import { describe, test, expect, beforeAll, afterEach } from "bun:test";
import { buildTestImage, createContainer, installPawInContainer, type Container } from "./helpers/docker";
import { createTestRepo, setupTestRepoInContainer } from "./helpers/fixtures";
import { expectSymlink, expectSuccess } from "./helpers/assertions";
import { $ } from "bun";

const BINARY_PATH = new URL("../../dist/paw-linux-x64", import.meta.url).pathname;

describe("paw install", () => {
  let container: Container;

  beforeAll(async () => {
    await buildTestImage();
    await $`bun build src/index.ts --compile --target=bun-linux-x64 --outfile=dist/paw-linux-x64`.quiet();
  });

  afterEach(async () => {
    if (container) await container.cleanup();
  });

  test("installs packages and creates symlinks", async () => {
    container = await createContainer();
    await installPawInContainer(container, BINARY_PATH);

    const repoPath = await createTestRepo({
      symlinks: { "shell/zshrc": ".zshrc" },
      packages: { common: [] }, // Skip packages in test for speed
    });
    await setupTestRepoInContainer(container, repoPath);

    const result = await container.run("cd /home/testuser/dotfiles && paw install --skip-packages");

    expectSuccess(result);
    expect(result.stdout).toContain("Installation complete");
    await expectSymlink(container, "/home/testuser/.zshrc", "dotfiles/config/shell/zshrc");
  });

  test("runs pre/post install hooks", async () => {
    container = await createContainer();
    await installPawInContainer(container, BINARY_PATH);

    const repoPath = await createTestRepo({
      symlinks: { "shell/zshrc": ".zshrc" },
      hooks: {
        preInstall: 'console.log("PRE_INSTALL_HOOK_RAN")',
        postInstall: 'console.log("POST_INSTALL_HOOK_RAN")',
      },
    });
    await setupTestRepoInContainer(container, repoPath);

    const result = await container.run("cd /home/testuser/dotfiles && paw install --skip-packages");

    expectSuccess(result);
    expect(result.stdout).toContain("PRE_INSTALL_HOOK_RAN");
    expect(result.stdout).toContain("POST_INSTALL_HOOK_RAN");
  });
});
