# Contributing to paw

Thanks for your interest in contributing. Since this is a personal dotfiles manager, the best way to use it is to **fork the repo** and make it your own.

## Reporting Issues

If you find a bug in the core logic that would affect forks:

1. Check [existing issues](https://github.com/alexcatdad/paw/issues) first
2. Open a new issue with:
   - What you expected to happen
   - What actually happened
   - Your OS and paw version (`paw --version`)
   - Steps to reproduce

## Pull Requests

1. Fork the repository
2. Create a feature branch (`git checkout -b feat/my-change`)
3. Make your changes
4. Run type checking: `bun run typecheck`
5. Run tests: `bun test`
6. Commit with a clear message
7. Open a PR against `main`

## Development Setup

```bash
git clone https://github.com/alexcatdad/paw.git
cd paw
bun install
bun run dev status        # Run locally
bun run typecheck         # Type check
bun test                  # Run tests
```

## Code Style

- TypeScript with strict mode
- No external runtime dependencies
- Use the existing `logger` module for console output
- Follow the patterns in `CLAUDE.md`

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
