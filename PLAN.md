# План

**alpha.** Ломающие изменения — [`AGENTS.md`](AGENTS.md).

Сейчас: [`zapman.bat`](zapman.bat) (GUI Zapret Manager, один `powershell.exe -STA` → [`src/gui/`](src/gui/)), [`cli.bat`](cli.bat) (после проверки — PowerShell → [`src/cli/cli.ps1`](src/cli/cli.ps1)), обвязка [`src/Zapman/`](src/Zapman/), генератор [`src/ZapretSpec/`](src/ZapretSpec/), движок [`src/Zapret/`](src/Zapret/), стратегии [`strategies/*.json`](strategies/). Третий `.bat` не добавлять.

## Движки (контракт)

Цель на **неопределённый срок** — оба: zapret1 (`winws.exe`) и zapret2 (`winws2.exe` + Lua). Не «сначала один, потом выкинуть второй». Поставка `bin/` — оба exe, общий WinDivert / `cygwin1.dll`, Lua рядом с winws2. Исследование: [`.local/zapret-vs-zapret2.md`](.local/zapret-vs-zapret2.md) (не в git).

1. **Выбор в окне стратегий** (GUI «Стратегия…» и консольное меню): `winws` / `winws2`. Ключ [`config.json`](config.json): `engine`. Дефолт — `winws`. Смена движка как Game Filter: зашита в ImagePath, нужен повторный Install, не Stop/Start.
2. **Одна стратегия — один файл** `strategies/*.json`. Намерение (списки, fake, атаки, числа). [`src/ZapretSpec/`](src/ZapretSpec/) собирает argv для выбранного `engine`. GUI флаги не парсит. Нет exe — отказ, без тихого перехода на другой.
3. **Идентичность.** Гарантируем **намерение**: те же списки / fake / Game Filter и те же имена атак antidpi с теми же числами, где они отображаются. Не гарантируем одинаковые пакеты и одинаковый обход у провайдера (у z2 другие range, payload, empty ACK, автолист, badseq). Сходимость — тесты на **обоих** движках.
4. **Подмножество (заморожено).** Только stock `bin/lua/zapret-antidpi.lua`. Атаки: `fake`, `multisplit`, `multidisorder`, `hostfakesplit`, `fakedsplit`, `syndata`. Fooling: `ts` → z2 `tcp_ts=-600000`; `badseq` + increment → `tcp_seq` / `tcp_ack`; `md5sig` → `tcp_md5`. Неизвестный `desync` — ошибка. Lua → z1 по-прежнему нельзя. Свой Lua не входит. `specVersion` в JSON должен совпадать с `ModuleVersion` ZapretSpec.

## Версии

- **Zapman** `ModuleVersion` — продукт, тег релизов, Auto-Update Check.
- **ZapretSpec** `ModuleVersion` — схема JSON и таблица перевода. Независима от Zapman. `cli.bat env` показывает обе.

## UI (текущее)

Журнал сессии 2026-08-30 (не в git): `.local/ui-ux-session/`. Контракт окна — в [`AGENTS.md`](AGENTS.md) (GUI, WPF). Кратко:

- RU/EN; коды фильтров и имена стратегий не переводить.
- GUI и `cli.bat` равноправны. Консоль: `service` / `tests` / `env`.
- Главное: статус (клик — полный дамп) + Старт / Стоп / Снять / Стратегия… (в т.ч. сброс стека, если ни одна стратегия не подходит).
- Settings: Game Filter, IPSet, Auto-Update Check, иконка в трее (`trayWatch`), Fake… (оба слота). Движок — в «Стратегия…». Tools: ipset, hosts, версия, диагностика.
- Крестик = выход, свернуть = панель задач. Главное окно в трей не прячется. Сторож — [`src/Zapman/Tray.ps1`](src/Zapman/Tray.ps1), задание `zapman-tray` при входе.

Позже: другие языки, resize / 125%+ DPI.

## Осталось по движкам

- `config.json` `engine` из окна стратегий; служба и «запустить стратегию» смотрят выбранный exe. Argv — ZapretSpec из JSON.
- `bin/`: winws из zapret v72.13, winws2 + Lua из zapret2 v1.0.4; хеши в [`bin/versions.json`](bin/versions.json). Обновить: `dev\update-zapret.ps1`.
- Тесты гоняют текущий `engine`: JSON через генератор, `Start-ZapretStrategyFile` в процессе, ждут `winws` или `winws2`.
- Служба winws2: ImagePath из шаблона + `--chdir` на `bin/` (иначе Lua не находится из System32).
- Win7: WinDivert из поставки (2.2.0 при необходимости), общий для обоих exe.

Флаги z1 и Lua — мануалы bol-van, не выдумывать атаки. Таблица перевода — только замороженное подмножество выше.

## Вне скоупа

pwsh / WinUI; совместимость обвязки; свои Lua-атаки без отдельной задачи; произвольный Lua → флаги winws1.
