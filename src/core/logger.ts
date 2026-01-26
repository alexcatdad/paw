/**
 * Colored Console Logger
 * Provides consistent, visually appealing output for CLI operations
 */

const COLORS = {
  reset: "\x1b[0m",
  bold: "\x1b[1m",
  dim: "\x1b[2m",
  red: "\x1b[31m",
  green: "\x1b[32m",
  yellow: "\x1b[33m",
  blue: "\x1b[34m",
  magenta: "\x1b[35m",
  cyan: "\x1b[36m",
  gray: "\x1b[90m",
  white: "\x1b[37m",
} as const;

const ICONS = {
  success: "\u2713", // ✓
  error: "\u2717",   // ✗
  warning: "\u26A0", // ⚠
  info: "\u2192",    // →
  skip: "\u25CB",    // ○
  dryRun: "\u25CC",  // ◌
  package: "\u25A0", // ■
  link: "\u26D3",    // ⛓
} as const;

export const logger = {
  /**
   * Log a success message (green checkmark)
   */
  success(msg: string): void {
    console.log(`${COLORS.green}${ICONS.success}${COLORS.reset} ${msg}`);
  },

  /**
   * Log an error message (red X)
   */
  error(msg: string): void {
    console.log(`${COLORS.red}${ICONS.error}${COLORS.reset} ${msg}`);
  },

  /**
   * Log a warning message (yellow triangle)
   */
  warn(msg: string): void {
    console.log(`${COLORS.yellow}${ICONS.warning}${COLORS.reset} ${msg}`);
  },

  /**
   * Log an info message (blue arrow)
   */
  info(msg: string): void {
    console.log(`${COLORS.blue}${ICONS.info}${COLORS.reset} ${msg}`);
  },

  /**
   * Log a skipped item (gray circle)
   */
  skip(msg: string): void {
    console.log(`${COLORS.gray}${ICONS.skip} ${msg}${COLORS.reset}`);
  },

  /**
   * Log a dry-run action (cyan dotted circle)
   */
  dryRun(msg: string): void {
    console.log(`${COLORS.cyan}${ICONS.dryRun} [dry-run]${COLORS.reset} ${msg}`);
  },

  /**
   * Log a section header
   */
  header(msg: string): void {
    console.log(`\n${COLORS.magenta}${COLORS.bold}═══ ${msg} ═══${COLORS.reset}\n`);
  },

  /**
   * Log a subsection header
   */
  subheader(msg: string): void {
    console.log(`\n${COLORS.cyan}─── ${msg} ───${COLORS.reset}\n`);
  },

  /**
   * Log a package installation message
   */
  package(msg: string): void {
    console.log(`${COLORS.blue}${ICONS.package}${COLORS.reset} ${msg}`);
  },

  /**
   * Log a symlink creation message
   */
  link(source: string, target: string): void {
    console.log(`${COLORS.green}${ICONS.link}${COLORS.reset} ${target} ${COLORS.dim}→${COLORS.reset} ${source}`);
  },

  /**
   * Log a debug message (only in verbose mode)
   */
  debug(msg: string, verbose: boolean = false): void {
    if (verbose) {
      console.log(`${COLORS.gray}[debug] ${msg}${COLORS.reset}`);
    }
  },

  /**
   * Log a blank line
   */
  newline(): void {
    console.log();
  },

  /**
   * Log a table of key-value pairs
   */
  table(entries: Record<string, string>): void {
    const maxKeyLength = Math.max(...Object.keys(entries).map(k => k.length));
    for (const [key, value] of Object.entries(entries)) {
      const paddedKey = key.padEnd(maxKeyLength);
      console.log(`  ${COLORS.dim}${paddedKey}${COLORS.reset}  ${value}`);
    }
  },
};

export default logger;
