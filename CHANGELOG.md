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
- Fake из upstream в `bin/` (видны в **Fake…**): `tls_clienthello_www_sferum_ru`, `tls_clienthello_awswaf_com`, `tls_clienthello_steam_chat_com`, `tls_clienthello_steamstatic_com`, `tls_clienthello_teamspeak_com`, `quic_initial_lemana_pro`, `quic_initial_steam_chat_com`, `quic_initial_steamstatic_com`.

### Changed

- ZapretSpec (Go), winws2: `cutoff` N → `--out-range=-nN` (было `-dN`), счёт пакетов как у winws. Меняется argv winws2 всех стратегий с cutoff. Почему на TCP не точно как winws — [`docs/dev.md`](docs/dev.md).
- GitHub Release: push тега `v*.*.*` собирает архивы в **черновик**; Publish — вручную. Повтор не трогает immutable-ассеты. См. [`docs/dev.md`](docs/dev.md).
- Стратегии: fake `bin:tls_clienthello_max_ru.bin` → `bin:tls_clienthello_www_sferum_ru.bin` в 16 `strategies/*.json` (ALT, ALT3, ALT4, ALT8, ALT10, ALT11, ALT11 Photon, ALT12, EXP, FAKE TLS AUTO*, SIMPLE FAKE*), как upstream 93ebbbf. Файл `tls_clienthello_max_ru.bin` остаётся в `bin/` для **Fake…**.
- ZapretSpec (Go), winws2: нет неявного TLS hello `!` для групп без `fake.tls`. ALT3, ALT8, FAKE TLS AUTO ALT/ALT2/ALT3 пишут `"tls": ["!"]` явно (на winws это `--dpi-desync-fake-tls=!`, тот же встроенный hello). Профили `payload: "all"` с UDP-fake больше не шлют google TLS hello.
- ZapretSpec (Go), winws2: в группах из нескольких слотов (`payload: "all"`, `discord,stun`) каждый fake идёт только на свой тип пакета, как у winws (`payload=` слота у `--lua-desync=fake`). Раньше все fake шли на все известные payload, а `unknown` / `unknownUdp` — не на неизвестные пакеты. Явный `luaPayload` заменяет эти фильтры (Sky, Sker). Меняется argv winws2 всех стратегий.
- ZapretSpec (Go), winws2: `tls_mod` только на последнем ref слота `tls` и только на вызове для `tls_client_hello`, как `--dpi-desync-fake-tls-mod` у winws. Без слота `tls` `tls_mod` не пишется (раньше шёл на `!` любого слота). Argv стратегий не меняется.
- ZapretSpec (Go), `badseq`: без `badseqIncrement` winws не получает `--dpi-desync-badseq-increment` (дефолт winws −10000 seq / −66000 ack; раньше писалось −10). На winws2 `tcp_seq` = `badseqIncrement` или −10000, `tcp_ack` всегда −66000, как badack winws. Меняется argv FAKE TLS AUTO (оба движка) и winws2 ALT4, ALT8, FAKE TLS AUTO ALT/ALT2, SIMPLE FAKE ALT.
- ZapretSpec (Go): одна проверка JSON для обоих движков, имена с учётом регистра. `engine` — ровно `winws` / `winws2`. Ошибка: нет `profiles` или список пуст; `desync` / `fooling` / `l7` / префикс пути не в нижнем регистре или неизвестны (`fooling` раньше проверялся только на winws2); `cutoff` не `nN` / `none`; `l7` с `quic` и `tls`; пустой домен в списке; fake не `!` / `0x<hex>` / путь с префиксом или с пробелом; `seqovlPattern` не путь (раньше winws2 принимал любой). Argv стратегий не меняется.
- Сток из upstream (Flowseal) по 2026-11: `lists/list-exclude.txt` (+`lesta.pw`), `lists/list-google.txt` (+`fonts.googleapis.com`, `www.gstatic.com`), `.service/ipset-service.txt` (источник **Скачать ipset**), `.service/hosts` (**Сверить hosts**).

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
