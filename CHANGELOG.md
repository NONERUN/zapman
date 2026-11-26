# Changelog

Как пользоваться сейчас — [`docs/usage.md`](docs/usage.md), [`AGENTS.md`](AGENTS.md), [`docs/dev.md`](docs/dev.md). Этот файл — только дельты.

## Unreleased

### Breaking

- Формат стратегий v2 (`"spec": 2`): `strategies/lists.json`, `targets.json`, `actions.json`, `games/*.json` и стратегии из правил `{ target, action }`; `wf` считается из портов целей. JSON Schema — `strategies/schema/`. Файлы v0.1 (`specVersion`) не читаются; перевод — одноразовый [`dev/convert-v2`](dev/convert-v2/). Argv всех стратегий байт в байт как раньше. См. [`docs/dev.md`](docs/dev.md).
- `general (ALT11 Photon)` удалена → `general (ALT11)` + игровые модули `sky`, `sker`, `photon`. Те же профили; игровые идут первыми, порты `wf` в порядке возрастания.

### Added

- Sky в `general (ALT11 Photon)`: UDP fake с TTL 8 для игровых IP и TCP multisplit для WebSocket чата/заданий; `user/ipset-sky.txt` и его обновление из игрового лога через трей. Игровая проверка выполнена на winws2.
- Sker Ritual в `general (ALT11 Photon)`: профиль UDP Photon с fake TTL 8 для подсетей из `user/ipset-sker.txt` (копия `lists/ipset-sker.default.txt`) перед общим профилем Photon. Общий профиль выкидывал из матча по `ClientTimeout`. Матч и голос проверены на winws2. Тот же профиль и подсеть Vultr `95.179.160.0/20` — для Hazard Levels.
- Поля ZapretSpec `luaPayload`, `ttl` и `cutoff: "none"` для переноса проверенных профилей Sky без ручной правки ImagePath службы.
- Макет главного окна в [`README.md`](README.md) ([`docs/ui/Main.svg`](docs/ui/Main.svg)).
- Версии стратегий и игр: у `flowseal/*.json` — `upstream` (тег Flowseal, из которого взят `.bat`: general, ALT2, ALT5, ALT6, ALT7, ALT9 — 1.10.0; ALT13 — 1.10.2; остальные — 1.10.4) и `rev` (сейчас 3); у `zapman/*.json` и `games/*.json` — `version` (sky, sker, photon — 2). Go: `Version()` / `Origin()` у стратегии и игры. Golden-файлы начинаются со строки `# version: …`; тест падает, если argv поменялся без новой версии. Сверка тегов с upstream — [`dev/upstream-versions`](dev/upstream-versions/). См. [`docs/dev.md`](docs/dev.md).
- [`dev/import-flowseal`](dev/import-flowseal/): `general*.bat` из git-ref upstream → `flowseal/*.json` с именованными кирпичами, round trip через `spec.Build`; отчёт `same` / `changed` / `new` / `removed` / `error`, `-write` пишет изменённые. См. [`docs/dev.md`](docs/dev.md).
- Go: [`internal/config`](internal/config/) — `user/config.json` (поля PowerShell-версии и новые `strategy`, `games`, `autoUpdateBricks`); атомарная запись, неизвестные поля сохраняются, ошибка называет поле. См. [`docs/dev.md`](docs/dev.md).
- Go: свои порты Game Filter — `gameFilterTcp` / `gameFilterUdp` в `user/config.json` (список `N` / `N-M`, по умолчанию `1024-65535`), как в Flowseal `2384961`.
- Go: [`internal/engine`](internal/engine/) — папки и `spec.Paths` (с `user/fakes/ACTIVE_*.bin`), создание недостающих `user:`-списков, сверка `bin/` с `versions.json`, командная строка движка, запуск на переднем плане с одним повтором при `0xC0000142`.
- Fake из upstream (сейчас в `strategies/fakes/`): `tls_clienthello_www_sferum_ru`, `tls_clienthello_awswaf_com`, `tls_clienthello_steam_chat_com`, `tls_clienthello_steamstatic_com`, `tls_clienthello_teamspeak_com`, `quic_initial_lemana_pro`, `quic_initial_steam_chat_com`, `quic_initial_steamstatic_com`.

### Changed

