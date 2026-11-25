# Changelog

Как пользоваться сейчас — [`docs/usage.md`](docs/usage.md), [`AGENTS.md`](AGENTS.md), [`docs/dev.md`](docs/dev.md). Этот файл — только дельты.

## Unreleased

### Added

- Sky в `general (ALT11 Photon)`: UDP fake с TTL 8 для игровых IP и TCP multisplit для WebSocket чата/заданий; `user/ipset-sky.txt` и его обновление из игрового лога через трей. Игровая проверка выполнена на winws2.
- Sker Ritual в `general (ALT11 Photon)`: профиль UDP Photon с fake TTL 8 для подсетей из `user/ipset-sker.txt` (копия `lists/ipset-sker.default.txt`) перед общим профилем Photon. Общий профиль выкидывал из матча по `ClientTimeout`. Матч и голос проверены на winws2. Тот же профиль и подсеть Vultr `95.179.160.0/20` — для Hazard Levels.
- Поля ZapretSpec `luaPayload`, `ttl`, `defaultTlsFake` и `cutoff: "none"` для переноса проверенных профилей Sky без ручной правки ImagePath службы.
- Макет главного окна в [`README.md`](README.md) ([`docs/ui/Main.svg`](docs/ui/Main.svg)).
- Fake из upstream в `bin/` (видны в **Fake…**): `tls_clienthello_www_sferum_ru`, `tls_clienthello_awswaf_com`, `tls_clienthello_steam_chat_com`, `tls_clienthello_steamstatic_com`, `tls_clienthello_teamspeak_com`, `quic_initial_lemana_pro`, `quic_initial_steam_chat_com`, `quic_initial_steamstatic_com`.

### Changed

- GitHub Release: push тега `v*.*.*` собирает архивы в **черновик**; Publish — вручную. Повтор не трогает immutable-ассеты. См. [`docs/dev.md`](docs/dev.md).
- Стратегии: fake `bin:tls_clienthello_max_ru.bin` → `bin:tls_clienthello_www_sferum_ru.bin` в 15 `strategies/*.json` (ALT, ALT3, ALT4, ALT8, ALT10, ALT11, ALT12, EXP, FAKE TLS AUTO*, SIMPLE FAKE*), как upstream 93ebbbf. Файл `tls_clienthello_max_ru.bin` остаётся в `bin/` для **Fake…**.
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
