# PROJECT KNOWLEDGE BASE

**Generated:** 2026-03-24
**Branch:** master
**State:** working-tree snapshot

## OVERVIEW
Go CLI watcher for SQL debug workflows. Core value: rerun only the changed SQL file with debounced filesystem events and execute it through `clickhouse local` or `clickhouse client`.

## STRUCTURE
```text
cmd/ch_watch/        # binary entry and signal handling
internal/app/        # orchestration and runtime defaults
internal/cli/        # command parsing and flag handling
internal/watch/      # recursive fsnotify watcher and SQL filtering
internal/queue/      # debounce, suppression, sequential execution
internal/runner/     # ClickHouse execution modes
internal/report/     # colored console output
internal/version/    # version string, set via -ldflags at build time
docs/                # install, quality, rationale
demo/                # smoke-test SQL fixtures
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Start from entrypoint | `cmd/ch_watch/main.go` | `main()` only wires signal handling into CLI |
| Understand top-level flow | `internal/app/app.go` | `RunWatch()` and `RunOnce()` coordinate everything |
| Change watcher semantics | `internal/watch/AGENTS.md` | Highest risk area for noisy events and path filtering |
| Change debounce / rerun policy | `internal/queue/AGENTS.md` | Owns batching, suppression, and run ordering |
| Change ClickHouse execution | `internal/runner/clickhouse.go` | `--db` => `client`; no `--db` => `local` |
| Change console output | `internal/report/report.go` | ANSI colors, emoji labels, system banners |
| Update docs | `docs/AGENTS.md` | Keep docs aligned with verified behavior |

## CODE MAP
| Symbol | Type | Location | Role |
|--------|------|----------|------|
| `main` | function | `cmd/ch_watch/main.go` | signal-aware binary entry |
| `RunWatch` | function | `internal/app/app.go` | watcher + queue + runner orchestration |
| `RunOnce` | function | `internal/app/app.go` | one-shot SQL execution; file or directory |
| `runDir` | function | `internal/app/app.go` | walk directory and execute all .sql files |
| `(*Controller).Run` | method | `internal/queue/controller.go` | debounce, suppression, single-run scheduling |
| `(*Recursive).Run` | method | `internal/watch/recursive.go` | recursive fsnotify event loop |
| `ClickHouseRunner.Run` | method | `internal/runner/clickhouse.go` | chooses `clickhouse client` vs `clickhouse local`; three dump paths: `runPlain` (no dump), `runDumpDirect` (`--dump` → PrettyCompact `.txt`), `runDumpWithRender` (`--dump-txt`/`--dump-md` → TSV pipeline → render `.txt`/`.md`) |
| `DumpFilePath` | function | `internal/runner/clickhouse.go` | derives canonical `.tsv` dump path from `.sql` path |
| `TextDumpFilePath` | function | `internal/runner/clickhouse.go` | derives PrettyCompact `.txt` dump path from `.sql` path |
| `MarkdownDumpFilePath` | function | `internal/runner/clickhouse.go` | derives Markdown `.md` dump path from `.sql` path |
| `DecodeExitCode` | function | `internal/runner/clickhouse.go` | decodes exit code into signal name when 128+ |
| `ConsoleReporter` | type | `internal/report/report.go` | colored lifecycle and system output |
| `Version` | var | `internal/version/version.go` | version string from VERSION file, set via `-ldflags` at build time |

## CONVENTIONS
- Tests start with `t.Parallel()` unless true serialization is required.
- Prefer injected dependencies (`exec`, clocks, fake runners, in-memory reporters) over real side effects in tests.
- Use `make check` before commit and `make check-full` before merge/release.
- **Always run `make check-full` locally before pushing** — it catches golangci-lint issues (errcheck, etc.) that `make check` skips. Do not rely on pre-commit hooks alone.
- Docs are written in Russian; CLI names, flags, file globs, and technical terms stay in English.
- Default binary name is `clickhouse`; do not drift back to the legacy `clickhouse-client` default.
- `--db` changes execution mode, not just a connection parameter: with DB uses `client`, without DB uses `local`.
- Version is stored in the `VERSION` file at project root; bump it there before release. `make build`/`make install` inject it via `-ldflags`.
- **Always bump VERSION** when making code changes (features, fixes, refactors) — even small ones. Follow semver: patch for fixes, minor for features.
- **After every completed task**: update documentation (README, docs/, CODE MAP, COMMANDS in AGENTS.md if flags/structure changed) and make a git commit. Mandatory — do not wait for explicit instruction.

## ANTI-PATTERNS (THIS PROJECT)
- Do not move core behavior back into `make` or shell wrappers.
- Do not make polling the main watcher path.
- Do not treat temporary or non-SQL files as runnable.
- Do not change queue/watch semantics without matching tests.
- Do not document workflows that have not been run locally.
- Do not resurrect session-planning docs as source-of-truth documentation.

## UNIQUE STYLES
- Lifecycle output is intentionally colored and emoji-based.
- Demo fixtures live in `demo/ch/` and double as smoke-test inputs.
- Root docs point to child `AGENTS.md` only for domains with real local complexity.

## COMMANDS
```bash
make check
make check-full
make smoke-run
make smoke-watch
go run ./cmd/ch_watch run ./demo/ch/dev/tmp.sql
go run ./cmd/ch_watch watch --root ./demo/ch --dry-run
go run ./cmd/ch_watch watch --root ./demo/ch --dry-run --dump
go run ./cmd/ch_watch run ./demo/ch/dev/tmp.sql --dump --dump-txt --dump-md
./bin/ch_watch version
make hooks-install
```

## NOTES
- `internal/queue` and `internal/watch` are the main correctness hotspots.
- `docs/PROJECT_RATIONALE.md` replaces the old session brief as the durable explanation of why this tool exists.
