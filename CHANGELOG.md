# Changelog

Секции: Added / Changed / Removed / Breaking. Политика alpha — [`AGENTS.md`](AGENTS.md).

## Unreleased

### Breaking

- GUI: `zapret.bat` → [`zapman.bat`](zapman.bat). Продукт — **Zapret Manager**; zapret остаётся движком (служба `zapret`, `winws` / `winws2`).
- Модуль: [`src/Zapman/Zapman.psd1`](src/Zapman/Zapman.psd1). Движок — [`src/Zapret/Bypass.ps1`](src/Zapret/Bypass.ps1) (dotsource, не отдельный Import-Module).
- Функции обвязки: `*-Zapret*` → `*-Zapman*` (config, UI, тесты, диагностика). Winws / служба / стратегии / фильтры остаются `*-Zapret*`. Без алиасов.

### Added

- WPF GUI (`zapret.bat`, [`src/gui/gui.ps1`](src/gui/gui.ps1) + [`src/gui/*.xaml`](src/gui/)): стратегии, служба, фильтры, Tools/Settings, тесты в окне.
- Консоль: [`cli.bat`](cli.bat) / [`src/cli/cli.ps1`](src/cli/cli.ps1) — меню или `service` / `tests` / `env`.
- Модуль [`src/Zapret/`](src/Zapret/) (`Zapret.psd1`).
- Линтер: [`dev/lint.ps1`](dev/lint.ps1) (ловушки 5.1/WPF + PSScriptAnalyzer + [Blinter](https://pypi.org/project/Blinter/) через `pipx`, [`blinter.ini`](blinter.ini)).
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
- Движки из официальных релизов: zapret **v72.13**, zapret2 **v1.0.4**. Хеши в [`bin/versions.json`](bin/versions.json); сверка при старте движка и в `cli.bat env`.
- Тесты гоняют текущий `engine`: только стратегии с флагами этого движка; ждут `winws` или `winws2`.
- Служба winws2: в ImagePath пишется `--chdir` на `bin/` (SCM стартует из System32, иначе нет Lua). ImagePath собирается из шаблона стратегии, не из WMI.
- Статус / баннер показывают живой процесс (`winws` или `winws2`), не зашитое имя.
- Линтер: `src/utils/lint.ps1` → [`dev/lint.ps1`](dev/lint.ps1).
- Рабочий `lists/ipset-all.txt` не в git. Заготовка — [`lists/ipset-all.default.txt`](lists/ipset-all.default.txt): нет файла — копируем, есть — оставляем.
- Settings: «Подменить fake» → «Fake…» / «Fakes…». В окне сразу оба слота (Discord UDP и Game UDP) с текущим файлом. После записи — тот же вопрос, что у IPSet (Stop/Start или повторный запуск стратегии).
- `zapret.bat`: просто запускает GUI. Нет `where` / `start` / Hidden. Проверки среды и SHA256 не на старте окна — ошибка при реальном сбое (нет WPF, старт движка). Вход — [`src/gui/gui-boot.ps1`](src/gui/gui-boot.ps1) → [`src/gui/gui.ps1`](src/gui/gui.ps1). UAC — `Start-Process -Verb RunAs` на `powershell.exe`.
- Раскладка: GUI — [`src/gui/`](src/gui/), консоль — [`src/cli/`](src/cli/). `test zapret.ps1` → [`src/cli/test-zapret.ps1`](src/cli/test-zapret.ps1). `src/utils/` удалён, без shim.
- GUI: статус и иконка — после первой отрисовки.
- Диалоги GUI вынесены в [`src/gui/gui-dialogs.ps1`](src/gui/gui-dialogs.ps1) (dotsource из `gui.ps1`). Статус-дамп и подсказки фильтров — ключи [`src/Zapret/Ui.ps1`](src/Zapret/Ui.ps1). Битый `config.json` даёт одну строку в статусе / `cli.bat env`, дефолты те же.
- Тесты: служба zapret снимается без WinDivert и ставится снова после прогона. «Стратегия…»: слева **Установить службу**, справа **Запустить без установки**.
- GUI старт: `gui-boot.ps1` сразу скрывает консоль, поднимает UAC и показывает каркас `Main.xaml`. Разбор `gui.ps1` и импорт модуля идут при уже видимом окне. Журнал старта: hide-console / shell / parse / import / build-form / shown / idle.

### Removed

- Трей (`NotifyIcon`, Open / Exit). Крестик закрывает GUI; свернуть — панель задач.
- WinForms GUI `src/utils/gui.ps1`. Вход — [`src/gui/gui.ps1`](src/gui/gui.ps1).
- Скрипты из `src/utils/`: boot → [`src/gui/gui-boot.ps1`](src/gui/gui-boot.ps1); консоль → [`src/cli/`](src/cli/).
- `utils/engine.ps1`, `utils/Zapret/`, корневой `utils/`.
- `last_strategy.txt`, `game_filter.enabled`, `check_updates.enabled`, `ui_lang.txt`, `targets.txt`.
- `src/Zapret/config.json`, `src/utils/test-results/`.
- `src/utils/lint.ps1` (теперь [`dev/lint.ps1`](dev/lint.ps1)).
- [`src/cli/check-env.ps1`](src/cli/check-env.ps1); `cli.bat env` по-прежнему в [`src/cli/cli.ps1`](src/cli/cli.ps1).
- [`src/gui/DiagRun.xaml`](src/gui/DiagRun.xaml) (диагностика собирает строки в коде).
- `Start-ZapretConfigTests` (GUI и консоль зовут `Invoke-ZapretStrategyTests` в процессе).

### Breaking

- GUI на inbox WPF, не WinForms. `Test-ZapretHostReady` (`cli.bat env`) проверяет `Test-ZapretWpf`, не `Test-ZapretWinForms`.
- Трея нет: нет иконки в области уведомлений, нет «открыть из трея».
- В корне `zapret.bat` и `cli.bat`. Удалены `gui.bat`, `service.bat`, корневые `general*.bat`. Стратегии: `strategies/*.ps1`.
- Совместимость только у набора стратегий — [`AGENTS.md`](AGENTS.md). Движки: оба на неопределённый срок, выбор `winws` / `winws2` в Settings — [`PLAN.md`](PLAN.md).
