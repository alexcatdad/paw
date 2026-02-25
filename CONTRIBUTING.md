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

## License

MIT
