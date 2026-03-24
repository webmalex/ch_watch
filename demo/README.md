# Demo Data

Эта директория содержит SQL files для ручных smoke tests уже реализованного watcher.

Рекомендуемые проверки:

1. `go run ./cmd/ch_watch run ./demo/ch/dev/tmp.sql --dry-run`
2. `go run ./cmd/ch_watch watch --root ./demo/ch --dry-run`, затем сохранить `demo/ch/dev/tmp.sql`
3. в том же watch-сценарии сохранить `demo/ch/dev/query.sql` и убедиться, что rerun идет только для него
4. сохранить `demo/ch/fm/task1.sql` и убедиться, что rerun идет только для него
5. если доступен ClickHouse, повторить `run` и `watch` без `--dry-run` в двух вариантах: с `--db <name>` (режим `clickhouse client`) и без `--db` (режим `clickhouse local`)

Файлы специально маленькие, чтобы их было легко редактировать и наблюдать.
