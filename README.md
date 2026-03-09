# ch_watch

Нативный Go watcher для SQL debug workflows.

Текущий статус: в этом repo лежат implementation brief и demo data для первой coding session.

Планируемая цель:
- рекурсивно watch дерево `ch/` без `watchexec` или polling в main path;
- реагировать только на SQL files matching `**/*.sql`;
- deduplicate noisy filesystem events;
- запускать только измененный SQL file и stream результат в console;
- оставаться хорошо covered tests;
- keep `make` as thin launcher только для длинных команд.

Старт отсюда:
- implementation brief: `docs/NEXT_SESSION_TASK.md`
- demo files для ручных smoke tests: `demo/ch/`

Важное примечание:
- implementation должна быть Go-native и event-driven;
- используй нативную Go watcher library с OS backends (`inotify` на Linux, native backend на macOS), а не `watchexec`;
- не завязывай core runner на shell redirection.
