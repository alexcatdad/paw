/**
 * E2E Tests: Machine-specific configurations
 */

import { describe, test, expect, beforeAll, afterEach } from "bun:test";
import { buildTestImage, createContainer, installPawInContainer, type Container } from "./helpers/docker";
import { expectSuccess, expectFileNotExists } from "./helpers/assertions";
import { mkdtemp, writeFile, mkdir } from "fs/promises";
import { tmpdir } from "os";
import { join } from "path";
import { $ } from "bun";

const BINARY_PATH = new URL("../../dist/paw-linux-x64", import.meta.url).pathname;

describe("machine-specific configs", () => {
  let container: Container;

  beforeAll(async () => {
    await buildTestImage();
    await $`bun build src/index.ts --compile --target=bun-linux-x64 --outfile=dist/paw-linux-x64`.quiet();
  });

  afterEach(async () => {
    if (container) await container.cleanup();
  });

  test("skips symlinks that don't match platform condition", async () => {
    container = await createContainer();
    await installPawInContainer(container, BINARY_PATH);

    // Create repo with darwin-only symlink (container is linux)
    const repoDir = await mkdtemp(join(tmpdir(), "paw-test-"));
    await mkdir(join(repoDir, "config/macos"), { recursive: true });
    await writeFile(join(repoDir, "config/macos/settings"), "# macOS settings");

    const configContent = `
import { defineConfig } from "paw";
export default defineConfig({
  symlinks: {
    "macos/settings": {
      target: ".macos-settings",
      when: { platform: "darwin" }
    }
  },
  packages: { common: [] },
  templates: {},
  ignore: [],
});
`;
    await writeFile(join(repoDir, "dotfiles.config.ts"), configContent);
    await $`git -C ${repoDir} init && git -C ${repoDir} add -A && git -C ${repoDir} commit -m "init"`.quiet();

    await container.copyTo(repoDir, "/home/testuser/dotfiles");
    await container.run("chown -R testuser:testuser /home/testuser/dotfiles");

    const result = await container.run("cd /home/testuser/dotfiles && paw link");

    expectSuccess(result);
    expect(result.stdout).toContain("skipped");
    expect(result.stdout).toContain("platform darwin");

    // File should not exist
    await expectFileNotExists(container, "/home/testuser/.macos-settings");
  });

  test("creates symlinks that match platform condition", async () => {
    container = await createContainer();
    await installPawInContainer(container, BINARY_PATH);

    // Create repo with linux-only symlink
    const repoDir = await mkdtemp(join(tmpdir(), "paw-test-"));
    await mkdir(join(repoDir, "config/linux"), { recursive: true });
    await writeFile(join(repoDir, "config/linux/settings"), "# Linux settings");

    const configContent = `
import { defineConfig } from "paw";
export default defineConfig({
  symlinks: {
    "linux/settings": {
      target: ".linux-settings",
      when: { platform: "linux" }
    }
  },
  packages: { common: [] },
  templates: {},
  ignore: [],
});
`;
    await writeFile(join(repoDir, "dotfiles.config.ts"), configContent);
    await $`git -C ${repoDir} init && git -C ${repoDir} add -A && git -C ${repoDir} commit -m "init"`.quiet();

    await container.copyTo(repoDir, "/home/testuser/dotfiles");
    await container.run("chown -R testuser:testuser /home/testuser/dotfiles");

    const result = await container.run("cd /home/testuser/dotfiles && paw link");

    expectSuccess(result);
    expect(result.stdout).not.toContain("skipped");

    // File should exist as symlink
    const checkResult = await container.run("test -L /home/testuser/.linux-settings");
    expect(checkResult.exitCode).toBe(0);
  });
});