- zapret2 v1.0.4 → [v1.0.8](https://github.com/bol-van/zapret2/releases/tag/v1.0.8): `bin/winws2.exe`, `bin/lua/zapret-lib.lua`, `bin/lua/zapret-antidpi.lua`.
- Раскладка каталога: `strategies/catalog.json` (`id` `stock`, `title`, `spec`); стратегии Flowseal — `strategies/<имя>.json` → `strategies/flowseal/<имя>.json` (имена те же); свои стратегии — в `strategies/zapman/`. Кирпичи (`lists.json`, `targets.json`, `actions.json`) и `games/` — в корне каталога. Стратегия в корне — ошибка загрузки.
- Стратегии v2: в `strategies/actions.json` только действия, которые используются 3+ раза (11 пресетов вместо 32); остальные — прямо в правиле стратегии. Argv не меняется.
- Каталог `strategies/` самодостаточен: 21 fake/паттерн `.bin` перенесён из `bin/` в `strategies/fakes/`; в `bin/` остались только файлы движков. Ссылки в JSON: `bin:<файл>.bin` → `cat:fakes/<файл>.bin` (префикс `cat:` — путь от корня каталога). `bin/ACTIVE_GAME_UDP.bin` и `bin/ACTIVE_DISCORD_UDP.bin` удалены (байт в байт `quic_initial_4pda_to.bin` и `quic_initial_rutube_ru.bin`); ссылки `bin:ACTIVE_<ИМЯ>.bin` → `active:<ИМЯ>`: файл пользователя `user\fakes\ACTIVE_<ИМЯ>.bin`, если он есть, иначе дефолт из `active` в `catalog.json`. В `catalog.json` новое поле `files` — SHA256 каждого файла каталога (`go generate ./internal/spec`; тест падает, если список устарел). `.gitattributes`: `strategies/**/*.json` — `eol=lf`, `*.bin` — `binary`. `rev` всех стратегий Flowseal 2 → 3, `version` игр 1 → 2 — поменялись только пути fake в argv, сами пакеты те же. Генератор PowerShell (`src/`) новые префиксы не знает. См. [`docs/dev.md`](docs/dev.md).
- ZapretSpec (Go), winws2: `cutoff` N → `--out-range=-nN` (было `-dN`), счёт пакетов как у winws. Меняется argv winws2 всех стратегий с cutoff. Почему на TCP не точно как winws — [`docs/dev.md`](docs/dev.md).
- GitHub Release: push тега `v*.*.*` собирает архивы в **черновик**; Publish — вручную. Повтор не трогает immutable-ассеты. См. [`docs/dev.md`](docs/dev.md).
- Стратегии: fake `bin:tls_clienthello_max_ru.bin` → `bin:tls_clienthello_www_sferum_ru.bin` в 16 `strategies/*.json` (ALT, ALT3, ALT4, ALT8, ALT10, ALT11, ALT11 Photon, ALT12, EXP, FAKE TLS AUTO*, SIMPLE FAKE*), как upstream 93ebbbf. Файл `tls_clienthello_max_ru.bin` остаётся в каталоге (`strategies/fakes/`).
- ZapretSpec (Go), winws2: нет неявного TLS hello `!` для групп без `fake.tls`. ALT3, ALT8, FAKE TLS AUTO ALT/ALT2/ALT3 пишут `"tls": ["!"]` явно (на winws это `--dpi-desync-fake-tls=!`, тот же встроенный hello). Профили `payload: "all"` с UDP-fake больше не шлют google TLS hello.
- ZapretSpec (Go), winws2: в группах из нескольких слотов (`payload: "all"`, `discord,stun`) каждый fake идёт только на свой тип пакета, как у winws (`payload=` слота у `--lua-desync=fake`). Раньше все fake шли на все известные payload, а `unknown` / `unknownUdp` — не на неизвестные пакеты. Явный `luaPayload` заменяет эти фильтры (Sky, Sker). Меняется argv winws2 всех стратегий.
- ZapretSpec (Go), winws2: `tls_mod` только на последнем ref слота `tls` и только на вызове для `tls_client_hello`, как `--dpi-desync-fake-tls-mod` у winws. Без слота `tls` `tls_mod` не пишется (раньше шёл на `!` любого слота). Argv стратегий не меняется.
- ZapretSpec (Go), `badseq`: без `badseqIncrement` winws не получает `--dpi-desync-badseq-increment` (дефолт winws −10000 seq / −66000 ack; раньше писалось −10). На winws2 `tcp_seq` = `badseqIncrement` или −10000, `tcp_ack` всегда −66000, как badack winws. Меняется argv FAKE TLS AUTO (оба движка) и winws2 ALT4, ALT8, FAKE TLS AUTO ALT/ALT2, SIMPLE FAKE ALT.
- ZapretSpec (Go): одна проверка JSON для обоих движков, имена с учётом регистра. `engine` — ровно `winws` / `winws2`. Ошибка: нет `profiles` или список пуст; `desync` / `fooling` / `l7` / префикс пути не в нижнем регистре или неизвестны (`fooling` раньше проверялся только на winws2); `cutoff` не `nN` / `none`; `l7` с `quic` и `tls`; пустой домен в списке; fake не `!` / `0x<hex>` / путь с префиксом или с пробелом; `seqovlPattern` не путь (раньше winws2 принимал любой). Argv стратегий не меняется.
- Сток из upstream (Flowseal) по 2026-11: `lists/list-exclude.txt` (+`lesta.pw`), `lists/list-google.txt` (+`fonts.googleapis.com`, `www.gstatic.com`), `.service/ipset-service.txt` (источник **Скачать ipset**), `.service/hosts` (**Сверить hosts**).
- Инструменты разработки на Go: `dev\update-zapret.ps1` → [`dev/update-zapret`](dev/update-zapret/) (ещё сверяет файлы с `sha256sum.txt` релиза), `.githooks/commit-msg` → [`dev/commitmsg`](dev/commitmsg/), правила argv и Sky из `dev\lint.ps1` / `dev\test-sky.ps1` → `go test ./...`. Workflow Lint → [Go](.github/workflows/go.yml) (golangci-lint, vet, test).
- golangci-lint v2 и govulncheck — Go-инструменты в отдельном модуле [`tools/go.mod`](tools/go.mod) (`go tool -modfile=tools/go.mod golangci-lint run ./...`, `… govulncheck ./...`), настройки линтеров — [`.golangci.yml`](.golangci.yml); заменяют шаг gofmt в workflow Go. Toolchain Go 1.27.0 → 1.27.4 (уязвимости `net/http`, `crypto/tls`, `crypto/x509` из govulncheck).

### Removed

- PowerShell-версия на ветке `go`: `src/`, `zapman.bat`, `cli.bat`, `dev/*.ps1`, `PSScriptAnalyzerSettings.psd1`, `blinter.ini`. Иконка — в [`assets/`](assets/). PowerShell-версия остаётся на `main`.

### Fixed

- Тесты: возврат прежнего `winws` после прогона берёт аргументы и из командной строки без кавычек; нет аргументов — предупреждение вместо старта пустого `winws`. `cli.bat tests` перед «Press any key» сбрасывает клавиши, нажатые во время прогона (upstream a1552cb).
- Диагностика: служба Happ (`HappService`) считается VPN (upstream 16cf6b8).

## v0.1.1 - 2026-09-06

### Breaking

- Вход: `zapret.bat` → [`zapman.bat`](zapman.bat) (GUI) и [`cli.bat`](cli.bat). Продукт — **Zapret Manager**; служба `zapret`, exe `winws` / `winws2`.
- Обвязка (config, UI, тесты): `*-Zapret*` → `*-Zapman*`. Без алиасов. Движок — [`src/Zapret/Bypass.ps1`](src/Zapret/Bypass.ps1), имена `*-Zapret*`.
- Свои списки: `lists/*-user.txt` и `lists/ipset-all.txt` → `user/`. Старые пути не читаются.
- Стратегии: `strategies/*.ps1` → `*.json`. Argv собирает [`src/ZapretSpec/`](src/ZapretSpec/) (`specVersion`). Ручные `$argList` удалены.
- Имя стратегии в реестре службы: `zapret-discord-youtube` → `zapman` (снова **Установить службу**).
- Не читаются: старые флаги CLI, `last_strategy.txt`, `src/Zapret/config.json`.

### Added

- Оба exe в `bin/`, выбор `winws` / `winws2` в «Стратегия…».
- Трей: [`src/Zapman/Tray.ps1`](src/Zapman/Tray.ps1), задание `zapman-tray` (opt-out `trayWatch`).
- Тесты в GUI и `cli.bat tests`. **Ни одна не подходит** — reset Winsock/IP/WinHTTP/DNS.
- CLI: RU/EN (хаб — 4, `service` — 11).
- [`dev/lint.ps1`](dev/lint.ps1), [`dev/update-zapret.ps1`](dev/update-zapret.ps1), docs и макеты [`docs/ui/`](docs/ui/index.md).
- Настройки: `user/config.json`. Обновление: копируют папку `user/`.

### Changed

- Install службы: argv / pin / ImagePath до `sc delete`; старт службой; при сбое — снимок ImagePath. Для winws2 в ImagePath — `--chdir` на `bin/`.
- HTTP User-Agent продукта: `zapman`. Манифесты Zapman и ZapretSpec: `PowerShellVersion` 5.1.
- Цели HTTP/ping тестов — [`Get-ZapmanTestTargets`](src/Zapman/Tests.ps1).
- Версия продукта — `ModuleVersion` в [`Zapman.psd1`](src/Zapman/Zapman.psd1) (тег `v` + число). Движки — [`bin/versions.json`](bin/versions.json). JSON → флаги — [`docs/dev.md`](docs/dev.md).

### Removed

- `src/utils/` / корневой `utils/`, трей внутри GUI, `PLAN.md`, `.service/version.txt`, автокомментарий Issue, `.github/github-bot.yml`.
