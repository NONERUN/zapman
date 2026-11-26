# Changelog

Как пользоваться сейчас — [`docs/usage.md`](docs/usage.md), [`AGENTS.md`](AGENTS.md), [`docs/dev.md`](docs/dev.md). Этот файл — только дельты.

## Unreleased

### Breaking

- Формат стратегий v2 (`"spec": 2`): `strategies/lists.json`, `targets.json`, `actions.json`, `games/*.json` и стратегии из правил `{ target, action }`; `wf` считается из портов целей. JSON Schema — `strategies/schema/`. Файлы v0.1 (`specVersion`) не читаются. Argv всех стратегий байт в байт как раньше. См. [`docs/dev.md`](docs/dev.md).

### Added

- Игровые модули (`strategies/games/`, окно «Игры...», `games` в `user/config.json`). Правила выбранных игр идут перед правилами стратегии, порты `wf` — в порядке возрастания. Списки IP игры — стоковый `lists/ipset-<игра>.txt` и свой `user/ipset-<игра>.txt` (новый файл — комментарий и `203.0.113.113/32`). См. [Игры](docs/usage.md#игры).
  - `sky` (Sky: Children of the Light): UDP 5000-8000 для IP из списков — fake TTL 8, cutoff 4, ×6; WebSocket чата и заданий — `multisplit`. Игроки, чат и задания проверены на winws2.
  - `photon` («Photon (UDP 5055-5058, 27000-27002)»): fake TTL 8, cutoff 4, ×6, любой протокол, `unknownUdp` `active:GAME_UDP`, `luaPayload` `all`; по умолчанию на все IP, опция `ipset` — только подсети Photon Cloud EU из списков (с Vultr `95.179.160.0/20` для Hazard Levels). Без TTL 8 игры (Sker Ritual) выкидывало из матча по `ClientTimeout`. Матч и голос проверены на winws2 для подсетей из списка; на всех IP и на winws не проверялось, PEAK с этим правилом — тоже.
- Опции игр: `options` и `when` у наборов в `games/*.json` (до 4 опций, каждая должна менять правила); в `user/config.json` — `gameOptions`, только значения не по умолчанию, например `"gameOptions": { "photon": { "ipset": true } }`. Опция Photon `ipset` (по умолчанию выключена) ограничивает правило списками `ipset-photon.txt`. Значения опций пишутся в `zapmanGames` (`photon 3 ipset=off`), поэтому служба после смены опции — устаревшая (`version`). Окно «Игры...»: опции — галки под игрой (неактивны, пока игра выключена), кнопка **Список** открывает пользовательский список игры в Notepad и создаёт его, если файла нет.
- Поля ZapretSpec `luaPayload`, `ttl` и `cutoff: "none"` для переноса проверенных профилей Sky без ручной правки ImagePath службы.
- Макет главного окна в [`README.md`](README.md) ([`docs/ui/Main.svg`](docs/ui/Main.svg)).
- Версии стратегий и игр: у `flowseal/*.json` — `upstream` (тег Flowseal, из которого взят `.bat`: general, ALT2, ALT5, ALT6, ALT7, ALT9 — 1.10.0; ALT13 — 1.10.2; остальные — 1.10.4) и `rev` (сейчас 3); у `zapman/*.json` и `games/*.json` — `version` (sky, photon — 3). Go: `Version()` / `Origin()` у стратегии и игры. Golden-файлы начинаются со строки `# version: …`; тест падает, если argv поменялся без новой версии. Сверка тегов с upstream — [`dev/upstream-versions`](dev/upstream-versions/). См. [`docs/dev.md`](docs/dev.md).
- [`dev/import-flowseal`](dev/import-flowseal/): `general*.bat` из git-ref upstream → `flowseal/*.json` с именованными кирпичами, round trip через `spec.Build`; отчёт `same` / `changed` / `new` / `removed` / `error`, `-write` пишет изменённые. См. [`docs/dev.md`](docs/dev.md).
- Go: [`internal/config`](internal/config/) — `user/config.json` (поля PowerShell-версии и новые `strategy`, `games`, `autoUpdateBricks`); атомарная запись, неизвестные поля сохраняются, ошибка называет поле. См. [`docs/dev.md`](docs/dev.md).
- Go: свои порты Game Filter — `gameFilterTcp` / `gameFilterUdp` в `user/config.json` (список `N` / `N-M`, по умолчанию `1024-65535`), как в Flowseal `2384961`.
- Go: [`internal/engine`](internal/engine/) — папки и `spec.Paths` (с `user/fakes/ACTIVE_*.bin`), создание недостающих `user:`-списков, сверка `bin/` с `versions.json`, командная строка движка, запуск на переднем плане с одним повтором при `0xC0000142`.
- Go: [`internal/service`](internal/service/) — служба `zapret` (имя PowerShell-версии): установка с восстановлением старой службы при ошибке, удаление (с WinDivert), старт, стоп, статус. В ключе службы — стратегия (`zapman`), версия, игры и SHA256 ImagePath; статус говорит, устарела ли служба и почему. Без прав администратора — понятная ошибка.
- Go: TCP timestamps (нужны `fooling=ts`) включаются при установке и старте службы, как в PowerShell-версии; проверка для диагностики — `TCPTimestamps()`.
- Go: при ошибке установки службы старая служба создаётся снова и запускается, если работала до установки.
- GUI на Go: `cmd/zapman` (Wails v3.1.0) — окна PS GUI (главное, «Стратегия», «Статус», тесты, hosts, fake, загрузка, диагностика) и трей в одном процессе; новое окно «Игры...» для игровых модулей. Бэкенд — [`internal/app`](internal/app/) поверх config, engine и service (запуск, служба, IPSet, fake, задача трея `zapman-tray`) и `internal/tools` (сеть, hosts, загрузки, диагностика, тесты стратегий). Флаги `-fake` (фейк в памяти) и `-root`. Exe просит права администратора (манифест через go-winres). См. [`docs/dev.md`](docs/dev.md).
- Fake из upstream (сейчас в `strategies/fakes/`): `tls_clienthello_www_sferum_ru`, `tls_clienthello_awswaf_com`, `tls_clienthello_steam_chat_com`, `tls_clienthello_steamstatic_com`, `tls_clienthello_teamspeak_com`, `quic_initial_lemana_pro`, `quic_initial_steam_chat_com`, `quic_initial_steamstatic_com`.
- Go: [`internal/tools/netreset`](internal/tools/netreset/) — сброс сети (`netsh winsock reset`, `netsh int ip reset all`, `netsh winhttp reset proxy`, `ipconfig /flushdns`) с логом как в PowerShell-версии и перезагрузка (`shutdown /r /f /t 0`). Программы запускает [`internal/tools/command`](internal/tools/command/) без окна, вывод из OEM-кодировки в UTF-8.
- Go: [`internal/tools/web`](internal/tools/web/) — последняя версия по тегам GitHub (лимит 8 с), загрузка IPSet в `user/ipset-all.txt` с прогрессом и атомарной записью (старый файл не портится при ошибке или отмене), проверка system hosts по первой и последней строке шаблона, открытие hosts в Notepad.
- Go: [`internal/tools/diag`](internal/tools/diag/) — диагностика: те же 18 проверок, ID, тексты и порядок, что в PowerShell-версии (`tcp` через `service.Manager.TCPTimestamps`), и действия `remove-services`, `remove-windivert`, `discord-cache` с логом `OK:` / `WARN:` / `FAIL:`.
- Go: [`internal/tools/stratest`](internal/tools/stratest/) — тесты стратегий без службы: стандартные (17 целей, curl HTTP / TLS1.2 / TLS1.3 и ping) и DPI (набор TCP 16-20 из hyperion-cs/dpi-checkers), строки `[n/m]`, аналитика, лучшая стратегия по правилу PowerShell-версии и файл в `test-results/`. Настройки из `TEST_CURL_TIMEOUT`, `TEST_MAX_PARALLEL`, `MONITOR_*`. Отмена останавливает движок и возвращает `context.Canceled`.
- GUI на Go: тесты стратегий как в PowerShell-версии — служба удаляется и возвращается точно такой, как была (и чужая, например от `service.bat`), запускается, только если работала; DPI-тесты на время ставят IPSet `any` (копия `user/ipset-all.test-backup.txt`, после сбоя возвращается при следующих тестах); «Запуск», что работал до тестов, стартует снова. В тесты идут только стратегии с флагами для выбранного движка.
- GUI на Go: проверка движка перед «Запуском», «Установкой» и тестами сверяет SHA256 файлов `bin/` с `bin/versions.json`; ошибки установки и возврата службы — понятным текстом (RU / EN), а не сырым текстом исключения.
- GUI на Go: иконка трея, окон и кнопки на панели задач синяя, пока обход работает (служба `zapret` или winws запущены), иначе серая (`assets/app-idle.ico`, делает `go generate ./assets`). Иконка exe не меняется.
- GUI на Go: жёлтая иконка — предупреждение, красная — ошибка; причина — первая строка подсказки и меню трея. См. [usage](docs/usage.md).
- GUI на Go: запуск другого `zapman.exe` (новая сборка на месте старой или копия из другой папки) закрывает работающий процесс и запускает новый; раньше открывалось окно старого. winws и служба не перезапускаются.

### Changed

- Go GUI: активные fake — `user\fakes\ACTIVE_<ИМЯ>.bin` (в PowerShell-версии — `bin\`); выбор в окне fake копирует файл туда.
- Go GUI: нет `user\fakes\ACTIVE_<ИМЯ>.bin` — запуск, установка службы и тесты создают его из дефолта `active` в `catalog.json` (`DISCORD_UDP` — `quic_initial_rutube_ru.bin`, `GAME_UDP` — `quic_initial_4pda_to.bin`); окно fake показывает этот дефолт вместо «нет файла». Выбранный файл не перезаписывается. Пакеты те же; в argv путь fake теперь `user\fakes\…`, а не `strategies\fakes\…`: у службы, поставленной раньше, путь старый до новой установки.
- Go GUI: окно «Игры...» показывает под названием игры описание модуля на языке интерфейса (новое поле `about` в `games/*.json`, `{ "ru", "en" }`; нет текста на языке интерфейса — берётся другой). Argv и `version` модулей не меняются.
- Go GUI: кнопка в строке диагностики — «Исправить» / "Fix" вместо «Запуск» / "Run" (как в PowerShell-версии): это действие исправления, не запуск обхода.
- zapret2 v1.0.4 → [v1.0.8](https://github.com/bol-van/zapret2/releases/tag/v1.0.8): `bin/winws2.exe`, `bin/lua/zapret-lib.lua`, `bin/lua/zapret-antidpi.lua`.
- Раскладка каталога: `strategies/catalog.json` (`id` `stock`, `title`, `spec`); стратегии Flowseal — `strategies/<имя>.json` → `strategies/flowseal/<имя>.json` (имена те же); свои стратегии — в `strategies/zapman/`. Кирпичи (`lists.json`, `targets.json`, `actions.json`) и `games/` — в корне каталога. Стратегия в корне — ошибка загрузки.
- Стратегии v2: в `strategies/actions.json` только действия, которые используются 3+ раза (11 пресетов вместо 32); остальные — прямо в правиле стратегии. Argv не меняется.
- Каталог `strategies/` самодостаточен: 21 fake/паттерн `.bin` перенесён из `bin/` в `strategies/fakes/`; в `bin/` остались только файлы движков. Ссылки в JSON: `bin:<файл>.bin` → `cat:fakes/<файл>.bin` (префикс `cat:` — путь от корня каталога). `bin/ACTIVE_GAME_UDP.bin` и `bin/ACTIVE_DISCORD_UDP.bin` удалены (байт в байт `quic_initial_4pda_to.bin` и `quic_initial_rutube_ru.bin`); ссылки `bin:ACTIVE_<ИМЯ>.bin` → `active:<ИМЯ>`: файл пользователя `user\fakes\ACTIVE_<ИМЯ>.bin`, если он есть, иначе дефолт из `active` в `catalog.json`. В `catalog.json` новое поле `files` — SHA256 каждого файла каталога (`go generate ./internal/spec`; тест падает, если список устарел). `.gitattributes`: `strategies/**/*.json` — `eol=lf`, `*.bin` — `binary`. `rev` всех стратегий Flowseal 2 → 3, `version` игр 1 → 2 — поменялись только пути fake в argv, сами пакеты те же. Генератор PowerShell (`src/`) новые префиксы не знает. См. [`docs/dev.md`](docs/dev.md).
- ZapretSpec (Go), winws2: `cutoff` N → `--out-range=-nN` (было `-dN`), счёт пакетов как у winws. Меняется argv winws2 всех стратегий с cutoff. Почему на TCP не точно как winws — [`docs/dev.md`](docs/dev.md).
- GitHub Release: push тега `v*.*.*` собирает архивы в **черновик**; Publish — вручную. Повтор не трогает immutable-ассеты. См. [`docs/dev.md`](docs/dev.md).
- Workflow Release собирает Go-версию: `zapman.exe` (`go build -tags production`, windows-latest) вместо PowerShell-файлов; в архиве нет `src/`, `zapman.bat`, `cli.bat`, есть `zapman.exe` и `assets/app.LICENSE.txt`. Версия — из git-тега, а не из `Zapman.psd1`: `-X github.com/NONERUN/zapman/internal/app.Version=<тег>` (заголовок окна, «Статус», проверка версии) и версия файла exe через go-winres. Сборка без `-X` — версия из build info Go или `dev-<коммит>`. См. [`docs/dev.md`](docs/dev.md#релиз).
- В архиве релиза нет `docs/dev.md` (документация для разработчиков): она на GitHub, ссылки в `README.md` ведут туда.
- Документация пользователя (`README.md`, `docs/usage.md`, `docs/troubleshooting.md`, `docs/ui/index.md`) — под Go-GUI: запуск `zapman.exe` (UAC из манифеста), Windows 10+ с WebView2, CLI пока нет; крестик с треем прячет окно; `zapman.exe --tray` в задании `zapman-tray`; окно «Игры...»; бэкап `user/ipset-all.txt.backup`; fake в `strategies/fakes/` → `user/fakes/`. Убраны `zapman.bat`, `cli.bat`, `src/`, WMF / Windows 7 и подмена WinDivert для Win7. Форма Issue: версия из заголовка или «Статус и журнал», текст «Статус и журнал» (обязательно), результат «Диагностики» (обязательно), игровые модули; без `cli.bat`.
- Стратегии: fake `bin:tls_clienthello_max_ru.bin` → `bin:tls_clienthello_www_sferum_ru.bin` в 15 `strategies/*.json` (ALT, ALT3, ALT4, ALT8, ALT10, ALT11, ALT12, EXP, FAKE TLS AUTO*, SIMPLE FAKE*), как upstream 93ebbbf. Файл `tls_clienthello_max_ru.bin` остаётся в каталоге (`strategies/fakes/`).
- ZapretSpec (Go), winws2: нет неявного TLS hello `!` для групп без `fake.tls`. ALT3, ALT8, FAKE TLS AUTO ALT/ALT2/ALT3 пишут `"tls": ["!"]` явно (на winws это `--dpi-desync-fake-tls=!`, тот же встроенный hello). Профили `payload: "all"` с UDP-fake больше не шлют google TLS hello.
- ZapretSpec (Go), winws2: в группах из нескольких слотов (`payload: "all"`, `discord,stun`) каждый fake идёт только на свой тип пакета, как у winws (`payload=` слота у `--lua-desync=fake`). Раньше все fake шли на все известные payload, а `unknown` / `unknownUdp` — не на неизвестные пакеты. Явный `luaPayload` заменяет эти фильтры (Sky, Photon). Меняется argv winws2 всех стратегий.
- ZapretSpec (Go), winws2: `tls_mod` только на последнем ref слота `tls` и только на вызове для `tls_client_hello`, как `--dpi-desync-fake-tls-mod` у winws. Без слота `tls` `tls_mod` не пишется (раньше шёл на `!` любого слота). Argv стратегий не меняется.
- ZapretSpec (Go), `badseq`: без `badseqIncrement` winws не получает `--dpi-desync-badseq-increment` (дефолт winws −10000 seq / −66000 ack; раньше писалось −10). На winws2 `tcp_seq` = `badseqIncrement` или −10000, `tcp_ack` всегда −66000, как badack winws. Меняется argv FAKE TLS AUTO (оба движка) и winws2 ALT4, ALT8, FAKE TLS AUTO ALT/ALT2, SIMPLE FAKE ALT.
- ZapretSpec (Go): одна проверка JSON для обоих движков, имена с учётом регистра. `engine` — ровно `winws` / `winws2`. Ошибка: нет `profiles` или список пуст; `desync` / `fooling` / `l7` / префикс пути не в нижнем регистре или неизвестны (`fooling` раньше проверялся только на winws2); `cutoff` не `nN` / `none`; `l7` с `quic` и `tls`; пустой домен в списке; fake не `!` / `0x<hex>` / путь с префиксом или с пробелом; `seqovlPattern` не путь (раньше winws2 принимал любой). Argv стратегий не меняется.
- Сток из upstream (Flowseal) по 2026-11: `lists/list-exclude.txt` (+`lesta.pw`), `lists/list-google.txt` (+`fonts.googleapis.com`, `www.gstatic.com`), `.service/ipset-service.txt` (источник **Скачать ipset**), `.service/hosts` (**Сверить hosts**).
- Инструменты разработки на Go: `dev\update-zapret.ps1` → [`dev/update-zapret`](dev/update-zapret/) (ещё сверяет файлы с `sha256sum.txt` релиза), `.githooks/commit-msg` → [`dev/commitmsg`](dev/commitmsg/), правила argv и Sky из `dev\lint.ps1` / `dev\test-sky.ps1` → `go test ./...`. Workflow Lint → [Go](.github/workflows/go.yml) (golangci-lint, vet, test).
- golangci-lint v2 и govulncheck — Go-инструменты в отдельном модуле [`tools/go.mod`](tools/go.mod) (`go tool -modfile=tools/go.mod golangci-lint run ./...`, `… govulncheck ./...`), настройки линтеров — [`.golangci.yml`](.golangci.yml); заменяют шаг gofmt в workflow Go. Toolchain Go 1.27.0 → 1.27.4 (уязвимости `net/http`, `crypto/tls`, `crypto/x509` из govulncheck).
- Go GUI: в меню трея пункт **Выход** — закрывает программу и тогда, когда главное окно спрятано в трей. Стратегия, запущенная без службы, после выхода работает дальше, как в PowerShell-версии. Документация — без «снять галку, чтобы закрыть».
- Go GUI: проверка версии сравнивает теги как числа `vMAJOR.MINOR.PATCH`, а не как строки. Сборка для разработки (`dev-<коммит>`, псевдоверсия, `+dirty`) обновление не получает: «Проверить версию» только показывает последний релиз.
- Локальная сборка в [`docs/dev.md`](docs/dev.md) — с `-ldflags "-H windowsgui"`: без окна консоли.

### Removed

- PowerShell-версия: `src/`, `zapman.bat`, `cli.bat`, `dev/*.ps1`, `PSScriptAnalyzerSettings.psd1`, `blinter.ini`. Иконка — в [`assets/`](assets/). Последний релиз PowerShell-версии — `v0.1.1`.

### Fixed

- Тесты: возврат прежнего `winws` после прогона берёт аргументы и из командной строки без кавычек; нет аргументов — предупреждение вместо старта пустого `winws`. `cli.bat tests` перед «Press any key» сбрасывает клавиши, нажатые во время прогона (upstream a1552cb).
- Диагностика: служба Happ (`HappService`) считается VPN (upstream 16cf6b8).
- Go GUI: главное окно больше не остаётся недоступным после ошибки, отмены или закрытия крестиком дочернего окна (тесты, «Стратегия», «Статус», fake, «Игры...», загрузка, hosts, диагностика). Окна включает и выключает одно место; при ошибке «Статуса» после действия главное окно больше не остаётся в состоянии «занято».
- Go GUI: при ошибке остановки службы перед тестами окно тестов не открывается.
- Go: чтение настроек службы `zapret` с ImagePath длиннее 4 КБ («array bounds are invalid»).
- Go GUI: процесс завершается после выхода, даже если цикл окон Wails не закончился (таймер 15 с).
- Go GUI: у дочерних окон обычный заголовок и крестик, а не узкий заголовок окна инструментов. Их владелец — главное окно: на панели задач их нет, а по щелчку на главное окно в панели задач они выходят вперёд.

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
