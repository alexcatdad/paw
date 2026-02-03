/**
 * Docker Container Management for E2E Tests
 */

import { $ } from "bun";
import { randomUUID } from "crypto";

const IMAGE_NAME = "paw-e2e-test";

export interface Container {
  id: string;
  run: (cmd: string) => Promise<{ stdout: string; stderr: string; exitCode: number }>;
  copyTo: (localPath: string, containerPath: string) => Promise<void>;
  cleanup: () => Promise<void>;
}

/**
 * Build the test Docker image (run once before tests)
 */
export async function buildTestImage(): Promise<void> {
  const dockerfilePath = new URL("../docker/Dockerfile", import.meta.url).pathname;
  await $`docker build -t ${IMAGE_NAME} -f ${dockerfilePath} .`.quiet();
}

/**
 * Create a new container for testing
 */
export async function createContainer(): Promise<Container> {
  const id = `paw-test-${randomUUID().slice(0, 8)}`;

  // Start container in detached mode
  await $`docker run -d --name ${id} ${IMAGE_NAME} sleep infinity`.quiet();

  const container: Container = {
    id,

    async run(cmd: string) {
      const result = await $`docker exec ${id} bash -c ${cmd}`.quiet().nothrow();
      return {
        stdout: result.stdout.toString(),
        stderr: result.stderr.toString(),
        exitCode: result.exitCode,
      };
    },

    async copyTo(localPath: string, containerPath: string) {
      await $`docker cp ${localPath} ${id}:${containerPath}`.quiet();
    },

    async cleanup() {
      await $`docker rm -f ${id}`.quiet().nothrow();
    },
  };

  return container;
}

/**
 * Copy the built paw binary into a container
 */
export async function installPawInContainer(container: Container, binaryPath: string): Promise<void> {
  await container.copyTo(binaryPath, "/home/testuser/.local/bin/paw");
  await container.run("chmod +x /home/testuser/.local/bin/paw");
}
