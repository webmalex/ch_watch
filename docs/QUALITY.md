# Проверка качества кода

## Базовые проверки

Эти проверки доступны только на стандартном Go toolchain и не требуют внешних linters.

### Форматирование

Проверить, что все Go files уже отформатированы:

```sh
make fmt-check
```

Исправить форматирование автоматически:

```sh
make fmt
```

### Unit и integration tests

```sh
make test
```

Это запускает весь набор `go test ./...`, включая queue tests, runner tests, CLI tests и watcher integration tests.

### Статический анализ из Go toolchain

```sh
make vet
```

### Проверка сборки

```sh
make build
```

### Рекомендуемый быстрый набор перед commit

```sh
make check
```

`make check` включает:

- `make fmt-check`
- `make test`
- `make vet`
- `make build`

## Установка внешних quality tools

На macOS все дополнительные инструменты можно поставить одной командой:

```sh
brew install pre-commit golangci-lint govulncheck
```

Проверить версии:

```sh
pre-commit --version
golangci-lint version
govulncheck -version
```

## Pre-commit workflow

`pre-commit --help` показывает, что для этого workflow нам важны команды `install`, `run`, `autoupdate`, `validate-config` и поддержка hook types `pre-commit` и `pre-push`.

### Установка hooks

```sh
make hooks-install
```

Эквивалентная команда без `make`:

```sh
pre-commit install --install-hooks -t pre-commit -t pre-push
```

### Что запускается автоматически

- `pre-commit`: базовые file checks и `make fmt-check`
- `pre-push`: `make test`, `make vet`, `make lint`
- `manual`: `make vuln`, `make test-race`, `make test-cover`, `make smoke-run`

### Ручной запуск hooks

Все hooks стадии `pre-commit`:

```sh
make hooks-run
```

Проверки стадии `pre-push`:

```sh
make hooks-run-push
```

Manual hooks:

```sh
make hooks-run-manual
```

### Валидация и обновление конфигурации hooks

Проверить `.pre-commit-config.yaml`:

```sh
pre-commit validate-config
```

Обновить pinned hook revisions:

```sh
make hooks-update
```

## Расширенные проверки

### Race detector

```sh
make test-race
```

Полезно перед merge и перед выпуском изменений в watcher/queue logic.

### Coverage

```sh
make test-cover
```

Команда создает `coverage.out` и печатает summary через `go tool cover -func`.

### GolangCI-Lint

```sh
make lint
```

Используется конфиг `/.golangci.yml`. Если tool еще не установлен:

```sh
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### Проверка уязвимостей зависимостей

```sh
make vuln
```

Если tool еще не установлен:

```sh
go install golang.org/x/vuln/cmd/govulncheck@latest
```

### Полный набор проверок

```sh
make check-full
```

`make check-full` выполняет:

- `make check`
- `make test-race`
- `make test-cover`
- `make lint`
- `make vuln`

## Ручные smoke checks

### One-shot dry run

```sh
make smoke-run
```

### Watch dry run

```sh
make smoke-watch
```

После запуска `make smoke-watch` можно сохранить `demo/ch/dev/tmp.sql`, `demo/ch/dev/query.sql` или `demo/ch/fm/task1.sql` и проверить, что rerun происходит только для измененного файла.

## Практический режим использования

- перед commit: `make check`
- для автоматизации локальных commits: `make hooks-install`
- перед большим merge или release: `make check-full`
- после изменений в watcher semantics: `make smoke-watch`
- после изменений в runner: `make smoke-run` и реальный `run` с `--db`
