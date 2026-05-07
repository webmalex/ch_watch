# Руководство для контрибьюторов

Краткий справочник по рабочему процессу в проекте. Подробности о сборке и качестве см. в `docs/INSTALL.md` и `docs/QUALITY.md`.

## Что понадобится

- **Go 1.26.1+**
- **make** (для стандартных target'ов)
- **GitHub CLI (`gh`)** для maintenance workflow с Dependabot и release
- **golangci-lint**, **govulncheck**, **pre-commit** (опционально, но рекомендуются; см. `docs/QUALITY.md`)
- **clickhouse** (опционально, только для реальных SQL-запросов; без него работает `--dry-run`)

Модуль: `github.com/webmalex/ch_watch`.

## Локальная настройка

```sh
git clone https://github.com/webmalex/ch_watch.git
cd ch_watch
make build
make test
```

Подключить git hooks (автоматические проверки при commit и push):

```sh
make hooks-install
```

## Проверки качества

| Когда | Что |
|-------|-----|
| Перед каждым commit | `make check` |
| Перед push или release | `make check-full` |

`make check` включает форматирование, тесты, `go vet` и сборку. `make check-full` добавляет race detector, coverage, golangci-lint и govulncheck.

## Стиль commit-сообщений

Проект использует Conventional Commits. Формат:

```
<тип>: краткое описание
```

Применяемые префиксы:

- `feat:` новая функциональность
- `fix:` исправление бага
- `docs:` изменения в документации
- `chore:` вспомогательные задачи (зависимости, VERSION, конфигурация)
- `ci:` изменения в CI/workflows
- `build:` изменения в сборке (Makefile, ldflags)
- `refactor:` рефакторинг без изменения поведения
- `test:` добавление или изменение тестов

Описание пишется на английском, по одному предложению. Пример:

```
feat: add --dump-md flag for Markdown query output
```

## Pull request

- Ветка `master` является trunk-веткой, PR отправляются в неё.
- Все CI jobs должны быть зелёными: `check`, `race`, `coverage`, `lint`, `vuln`.
- Один PR, одна логическая задача. Крупные изменения стоит разбить на последовательные PR.
- Перед отправкой: `make check-full`.

## Dependabot и patch release

Для локального preview без merge, commit, push и tag:

```sh
make deps_accept_dry_run
```

Для принятия последнего открытого Dependabot PR и выпуска release одной командой:

```sh
make deps_accept
```

Если нужно зафиксировать конкретный PR, можно передать `PR`:

```sh
make deps_accept PR=42
```

Команда сама находит последний open PR от Dependabot, показывает summary изменённых файлов, гоняет local gate в временном worktree, ждёт GitHub PR checks, squash-merge'ит PR, ждёт `master` CI, создаёт `v*` tag, ждёт release workflow и проверяет `GOPROXY=direct go install` с запуском binary. Версия берётся из `VERSION`: если tag `v<VERSION>` ещё не существует, выпускается она; если tag уже есть, команда поднимает patch до первого свободного tag и commit'ит новый `VERSION`.
