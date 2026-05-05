# ch_watch

Нативный Go watcher для SQL debug workflows с event-driven rerun только измененного файла.

## Что умеет

- рекурсивно watch дерево `ch/`;
- реагировать только на `.sql` files внутри выбранного root;
- deduplicate noisy filesystem events;
- queue изменения, пока текущий SQL file еще выполняется;
- запускать SQL через `clickhouse` по `stdin`, автоматически выбирая `client` или `local` режим;
- работать в `--dry-run` mode для smoke tests без ClickHouse;
- дампить результат запроса в файлы рядом с SQL файлом: `--dump` (PrettyCompact `.txt`), `--dump-txt` (TSV pipeline → `.txt`), `--dump-md` (TSV pipeline → `.md`); во все файлы добавляется комментарий с длительностью запроса.

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
go run ./cmd/ch_watch run ./demo/ch/dev/tmp.sql
```

Запуск всех `.sql` файлов в директории с PrettyCompact dump:

```sh
go run ./cmd/ch_watch run ./demo/ch --dump
```

Дополнительные dump views через TSV pipeline (оптимизация — без повторного запроса):

```sh
go run ./cmd/ch_watch run ./demo/ch/dev/tmp.sql --dump-txt
go run ./cmd/ch_watch run ./demo/ch/dev/tmp.sql --dump-md
go run ./cmd/ch_watch run ./demo/ch/dev/tmp.sql --dump --dump-txt --dump-md
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

Для полного developer workflow на macOS:

```sh
brew install go pre-commit golangci-lint govulncheck
```

## Проверка качества

Быстрый набор проверок:

```sh
make check
```

Полный набор проверок:

```sh
make check-full
```

Подключение git hooks:

```sh
make hooks-install
```

Подробности: `docs/QUALITY.md`

## Полезные flags

- `--root`: root directory для watch, по умолчанию `./ch`
- `--db`: имя ClickHouse database; если задан, используется `clickhouse client --database <db>`
- `--client`: путь к binary `clickhouse`, по умолчанию `clickhouse`
- `--format`: output format для `clickhouse client/local`, по умолчанию `PrettyCompact`
- `--debounce`: окно batch dedupe, по умолчанию `75ms`
- `--suppress`: окно suppression для повторных fingerprints, по умолчанию `250ms`
- `--print-events`: печатать normalized watcher events
- `--dry-run`: не выполнять SQL, а только печатать `RUN`/`OK`
- `--dump`: сохранять результат запроса в PrettyCompact `.txt` файл рядом с SQL файлом (надежно, корректно для `WITH TOTALS` и множественных result sets)
- `--dump-txt`: рендерить результат через TSV pipeline в `.txt` (`PrettyCompact`); включает canonical `.tsv` dump
- `--dump-md`: рендерить результат через TSV pipeline в `.md` (`Markdown`); включает canonical `.tsv` dump

## Версия

```sh
ch_watch version
ch_watch --version
ch_watch -v
```

## Тесты

```sh
go test ./...
```

## Demo Data

- guide для ручных smoke tests: `demo/README.md`
- сборка и install: `docs/INSTALL.md`
- quality checks и linters: `docs/QUALITY.md`
- git hooks config: `.pre-commit-config.yaml`
- project motivation and design intent: `docs/PROJECT_RATIONALE.md`
