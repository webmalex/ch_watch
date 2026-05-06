# ch_watch

Нативный Go watcher для SQL debug workflows с event-driven rerun только измененного файла.

Репозиторий: <https://github.com/webmalex/ch_watch>

## Что умеет

- рекурсивно watch дерево `ch/`;
- реагировать только на `.sql` files внутри выбранного root;
- deduplicate noisy filesystem events;
- queue изменения, пока текущий SQL file еще выполняется;
- запускать SQL через `clickhouse` по `stdin`, автоматически выбирая `client` или `local` режим;
- работать в `--dry-run` mode для smoke tests без ClickHouse;
- дампить результат запроса в файлы рядом с SQL файлом: `--dump`/`--dump-txt` (прямой dump в PrettyCompact `.txt`), `--dump-md` (прямой dump в Markdown `.md`), `--pipe-txt`/`--pipe-md` (TSV pipeline → render `.txt`/`.md`); во все файлы добавляется комментарий с длительностью запроса.

## Быстрый старт

Dry run одного файла:

```sh
go run ./cmd/ch_watch run ./demo/ch/dev/tmp.sql --dry-run
```

Watch demo tree в dry run:

```sh
go run ./cmd/ch_watch watch ./demo/ch --dry-run
```

Реальный запуск через ClickHouse:

```sh
go run ./cmd/ch_watch run ./demo/ch/dev/tmp.sql --db demo
go run ./cmd/ch_watch watch --root ./demo/ch --db demo --format PrettyCompact
go run ./cmd/ch_watch run ./demo/ch/dev/tmp.sql
```

Запуск всех `.sql` файлов в директории с прямым PrettyCompact dump:

```sh
go run ./cmd/ch_watch run ./demo/ch --dump
```

Прямой dump в Markdown:

```sh
go run ./cmd/ch_watch run ./demo/ch/dev/tmp.sql --dump-md
```

TSV pipeline — render `.txt`/`.md` из canonical `.tsv` без повторного тяжелого запроса:

```sh
go run ./cmd/ch_watch run ./demo/ch/dev/tmp.sql --pipe-txt
go run ./cmd/ch_watch run ./demo/ch/dev/tmp.sql --pipe-md
go run ./cmd/ch_watch run ./demo/ch/dev/tmp.sql --pipe-txt --pipe-md
```

Комбинирование прямого dump + pipe:

```sh
go run ./cmd/ch_watch run ./demo/ch/dev/tmp.sql --dump --pipe-md
```

## Сборка и установка

### Установка из репозитория (публичная)

```sh
go install github.com/webmalex/ch_watch/cmd/ch_watch@latest
```

Binary будет установлен в `GOBIN` (или `$(go env GOPATH)/bin`, если `GOBIN` не задан). Убедитесь, что этот каталог добавлен в `PATH`.

> **Примечание:** `go install` с remote path работает только для публичных репозиториев. Если репозиторий приватный, используйте локальную сборку (см. ниже).

### Бинарные архивы (GitHub Releases)

При создании tag в формате `v*` (например, `v0.7.0`) GitHub Actions автоматически собирает бинарные архивы и публикует их как GitHub Release. Workflow описан в `.github/workflows/release.yml`.

Архивы собираются для:

- linux, darwin, windows
- amd64, arm64

Скачать binary можно со страницы Releases репозитория, распаковать и использовать без установки Go.

### Локальная сборка и установка (разработка)

Локальная сборка binary:

```sh
make build
```

Установка в `GOBIN`:

```sh
make install
```

или напрямую:

```sh
go install ./cmd/ch_watch
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

- `[root]`: root directory для watch (позиционный аргумент, по умолчанию `./ch`; `--root` тоже работает)
- `--db`: имя ClickHouse database; если задан, используется `clickhouse client --database <db>` (также `CH_DB` env variable; флаг имеет приоритет)
- `--client`: путь к binary `clickhouse`, по умолчанию `clickhouse`
- `--format`: output format для `clickhouse client/local`, по умолчанию `PrettyCompact`
- `--debounce`: окно batch dedupe, по умолчанию `75ms`
- `--suppress`: окно suppression для повторных fingerprints, по умолчанию `250ms`
- `--print-events`: печатать normalized watcher events
- `--dry-run`: не выполнять SQL, а только печатать `RUN`/`OK`
- `--dump`: сохранять результат запроса напрямую в файл в формате `--format` (default PrettyCompact `.txt`; надёжно, корректно для `WITH TOTALS` и множественных result sets)
- `--dump-txt`: shorthand для `--dump` с PrettyCompact → `.txt`
- `--dump-md`: shorthand для `--dump` с Markdown → `.md`
- `--pipe-txt`: TSV pipeline → render PrettyCompact `.txt` из canonical `.tsv` (может терять precision floats)
- `--pipe-md`: TSV pipeline → render Markdown `.md` из canonical `.tsv` (может терять precision floats)

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
