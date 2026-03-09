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
- перед большим merge или release: `make check-full`
- после изменений в watcher semantics: `make smoke-watch`
- после изменений в runner: `make smoke-run` и реальный `run` с `--db`
