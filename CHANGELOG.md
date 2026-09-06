# Changelog

Как пользоваться сейчас — [`docs/usage.md`](docs/usage.md), [`AGENTS.md`](AGENTS.md), [`docs/dev.md`](docs/dev.md). Этот файл — только дельты.

## Unreleased

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
