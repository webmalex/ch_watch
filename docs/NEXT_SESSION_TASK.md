# Next Session Task

## Mission

Implement a production-quality Go utility for watching SQL debug files and rerunning only the changed file.

This project replaces the fragile `make + watchexec + batch rerun` approach used elsewhere. The new tool must be a standalone Go program with a small CLI and a well-tested internal architecture.

## Product Goal

Target workflow:

1. User starts a watch command once.
2. User edits any debug SQL file under `ch/`.
3. The tool deduplicates noisy filesystem events.
4. The tool reruns only the changed debug SQL file.
5. SQL output and errors stream directly to the console.

The user should never see accidental reruns of unrelated files because of duplicated filesystem events.

## Why This Exists

The old approach had two core UX problems:

- watcher events often arrive in noisy duplicated bursts;
- the old batch command can rerun more than the user actually changed.

The new tool must be file-oriented, deterministic, and pleasant for repeated local debugging.

## Important Constraint: "native inotify on Go"

The intent is: no external watcher such as `watchexec`, and no shell-based event dispatching.

However, the current machine is macOS, so a Linux-only raw `inotify` implementation would be impossible to run locally. Therefore the implementation should use a Go-native watcher abstraction backed by the OS-native event engine:

- Linux: `inotify`
- macOS: native backend provided by the Go watcher library

Recommended choice:

- `github.com/fsnotify/fsnotify`

This still satisfies the core requirement: native Go filesystem events, no external watcher process.

Do not build the core architecture around polling. Polling may exist only as an explicit optional fallback if truly needed later, but not in the main path.

## Primary Requirements

### Functional

The tool must:

- watch a root directory recursively;
- react only to files matching the semantic rule `**/_*.sql`;
- ignore non-debug SQL such as `query.sql`;
- add watches for directories created after startup;
- run only the changed SQL file;
- serialize executions so concurrent saves do not interleave SQL output;
- queue later work while one SQL file is already running;
- deduplicate noisy bursts of filesystem events;
- show clear console banners for `RUN`, `OK`, and `FAIL`.

### Non-Functional

The tool must:

- be testable without a real ClickHouse connection;
- separate watcher logic from runner logic;
- avoid shell quoting traps by sending SQL to the executor via stdin;
- handle `SIGINT`/graceful shutdown cleanly;
- have readable logs and predictable exit codes;
- be easy to run via `go run` and later via a tiny `make` wrapper.

## Recommended CLI Shape

Keep the CLI intentionally small.

Recommended commands:

### `watch`

Main development mode.

Example:

```sh
go run ./cmd/ch_watch watch \
  --root ./demo/ch \
  --db demo \
  --format PrettyCompact
```

Recommended flags:

- `--root`: watch root, default `./ch`
- `--db`: ClickHouse database name
- `--client`: path to `clickhouse-client`, default `clickhouse-client`
- `--format`: output format passed to client, default `PrettyCompact`
- `--debounce`: debounce window, e.g. `75ms`
- `--suppress`: suppression window for repeated same-file reruns across adjacent event batches, e.g. `250ms`
- `--print-events`: optional debug output of normalized events
- `--dry-run`: do not execute SQL, only print what would run

### `run`

One-shot execution of a single SQL file.

Example:

```sh
go run ./cmd/ch_watch run ./demo/ch/dev/_tmp.sql --db demo
```

This command is valuable for tests, manual verification, and thin `make` wrappers.

## Matching Rules

Only treat a file as runnable when all conditions hold:

- file extension is `.sql`;
- basename starts with `_`;
- path is under the watched root;
- path refers to a regular file.

Ignore editor artifacts and temporary files such as:

- swap files;
- hidden temporary files;
- incomplete rename targets that do not end with `.sql`;
- remove events for already deleted debug SQL files.

## Event Handling Semantics

### High-Level Model

Implement an internal pipeline like this:

1. filesystem event source;
2. path normalization;
3. recursive watch management;
4. semantic filtering to runnable debug SQL;
5. debounce batch builder;
6. dedupe and suppression logic;
7. sequential execution queue;
8. console reporter.

### Dedupe Strategy

Do not rely on a single mechanism.

Use two layers:

1. **Batch dedupe**
   - within one debounce batch, collapse multiple raw events for the same canonical file into one candidate run.

2. **Cross-batch suppression**
   - maintain recent execution or scheduling fingerprints such as `(canonical path, size, mtime)`;
   - if the same fingerprint reappears inside a short suppression window, ignore it.

This is important because many editors produce `write + chmod + rename + write` patterns that may cross debounce boundaries.

### Busy Behavior

When a SQL run is already in progress:

- collect new changed files into a pending set;
- when the current run completes, execute the pending files in deterministic order;
- if the same file changes many times while busy, run it once after the current job finishes;
- preserve order by first-seen time within the pending queue.

Recommended behavior:

- no hard restart of an already running SQL process;
- finish current run, then execute pending queue.

This is much easier to reason about and test than kill-and-restart behavior.

