# Demo Data

Эта директория содержит SQL files для ручных smoke tests после того, как Go watcher будет реализован.

Рекомендуемые проверки для будущей implementation:

1. `run` на `demo/ch/dev/tmp.sql`
2. `watch --dry-run` на `demo/ch/` и сохранить `demo/ch/dev/tmp.sql`
3. сохранить `demo/ch/dev/query.sql` и убедиться, что rerun идет только для этого файла
4. сохранить `demo/ch/fm/task1.sql` и убедиться, что rerun идет только для него

Эти файлы специально очень маленькие, чтобы их было легко редактировать и наблюдать.
