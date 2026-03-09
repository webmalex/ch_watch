# ch_watch

Native Go watcher for SQL debug workflows.

Current status: this repo contains the implementation brief and demo data for the first coding session.

Planned goal:
- watch a `ch/` tree recursively without `watchexec` or polling in the main path;
- react only to debug SQL files matching `**/_*.sql`;
- deduplicate noisy filesystem events;
- run only the changed SQL file and stream the result to the console;
- stay heavily covered by tests;
- keep `make` as a thin launcher for long commands only.

Start here:
- implementation brief: `docs/NEXT_SESSION_TASK.md`
- demo files for manual smoke tests: `demo/ch/`

Important note:
- the implementation should be Go-native and event-driven;
- use a native Go watcher library with OS backends (`inotify` on Linux, native backend on macOS), not `watchexec`;
- do not depend on shell redirection inside the core runner.
