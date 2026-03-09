# Задача на следующую сессию

## Миссия

Реализовать production-quality Go utility для watch SQL files и rerun только измененного файла.

Этот проект заменяет хрупкий подход `make + watchexec + batch rerun`, который используется в другом месте. Новый tool должен быть standalone Go program с маленьким CLI и хорошо протестированной internal architecture.

## Цель продукта

Целевой workflow:

1. Пользователь один раз запускает команду watch.
2. Пользователь редактирует любой SQL file под `ch/`.
3. Tool deduplicate noisy filesystem events.
4. Tool rerun только измененный SQL file.
5. SQL output и errors stream напрямую в console.

Пользователь не должен видеть случайные rerun несвязанных файлов из-за duplicated filesystem events.

## Зачем это нужно

У старого подхода были две ключевые UX-проблемы:

- watcher events часто приходят шумными duplicated bursts;
- старая batch-команда может rerun больше, чем пользователь реально изменил.

Новый tool должен быть file-oriented, deterministic и удобным для повторяющегося локального debugging.

## Важное ограничение: "native inotify on Go"

Смысл такой: никаких external watcher вроде `watchexec` и никакого shell-based event dispatching.

Однако текущая машина - macOS, поэтому Linux-only raw `inotify` implementation локально запустить невозможно. Значит, implementation должна использовать Go-native watcher abstraction с OS-native event engine:

- Linux: `inotify`
- macOS: native backend, который дает Go watcher library

Рекомендуемый выбор:

- `github.com/fsnotify/fsnotify`

Это все равно удовлетворяет ключевому требованию: native Go filesystem events, без external watcher process.

Не строй core architecture вокруг polling. Polling может появиться только как явный optional fallback, если он действительно понадобится позже, но не в main path.

## Основные требования

### Functional

Tool должен:

- рекурсивно watch root directory;
- реагировать только на files matching semantic rule `**/*.sql`;
- игнорировать non-SQL files и editor artifacts;
- добавлять watches для директорий, созданных после startup;
- запускать только измененный SQL file;
- сериализовать executions, чтобы concurrent saves не перемешивали SQL output;
- queue последующую работу, пока один SQL file уже выполняется;
- deduplicate noisy bursts of filesystem events;
- показывать понятные console banners для `RUN`, `OK` и `FAIL`.

### Non-Functional

Tool должен:

- быть testable без реального ClickHouse connection;
- разделять watcher logic и runner logic;
- избегать shell quoting traps, передавая SQL в executor через stdin;
- корректно обрабатывать `SIGINT`/graceful shutdown;
- иметь читаемые logs и предсказуемые exit codes;
- легко запускаться через `go run`, а позже через маленький `make` wrapper.

## Рекомендуемая форма CLI

Держи CLI намеренно маленьким.

Рекомендуемые команды:

### `watch`

Основной development mode.

Пример:

```sh
go run ./cmd/ch_watch watch \
  --root ./demo/ch \
  --db demo \
  --format PrettyCompact
```

Рекомендуемые flags:

- `--root`: watch root, по умолчанию `./ch`
- `--db`: имя базы ClickHouse
- `--client`: путь к `clickhouse-client`, по умолчанию `clickhouse-client`
- `--format`: output format, который передается в client, по умолчанию `PrettyCompact`
- `--debounce`: debounce window, например `75ms`
- `--suppress`: suppression window для повторных rerun одного и того же файла через соседние event batches, например `250ms`
- `--print-events`: optional debug output нормализованных events
- `--dry-run`: не выполнять SQL, а только печатать, что было бы запущено

### `run`

One-shot execution одного SQL file.

Пример:

```sh
go run ./cmd/ch_watch run ./demo/ch/dev/tmp.sql --db demo
```

Эта команда полезна для tests, ручной verification и тонких `make` wrappers.

## Правила matching

Считай файл runnable только если выполнены все условия:

- file extension - `.sql`;
- path находится под watched root;
- path указывает на regular file.

Игнорируй editor artifacts и temporary files, например:

- swap files;
- hidden temporary files;
- incomplete rename targets, которые не оканчиваются на `.sql`;
- remove events для уже удаленных SQL files.

## Семантика обработки events

### High-Level model

Собери internal pipeline примерно так:

1. filesystem event source;
2. path normalization;
3. recursive watch management;
4. semantic filtering до runnable SQL files;
5. debounce batch builder;
6. dedupe и suppression logic;
7. sequential execution queue;
8. console reporter.

### Стратегия dedupe

Не полагайся на один механизм.

Используй два слоя:

1. **Batch dedupe**
   - внутри одного debounce batch схлопывай несколько raw events для одного canonical file в один candidate run.

2. **Cross-batch suppression**
   - поддерживай недавние execution или scheduling fingerprints, например `(canonical path, size, mtime)`;
   - если тот же fingerprint появляется снова внутри короткого suppression window, игнорируй его.

Это важно, потому что многие editors создают паттерны `write + chmod + rename + write`, которые могут пересекать границы debounce.

### Поведение при busy runner

Когда SQL run уже идет:

- собирай новые измененные файлы в pending set;
- когда текущий run завершается, выполняй pending files в deterministic order;
- если один и тот же файл меняется много раз, пока runner busy, запускай его один раз после завершения текущей job;
- сохраняй порядок по времени первого появления в pending queue.

Рекомендуемое поведение:

- никакого hard restart уже запущенного SQL process;
- сначала завершить текущий run, потом выполнить pending queue.

Такое поведение гораздо проще понимать и тестировать, чем kill-and-restart.