## Runner Requirements

The SQL runner should:

- read SQL file contents directly;
- invoke `clickhouse-client` as a subprocess;
- pass SQL text through stdin, not shell redirection;
- stream stdout/stderr to the terminal;
- return structured execution metadata to the caller.

Recommended command shape:

```text
clickhouse-client -d <db> -f <format>
```

The Go process should provide stdin.

### Dry Run

`--dry-run` must still exercise the full watcher, debounce, dedupe, and queue logic. It should simply replace the real runner with a reporter that prints the normalized target.

This will make manual smoke testing possible even without ClickHouse.

## Console UX

Every actual or dry run should print a compact, readable banner, for example:

```text
[12:30:11] RUN demo/ch/dev/_tmp.sql
[12:30:11] OK  demo/ch/dev/_tmp.sql (138ms)
```

On failure:

```text
[12:30:11] RUN  demo/ch/dev/_tmp.sql
[12:30:12] FAIL demo/ch/dev/_tmp.sql (exit 62, 842ms)
```

Do not flood the terminal with raw watcher noise unless `--print-events` is enabled.

## Recommended Architecture

Suggested layout:

```text
cmd/ch_watch/main.go
internal/app/
internal/cli/
internal/watch/
internal/queue/
internal/runner/
internal/report/
internal/model/
internal/testutil/
demo/ch/...
```

Recommended package responsibilities:

- `internal/cli`: flag parsing and app wiring
- `internal/watch`: recursive watch management and raw event normalization
- `internal/queue`: debounce, dedupe, suppression, pending queue
- `internal/runner`: real executor and fake executor for tests
- `internal/report`: console formatting
- `internal/model`: shared structs such as normalized event, run request, run result

## Testing Requirements

This project must be test-heavy from the start.

### Test Layers

#### 1. Pure unit tests

For pure logic with no real filesystem watcher:

- debug SQL path matching;
- path normalization;
- batch dedupe;
- cross-batch suppression;
- pending queue behavior;
- deterministic ordering.

#### 2. Filesystem integration tests

Use temp directories and a fake runner.

Test scenarios:

- modify one `_*.sql` file -> exactly one run;
- modify non-debug `query.sql` -> zero runs;
- create a new watched subdirectory after startup -> file in it is detected;
- duplicate writes to the same file -> one run;
- two files changed while first run is busy -> both run sequentially;
- same file changed repeatedly while busy -> one queued rerun;
- remove or rename noise does not crash the watcher.

These tests should not require ClickHouse.

#### 3. Runner tests

Use a fake command or injectable subprocess abstraction.

Verify:

- SQL is passed via stdin;
- arguments are built correctly;
- stdout/stderr propagation;
- exit code handling;
- dry-run mode bypasses subprocess execution.

#### 4. CLI tests

Minimal, but enough to confirm:

- flag parsing;
- required argument validation;
- `run` and `watch` wiring;
- `--dry-run` path.

### Coverage Goal

Aim for meaningful coverage of core logic, not just line count. The logic-heavy packages should be close to fully exercised.

## Demo Data Requirements

The repository must keep demo inputs for manual testing.

Required:

- `demo/ch/dev/_tmp.sql`
- `demo/ch/fm/_task1.sql`
- at least one non-debug SQL file such as `demo/ch/dev/query.sql`

Manual smoke scenarios should be documented in `README.md`.

Recommended manual flows:

1. run one file in dry mode;
2. watch demo tree in dry mode and save `_tmp.sql`;
3. watch demo tree and confirm `query.sql` is ignored;
4. if ClickHouse is available, run against a real DB.

## Make Integration

`make` is not the core product anymore.

Later, keep `make` only as a thin launcher for long commands, for example:

```make
ch_watch:
	go run ./cmd/ch_watch watch --root ./ch --db demo --format PrettyCompact
```

Do not move business logic back into `make`.

## Definition of Done

The first implementation session is done when all items below are true:

- Go module initialized and builds cleanly;
- `watch` command works on the demo tree;
- recursive watching works for new directories;
- duplicate filesystem noise does not cause duplicate reruns in covered scenarios;
- `run` command executes one SQL file correctly;
- `--dry-run` works and is documented;
- tests are comprehensive and green;
- repo contains `README.md` and demo files;
- no dependency on `watchexec`;
- core logic is separated enough to unit test without real subprocesses.

## Recommended First Implementation Order

1. initialize module and CLI skeleton;
2. implement path matching and normalized event model;
3. implement fake runner and queue logic with pure unit tests;
4. implement recursive watcher with fsnotify;
5. add integration tests with temp dirs;
6. implement real ClickHouse runner;
7. wire console reporting and dry-run mode;
8. finish README with usage examples.

## Notes To Future Session

- favor correctness and testability over clever concurrency;
- keep execution single-threaded from the user's point of view;
- do not overdesign the CLI;
- do not hide duplicate-run problems with giant debounce values alone;
- the hard part is not starting a process, it is getting event semantics right.
