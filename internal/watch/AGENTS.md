# WATCH DOMAIN GUIDE

## OVERVIEW
`internal/watch` turns raw fsnotify activity into normalized SQL-file events under the watched root and handles recursive directory discovery.

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Recursive watcher loop | `recursive.go` | add/remove watched directories, event filtering |
| Path normalization rules | `path.go` | `NormalizePath`, `IsSQLFile`, `IsWithinRoot`, `SnapshotFile` |
| Integration scenarios | `recursive_test.go` | new dirs, duplicate writes, rename noise |
| Path edge cases | `path_test.go` | SQL matching and root scoping |

## CONVENTIONS
- `SnapshotFile()` is the runnable-file gate; keep it strict about root, extension, and regular files.
- Only `.sql` files under the watched root should reach downstream queue logic.
- New directories created after startup must be added recursively.
- Use normalized slash-safe path handling and root checks before emitting events.

## ANTI-PATTERNS
- Do not introduce polling as the default watch strategy.
- Do not emit events for temp files, renamed noise, or files outside the root.
- Do not let remove/rename noise crash the watcher loop.
- Do not couple watch logic to ClickHouse execution details.

## TEST EXPECTATIONS
- Integration-style tests here are the behavioral contract.
- Keep coverage for duplicate writes, new directory discovery, queueing while busy, and rename noise.
- Use temp dirs and fake runners; avoid depending on external ClickHouse.

## COMMANDS
```bash
go test ./internal/watch
go test -race ./internal/watch
```
