# Demo Data

This directory contains SQL files for manual smoke testing once the Go watcher is implemented.

Suggested checks for the future implementation:

1. `run` on `demo/ch/dev/_tmp.sql`
2. `watch --dry-run` on `demo/ch/` and save `demo/ch/dev/_tmp.sql`
3. confirm that `demo/ch/dev/query.sql` is ignored
4. save `demo/ch/fm/_task1.sql` and confirm only that file reruns

These files are intentionally tiny so they are easy to edit and observe.
