# Contributing

## Development Setup

```bash
git clone https://github.com/alexcatdad/paw.git
cd paw
go test ./...
go build ./cmd/paw
```

## Commands

```bash
go test ./...
go test ./... -coverprofile=coverage.out
go build -o dist/paw ./cmd/paw
./scripts/test/quality-check.sh
./scripts/test/quality-fix.sh
```

## Quality + Security Gates

- CI blocks on formatting drift, `go vet`, `golangci-lint`, `shellcheck`, and `actionlint`.
- Security checks run through `govulncheck` + `gosec` and block on `HIGH`/`CRITICAL`.
- Lower-severity security findings are surfaced as warnings and artifacts.
- PR autofix workflow can commit safe formatting/lint fixes for same-repo branches.

## Expectations

- Keep `paw.toml` schema backward compatible (`version = 1` currently).
- Keep CLI behavior stable for existing commands.
- Add tests for any behavior change.

## Release Notes

- `main` uses conventional commits for automatic semantic versioning:
  - `feat:` => minor
  - `fix:`, `perf:`, `refactor:` => patch
  - `!` or `BREAKING CHANGE` => major
- `docs:`, `ci:`, `chore:`, `test:`, and `style:` commits do not create a release.
- Homebrew tap updates use a GitHub App token from:
  - repo variable `APP_ID`
  - repo secret `APP_SECRET`

## License

MIT
