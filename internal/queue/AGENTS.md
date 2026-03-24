# QUEUE DOMAIN GUIDE

## OVERVIEW
`internal/queue` owns debounce, duplicate suppression, pending ordering, and the guarantee that only one SQL run is active at a time.

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Main coordination loop | `controller.go` | `Controller.Run()` is the behavioral center |
| Stable queue ordering | `ordered_set.go` | preserves first-seen order without duplicates |
| Cross-batch duplicate filtering | `suppressor.go` | fingerprint window logic |
| Real behavior examples | `controller_test.go` | busy-runner, dedupe, rerun timing |

## CONVENTIONS
- Keep queue logic filesystem-agnostic; it should consume normalized runnable paths, not raw OS events.
- Preserve first-seen deterministic order when multiple files queue while a run is active.
- Prefer injected `SnapshotFunc`, `Runner`, `Reporter`, and `Now` clock for testability.
- Batch dedupe and cross-batch suppression are separate layers; do not merge them casually.

## ANTI-PATTERNS
- Do not restart a running SQL process on every save.
- Do not replace the ordered pending set with a map-only structure that loses order.
- Do not bypass suppression when trying to fix noisy editor writes.
- Do not add random goroutines inside queue logic without explicit coordination.

## TEST EXPECTATIONS
- Add or update tests for dedupe, busy-runner ordering, and repeated writes.
- Use fake runners and deterministic timeouts instead of real subprocesses.
- `controller_test.go` is the contract for intended rerun semantics.

## COMMANDS
```bash
go test ./internal/queue
go test -race ./internal/queue
```
