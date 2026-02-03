/**
 * Custom Test Assertions
 */

import { expect } from "bun:test";
import type { Container } from "./docker";

/**
 * Assert a symlink exists and points to expected target
 */
export async function expectSymlink(
  container: Container,
  linkPath: string,
  expectedTarget: string
): Promise<void> {
  const result = await container.run(`readlink ${linkPath}`);
  expect(result.exitCode).toBe(0);
  expect(result.stdout.trim()).toContain(expectedTarget);
}

/**
 * Assert a file exists
 */
export async function expectFileExists(container: Container, path: string): Promise<void> {
  const result = await container.run(`test -e ${path}`);
  expect(result.exitCode).toBe(0);
}

/**
 * Assert a file does not exist
 */
export async function expectFileNotExists(container: Container, path: string): Promise<void> {
  const result = await container.run(`test -e ${path}`);
  expect(result.exitCode).not.toBe(0);
}

/**
 * Assert command output contains text
 */
export function expectOutputContains(output: string, text: string): void {
  expect(output).toContain(text);
}

/**
 * Assert command succeeded
 */
export function expectSuccess(result: { exitCode: number }): void {
  expect(result.exitCode).toBe(0);
}

/**
 * Assert command failed
 */
export function expectFailure(result: { exitCode: number }): void {
  expect(result.exitCode).not.toBe(0);
}
