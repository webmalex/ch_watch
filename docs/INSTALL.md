# Сборка и установка

## Требования

- Go `1.26.1+`
- для реального запуска SQL: `clickhouse` в `PATH` или путь через `--client`

Для полного local workflow на macOS можно поставить инструменты через Homebrew:

```sh
brew install go pre-commit golangci-lint govulncheck
```

`pre-commit` уже нужен для hook workflow, а `golangci-lint` и `govulncheck` используются в расширенных quality checks.

## Установка из репозитория (публичная)

Для быстрой установки без клонирования репозитория:

```sh
go install github.com/webmalex/ch_watch/cmd/ch_watch@latest
```

Binary будет установлен в `GOBIN` (или `$(go env GOPATH)/bin`, если `GOBIN` не задан). Убедитесь, что этот каталог добавлен в `PATH`.

> **Примечание:** `go install` с remote path работает только для публичных репозиториев. Если репозиторий приватный, используйте локальную сборку (см. ниже).

## Бинарные архивы (GitHub Releases)

При создании tag в формате `v*` (например, `v0.7.0`) GitHub Actions автоматически собирает бинарные архивы и публикует их как GitHub Release. Workflow описан в `.github/workflows/release.yml`.

Архивы собираются для:

- linux, darwin, windows
- amd64, arm64

Скачать binary можно со страницы Releases репозитория, распаковать и использовать без установки Go.

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

- для публичной установки снова выполнить `go install github.com/webmalex/ch_watch/cmd/ch_watch@latest`
- для локальной сборки снова выполнить `make build`
- для установленной версии снова выполнить `make install` или `go install ./cmd/ch_watch`

## Очистка артефактов сборки

```sh
make clean
```
