# Project Rationale

## Why this tool exists

`ch_watch` replaces a fragile workflow built around `make`, external watchers, and broad batch reruns.

The old approach had two recurring UX problems:

- filesystem watchers often emitted noisy duplicate bursts;
- a single local edit could rerun more SQL than the user actually changed.

For iterative SQL debugging, that made feedback slow, noisy, and hard to trust.

## Product goal

This project exists to give a deterministic local workflow:

1. start one watch command;
2. edit any `.sql` file under the watched tree;
3. rerun only the changed SQL file;
4. keep console output readable and attributable to one file;
5. avoid duplicate reruns caused by editor or OS filesystem noise.

## Design intent

The tool is intentionally:

- **Go-native** — watcher and orchestration live inside the Go program, not in shell glue;
- **file-oriented** — execution is tied to the changed SQL file, not to a batch job;
- **deterministic** — debounce, suppression, and queueing are explicit parts of the architecture;
- **testable** — queue, watcher, and runner logic can be validated without a real ClickHouse server.

## Current execution model

The runtime now supports two explicit ClickHouse modes:

- with `--db`: execute through `clickhouse client`;
- without `--db`: execute through `clickhouse local`.

That keeps local experimentation lightweight while still supporting real database runs when needed.

## What should stay true

Future changes should preserve these project-level properties:

- no regression to broad multi-file reruns for one edit;
- no main-path return to polling or shell-heavy orchestration;
- no loss of clear, per-file console feedback;
- no weakening of the testable boundaries between watch, queue, runner, and report layers.
