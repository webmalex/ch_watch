# ch_watch

Нативный Go watcher для SQL debug workflows с event-driven rerun только измененного файла.

## Что умеет

- рекурсивно watch дерево `ch/`;
- реагировать только на `.sql` files внутри выбранного root;
- deduplicate noisy filesystem events;
- queue изменения, пока текущий SQL file еще выполняется;
- запускать SQL через `clickhouse-client` по `stdin`, без shell redirection;
- работать в `--dry-run` mode для smoke tests без ClickHouse.

## Быстрый старт

Dry run одного файла:

```sh
go run ./cmd/ch_watch run ./demo/ch/dev/tmp.sql --dry-run
```

Watch demo tree в dry run:

```sh
go run ./cmd/ch_watch watch --root ./demo/ch --dry-run
```

Реальный запуск через ClickHouse:

```sh
go run ./cmd/ch_watch run ./demo/ch/dev/tmp.sql --db demo
go run ./cmd/ch_watch watch --root ./demo/ch --db demo --format PrettyCompact
```

## Сборка и установка

Локальная сборка binary:

```sh
make build
```

Установка в `GOBIN`:

```sh
make install
```

Подробности: `docs/INSTALL.md`

## Полезные flags

- `--root`: root directory для watch, по умолчанию `./ch`
- `--db`: имя ClickHouse database; обязателен без `--dry-run`
- `--client`: путь к `clickhouse-client`, по умолчанию `clickhouse-client`
- `--format`: output format для `clickhouse-client`, по умолчанию `PrettyCompact`
- `--debounce`: окно batch dedupe, по умолчанию `75ms`
- `--suppress`: окно suppression для повторных fingerprints, по умолчанию `250ms`
- `--print-events`: печатать normalized watcher events
- `--dry-run`: не выполнять SQL, а только печатать `RUN`/`OK`

## Тесты

```sh
go test ./...
```

## Demo Data

- guide для ручных smoke tests: `demo/README.md`
- сборка и install: `docs/INSTALL.md`
- implementation brief первой сессии: `docs/NEXT_SESSION_TASK.md`
