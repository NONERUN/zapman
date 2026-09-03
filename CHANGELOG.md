# Changelog

Секции: Added / Changed / Removed / Breaking. Политика alpha — [`AGENTS.md`](AGENTS.md).

## Unreleased

### Added

- WinForms GUI (`zapret.bat`, `src/utils/gui.ps1`): стратегии, служба, фильтры, Tools/Settings, тесты в окне, трей.
- Консоль: [`cli.bat`](cli.bat) / [`src/utils/cli.ps1`](src/utils/cli.ps1) — меню или `service` / `tests` / `env`.
- Модуль [`src/Zapret/`](src/Zapret/) (`Zapret.psd1`).
- Линтер: [`dev/lint.ps1`](dev/lint.ps1) (ловушки 5.1/WinForms + PSScriptAnalyzer + [Blinter](https://pypi.org/project/Blinter/) через `pipx`, [`blinter.ini`](blinter.ini)).
- [`dev/update-zapret.ps1`](dev/update-zapret.ps1): скачать официальные релизы bol-van/zapret и bol-van/zapret2 в `bin/` и переписать [`bin/versions.json`](bin/versions.json).
- [`AGENTS.md`](AGENTS.md), [`PLAN.md`](PLAN.md).
- [`.githooks/commit-msg`](.githooks/commit-msg): `Co-authored-by: Cursor` → `Assisted-by`. Включение: `git config core.hooksPath .githooks`.

### Changed

- Два входа в корне: `zapret.bat` (один `powershell.exe -STA`) и `cli.bat` (после проверки — PowerShell). `gui.bat` удалён.
- GUI: стратегии в «Стратегия…»; на главном Старт / Стоп / Снять / статус. RU/EN. `config.json` в корне, логи — `test-results/`. Нет `last_strategy.txt`.
- Тесты: `Invoke-ZapretStrategyTests` в процессе (GUI и консоль). `cli.bat tests` отдаёт ненулевой код, если прогон не стартовал или без результатов.
- Служба и IPSet — одна реализация в `src/Zapret/Bypass.ps1`.
- Обвязка: `utils/` → `src/utils/` и `src/Zapret/` без shim.
- Движки: целимся в `winws` и `winws2` сразу (не «заменить и забыть»). Выбор в окне стратегий (`config.json` `engine`). Идентичность — намерение и тесты, не пакеты и не компилятор Lua→z1.
- `general (ALT11)`: в том же файле флаги winws и winws2 (`$argListWinws` / `$argListWinws2`).
- Движки из официальных релизов: zapret **v72.13**, zapret2 **v1.0.4**. Хеши в [`bin/versions.json`](bin/versions.json); GUI и CLI сверяют SHA256 при старте.
- Тесты гоняют текущий `engine`: только стратегии с флагами этого движка; ждут `winws` или `winws2`.
- Служба winws2: в ImagePath пишется `--chdir` на `bin/` (SCM стартует из System32, иначе нет Lua). ImagePath собирается из шаблона стратегии, не из WMI.
- Статус / трей / баннер показывают живой процесс (`winws` или `winws2`), не зашитое имя.
- Линтер: `src/utils/lint.ps1` → [`dev/lint.ps1`](dev/lint.ps1).
- Рабочий `lists/ipset-all.txt` не в git. Заготовка — [`lists/ipset-all.default.txt`](lists/ipset-all.default.txt): нет файла — копируем, есть — оставляем.

### Removed

- `utils/engine.ps1`, `utils/Zapret/`, корневой `utils/`.
- `last_strategy.txt`, `game_filter.enabled`, `check_updates.enabled`, `ui_lang.txt`, `targets.txt`.
- `src/Zapret/config.json`, `src/utils/test-results/`.
- `src/utils/lint.ps1` (теперь [`dev/lint.ps1`](dev/lint.ps1)).

### Breaking

- В корне `zapret.bat` и `cli.bat`. Удалены `gui.bat`, `service.bat`, корневые `general*.bat`. Стратегии: `strategies/*.ps1`.
- Совместимость только у набора стратегий — [`AGENTS.md`](AGENTS.md). Движки: оба на неопределённый срок, выбор `winws` / `winws2` в Settings — [`PLAN.md`](PLAN.md).
