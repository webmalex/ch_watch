[Русский](README.ru.md)

# ch_watch

Go CLI watcher for ClickHouse SQL debug workflows. Watches a directory tree for `.sql` file changes, debounces noisy filesystem events, and re-executes only the changed file through `clickhouse local` or `clickhouse client`.

## Install

```sh
go install github.com/webmalex/ch_watch/cmd/ch_watch@latest
```

Binary lands in `$GOBIN` (or `$(go env GOPATH)/bin`). Make sure it's on your `PATH`.

Pre-built binaries for linux/darwin/windows (amd64, arm64) are attached to [GitHub Releases](https://github.com/webmalex/ch_watch/releases) for each `v*` tag.

## Quick start

Dry run a single file (no ClickHouse required):

```sh
go run ./cmd/ch_watch run ./demo/ch/dev/tmp.sql --dry-run
```

Watch a demo tree in dry run:

```sh
go run ./cmd/ch_watch watch ./demo/ch --dry-run
```

## Usage

### Commands

| Command | Description |
|---------|-------------|
| `watch [root]` | Recursively watch `[root]` for `.sql` changes and re-execute on edit |
| `run [path]` | Execute a single `.sql` file or all `.sql` files in a directory |
| `version` | Print the binary version |

### Flags

| Flag | Description |
|------|-------------|
| `[root]` | Root directory to watch or run (default `./ch`; also accepts `--root`) |
| `--db` | ClickHouse database name; switches to `clickhouse client` mode (env: `CH_DB`) |
| `--client` | Path to the `clickhouse` binary (default `clickhouse`) |
| `--format` | Output format for ClickHouse (default `PrettyCompact`) |
| `--debounce` | Event batch dedup window (default `75ms`) |
| `--suppress` | Repeated-fingerprint suppression window (default `250ms`) |
| `--print-events` | Print normalized watcher events to stderr |
| `--dry-run` | Print `RUN`/`OK` without executing SQL |
| `--dump` | Save query result to file in `--format` (`.txt`) |
| `--dump-txt` | Save query result as PrettyCompact `.txt` |
| `--dump-md` | Save query result as Markdown `.md` |
| `--pipe-txt` | Render `.txt` from canonical `.tsv` without re-running the query |
| `--pipe-md` | Render `.md` from canonical `.tsv` without re-running the query |

Without `--db`, SQL runs through `clickhouse local`. With `--db`, it connects via `clickhouse client --database <db>`.

## Build from source

```sh
make build
```

See [docs/INSTALL.md](docs/INSTALL.md) for full build and install instructions.

## Development

- [CONTRIBUTING.md](CONTRIBUTING.md)
- [docs/QUALITY.md](docs/QUALITY.md)

## License

[MIT](LICENSE)
