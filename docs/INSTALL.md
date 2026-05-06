# Сборка и установка

## Требования

- Go `1.26.1+`
- для реального запуска SQL: `clickhouse` в `PATH` или путь через `--client`

Для полного local workflow на macOS можно поставить инструменты через Homebrew:

```sh
brew install go pre-commit golangci-lint govulncheck
```

`pre-commit` уже нужен для hook workflow, а `golangci-lint` и `govulncheck` используются в расширенных quality checks.

## Локальная сборка

Собрать binary в `./bin/ch_watch`:

```sh
make build
```

Эквивалентная команда без `make`:

```sh
mkdir -p ./bin
go build -o ./bin/ch_watch ./cmd/ch_watch
```

После сборки binary можно проверить так:

```sh
./bin/ch_watch run ./demo/ch/dev/tmp.sql --dry-run
```

## Установка в `GOBIN`

Установить binary через стандартный Go workflow:

```sh
make install
```

Эквивалентная команда без `make`:

```sh
go install ./cmd/ch_watch
```

Go положит binary в `GOBIN`, а если он не задан - в `$(go env GOPATH)/bin`.

Проверить путь можно так:

```sh
go env GOBIN
go env GOPATH
```

Если нужен локальный install без записи в пользовательский `GOBIN`, можно переопределить переменную:

```sh
GOBIN="$(pwd)/bin" make install
```

## Подключение git hooks

После установки `pre-commit` можно сразу подключить hooks для этого repo:

```sh
make hooks-install
```

Эта команда ставит hook types `pre-commit` и `pre-push` и сразу загружает environments для конфигурации из `.pre-commit-config.yaml`.

## Обновление binary

- для локальной сборки снова выполнить `make build`
- для установленной версии снова выполнить `make install`

## Очистка артефактов сборки

```sh
make clean
```
