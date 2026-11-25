# Инструкции для агентов

**Zapret Manager** (`zapman`) запускает [zapret](https://github.com/bol-van/zapret) (`winws.exe`) и [zapret2](https://github.com/bol-van/zapret2) (`winws2.exe`) из `bin/`. Ветка `go` — переписывание на Go. PowerShell-версия (GUI, CLI, служба, трей) — только на ветке `main`; релизы собираются оттуда.

Какие поля JSON дают какие флаги — [`docs/dev.md`](docs/dev.md). [`CHANGELOG.md`](CHANGELOG.md) — дельты, не описание «как сейчас». Документация — на русском.

## Раскладка

| Путь | Роль |
|---|---|
| `internal/spec/` | Каталог стратегий: загрузка, проверка, сборка argv winws / winws2 (`Build`). Golden-файлы — `testdata/golden/` |
| `strategies/` | Сток-каталог: `catalog.json`, `lists.json`, `targets.json`, `actions.json`; `flowseal/` — стратегии Flowseal; `games/` — игровые модули; `fakes/` — fake и паттерны `.bin`; `schema/` — JSON Schema (генерируется) |
| `lists/` | Сток-списки (`list-*.txt`, `ipset-*.txt`, `*.default.txt`) |
| `bin/` | Только файлы движков: `winws.exe`, `winws2.exe`, `cygwin1.dll`, WinDivert, `lua/`. Теги и SHA256 — [`bin/versions.json`](bin/versions.json) |
| `.service/` | Данные для «Сверить hosts» (`hosts`) и «Скачать ipset» (`ipset-service.txt`) |
| `assets/` | Иконка и её лицензия |
| `dev/` | Инструменты разработки (ниже) |
| `user/` | Данные пользователя (gitignore) |

## Не делать

- Не читать `lists/*-user.txt`, старый `lists/ipset-all.txt` и корневой `config.json`. Свои списки и настройки — только `user/`.
- Не держать второй JSON стратегии «для winws2». Один JSON, два argv из `internal/spec`.
- Не разбирать строку флагов вне `internal/spec`.
- Не добавлять чтение второго файла «если первого нет», тихий fallback на другой `engine`, предупреждение об устаревании, которое оставляет оба поведения.
- Не писать атаки вне списка в [`docs/dev.md`](docs/dev.md).
- Не делать `git push`: пушит человек.

Новые стратегии не выдумывать. Смысл атак брать из [Flowseal/zapret-discord-youtube](https://github.com/Flowseal/zapret-discord-youtube). Свой набор файлов — только после явного решения в чате или issue.

Смена имени стратегии — строка в CHANGELOG Changed (`старое` → `новое`). Удаление стратегии без преемника — Breaking. Документацию (`README.md`, этот файл, `docs/`, `lists/`, JSON стратегий) править в том же изменении, что и код.

`bin/` всегда содержит и `winws.exe`, и `winws2.exe`. Нет exe выбранного `engine` — ошибка, не переход на другой exe.

Комментарии в коде — [ASD-STE100](https://www.asd-ste100.org/) Simplified Technical English.

## Проверки

Перед каждым коммитом все три без ошибок (`gofmt -l .` — пустой вывод):

```text
gofmt -l .
go vet ./...
go test ./...
```

То же делает workflow [Go](.github/workflows/go.yml). Тесты не ходят в сеть.

После любой правки каталога (`strategies/`, типы в `internal/spec`):

```text
go generate ./internal/spec
```

Он пишет `strategies/schema/` и список `files` (SHA256) в `catalog.json`; без него тест падает.

Argv стратегии или игры поменялся — поднять `rev` (Flowseal) или `version` (свои стратегии, игры). Первая строка golden-файла (`# version: …`) — страж: argv другой, а версия та же — `TestGolden` падает с `argv changed: bump rev/version`. Порядок: поднять версию → обновить golden → проверить diff:

```text
go test ./internal/spec -run "Golden|Edge" -update
```

`TestGolden` ещё проверяет правила argv (нет `--dpi-desync` на winws2 и `--lua-desync` на winws, `--blob=name:value` и т.д.); `TestGames` — свойства профилей Sky / Sker / Photon. `-update` их не обходит.

## Инструменты (`dev/`)

| Команда | Роль |
|---|---|
| `go run ./dev/update-zapret` | Официальные zip zapret / zapret2 → `bin/` + `bin/versions.json`; сверка с `sha256sum.txt` релиза. `-dry-run`, `-zapret-tag`, `-zapret2-tag`, `-force`. Останавливает службу `zapret` и winws |
| `go run ./dev/import-flowseal` | `general*.bat` из git-ref upstream → `strategies/flowseal/*.json` (`-write`) |
| `go run ./dev/upstream-versions` | Сверка `upstream`-тегов стратегий Flowseal с upstream (`-write`) |
| `dev/convert-v2` | Одноразовый перевод v0.1 → v2 |
| `dev/schemagen`, `dev/catalogfiles` | Вызываются из `go generate ./internal/spec` |
| `dev/commitmsg` | Хук commit-msg (ниже) |

Флаги и коды выхода — [`docs/dev.md`](docs/dev.md) и комментарий в начале `main.go`.

## Коммиты

- Тема — по-английски, в повелительном наклонении, как в `git log` (`Add …`, `Port …`, `Fix …`). Мелкие коммиты.
- Хук: `git config core.hooksPath .githooks`. [`.githooks/commit-msg`](.githooks/commit-msg) запускает `go run ./dev/commitmsg`: `Co-authored-by: Cursor <cursoragent@cursor.com>` → `Assisted-by: Cursor <cursoragent@cursor.com>`.
- Хуки не пропускать (`--no-verify`).
