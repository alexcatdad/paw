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
```

## Expectations

- Keep `paw.toml` schema backward compatible (`version = 1` currently).
- Keep CLI behavior stable for existing commands.
- Add tests for any behavior change.

## License

MIT