## Требования к runner

SQL runner должен:

- читать содержимое SQL file напрямую;
- вызывать `clickhouse-client` как subprocess;
- передавать SQL text через stdin, а не через shell redirection;
- stream `stdout`/`stderr` в terminal;
- возвращать caller structured execution metadata.

Рекомендуемая форма команды:

```text
clickhouse-client -d <db> -f <format>
```

Go process должен предоставить stdin.

### Dry Run

`--dry-run` все равно должен прогонять полный watcher, debounce, dedupe и queue logic. Он должен только заменить реальный runner на reporter, который печатает normalized target.

Это позволит делать ручные smoke tests даже без ClickHouse.

## Console UX

Каждый реальный или dry run должен печатать компактный и читаемый banner, например:

```text
[12:30:11] RUN demo/ch/dev/tmp.sql
[12:30:11] OK  demo/ch/dev/tmp.sql (138ms)
```

При ошибке:

```text
[12:30:11] RUN  demo/ch/dev/tmp.sql
[12:30:12] FAIL demo/ch/dev/tmp.sql (exit 62, 842ms)
```

Не зашумляй terminal raw watcher noise, если `--print-events` не включен.

## Рекомендуемая architecture

Предлагаемая структура:

```text
cmd/ch_watch/main.go
internal/app/
internal/cli/
internal/watch/
internal/queue/
internal/runner/
internal/report/
internal/model/
internal/testutil/
demo/ch/...
```

Рекомендуемые package responsibilities:

- `internal/cli`: parsing flags и wiring app
- `internal/watch`: recursive watch management и normalization raw events
- `internal/queue`: debounce, dedupe, suppression, pending queue
- `internal/runner`: реальный executor и fake executor для tests
- `internal/report`: formatting для console
- `internal/model`: общие structs, например normalized event, run request, run result

## Требования к testing

Этот проект должен с самого начала быть test-heavy.

### Слои tests

#### 1. Чистые unit tests

Для чистой логики без реального filesystem watcher:

- SQL path matching;
- path normalization;
- batch dedupe;
- cross-batch suppression;
- pending queue behavior;
- deterministic ordering.

#### 2. Filesystem integration tests

Используй temp directories и fake runner.

Сценарии для test:

- изменить один `.sql` file -> ровно один run;
- изменить `query.sql` -> ровно один run;
- создать новую watched subdirectory после startup -> файл внутри нее обнаруживается;
- duplicate writes в один и тот же файл -> один run;
- два файла меняются, пока первый run busy -> оба выполняются последовательно;
- один и тот же файл многократно меняется, пока runner busy -> один queued rerun;
- remove или rename noise не приводит к crash watcher.

Эти tests не должны требовать ClickHouse.

#### 3. Runner tests

Используй fake command или injectable subprocess abstraction.

Проверь:

- SQL передается через stdin;
- arguments собираются корректно;
- propagation `stdout`/`stderr`;
- обработку exit codes;
- `dry-run` mode обходит выполнение subprocess.

#### 4. CLI tests

Минимальные, но достаточные, чтобы подтвердить:

- parsing flags;
- validation обязательных arguments;
- wiring `run` и `watch`;
- путь `--dry-run`.

### Цель по coverage

Стремись к meaningful coverage core logic, а не только к line count. Packages с насыщенной логикой должны быть почти полностью покрыты.

## Требования к demo data

В repo должны остаться demo inputs для ручного testing.

Нужно сохранить:

- `demo/ch/dev/tmp.sql`
- `demo/ch/fm/task1.sql`
- как минимум один дополнительный SQL file, например `demo/ch/dev/query.sql`

Ручные smoke scenarios должны быть задокументированы в `README.md`.

Рекомендуемые ручные flow:

1. запустить один файл в dry mode;
2. watch demo tree в dry mode и сохранить `tmp.sql`;
3. watch demo tree и сохранить `query.sql`;
4. если ClickHouse доступен, выполнить run против реальной DB.

## Интеграция с Make

`make` больше не является core product.

Позже оставь `make` только как thin launcher для длинных команд, например:

```make
ch_watch:
	go run ./cmd/ch_watch watch --root ./ch --db demo --format PrettyCompact
```

Не возвращай business logic обратно в `make`.

## Definition of Done

Первая implementation session завершена, когда все пункты ниже истинны:

- Go module инициализирован и cleanly builds;
- команда `watch` работает на demo tree;
- recursive watching работает для новых директорий;
- duplicate filesystem noise не вызывает duplicate rerun в покрытых сценариях;
- команда `run` корректно выполняет один SQL file;
- `--dry-run` работает и задокументирован;
- tests comprehensive и green;
- в repo есть `README.md` и demo files;
- нет зависимости от `watchexec`;
- core logic достаточно разделена, чтобы unit test проходили без реальных subprocesses.

## Рекомендуемый порядок первой implementation

1. инициализировать module и skeleton CLI;
2. реализовать path matching и normalized event model;
3. реализовать fake runner и queue logic с чистыми unit tests;
4. реализовать recursive watcher с `fsnotify`;
5. добавить integration tests с temp dirs;
6. реализовать реальный ClickHouse runner;
7. собрать console reporting и dry-run mode;
8. закончить `README` примерами использования.

## Заметки для будущей сессии

- предпочитай correctness и testability вместо clever concurrency;
- с точки зрения пользователя держи execution single-threaded;
- не переусложняй CLI;
- не прячь duplicate-run проблемы только за счет огромных debounce values;
- сложность не в запуске process, а в корректной event semantics.
