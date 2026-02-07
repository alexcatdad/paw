# Security Policy

## Important Context

paw is a personal dotfiles manager that executes user-provided configuration (`dotfiles.config.ts`) with full permissions. It creates symlinks, installs packages, and runs lifecycle hooks. **Only use dotfiles repos you trust.**

## Reporting a Vulnerability

If you discover a security vulnerability in paw's core logic (path traversal, command injection, etc.), please report it responsibly:

1. **Do not** open a public GitHub issue
2. Email the maintainer or use [GitHub's private vulnerability reporting](https://github.com/alexcatdad/paw/security/advisories/new)
3. Include:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)

## Scope

Security concerns that are **in scope**:
- Path traversal in symlink creation or scaffold output
- Command injection through package names or hook arguments
- Backup/restore bypasses that could overwrite protected files
- Self-update mechanism vulnerabilities (binary verification, MITM)

Security concerns that are **out of scope**:
- Malicious `dotfiles.config.ts` — this file is user-controlled code and runs with full permissions by design
- Anything requiring physical access to the machine
- Social engineering

## Built-in Protections

- Path validation ensures symlinks and scaffolded files stay within allowed directories
- Package names are validated to prevent shell injection
- Original files are backed up before being overwritten
- Dry-run mode (`--dry-run`) lets you preview all changes before applying
