# Changelog

Секции: Added / Changed / Removed / Breaking. Политика alpha — [`AGENTS.md`](AGENTS.md).

## Unreleased

### Breaking

- GUI: `zapret.bat` → [`zapman.bat`](zapman.bat). Продукт — **Zapret Manager**; zapret остаётся движком (служба `zapret`, `winws` / `winws2`).
- Модуль: [`src/Zapman/Zapman.psd1`](src/Zapman/Zapman.psd1). Движок — [`src/Zapret/Bypass.ps1`](src/Zapret/Bypass.ps1) (dotsource, не отдельный Import-Module).
- Функции обвязки: `*-Zapret*` → `*-Zapman*` (config, UI, тесты, диагностика). Winws / служба / стратегии / фильтры остаются `*-Zapret*`. Без алиасов.
- Свои списки и рабочий ipset: `lists/*-user.txt` / `lists/ipset-all.txt` → `user/` (gitignore). Старый путь не читается. При обновлении копируют папку `user/` в новую распаковку. [`config.json`](config.json) остаётся в корне.
- Стратегии: `strategies/*.ps1` → [`strategies/*.json`](strategies/). Argv собирает [`src/ZapretSpec/`](src/ZapretSpec/) (`ModuleVersion` 0.1.0, поле `specVersion`). Ручные `$argList` / `$argListWinws2` удалены.

### Added

- WPF GUI (`zapret.bat`, [`src/gui/gui.ps1`](src/gui/gui.ps1) + [`src/gui/*.xaml`](src/gui/)): стратегии, служба, фильтры, Tools/Settings, тесты в окне.
- Консоль: [`cli.bat`](cli.bat) / [`src/cli/cli.ps1`](src/cli/cli.ps1) — меню или `service` / `tests` / `env`.
- Модуль [`src/ZapretSpec/`](src/ZapretSpec/) (`ZapretSpec.psd1`): JSON-стратегия → argv winws / winws2. Своя `ModuleVersion`.
- Линтер: [`dev/lint.ps1`](dev/lint.ps1) (ловушки 5.1/WPF + PSScriptAnalyzer + [Blinter](https://pypi.org/project/Blinter/) через `pipx`, [`blinter.ini`](blinter.ini)).
- [`dev/update-zapret.ps1`](dev/update-zapret.ps1): скачать официальные релизы bol-van/zapret и bol-van/zapret2 в `bin/` и переписать [`bin/versions.json`](bin/versions.json).
- [`AGENTS.md`](AGENTS.md), [`PLAN.md`](PLAN.md).
- [`.githooks/commit-msg`](.githooks/commit-msg): `Co-authored-by: Cursor` → `Assisted-by`. Включение: `git config core.hooksPath .githooks`.
- **Стратегия…** / `cli.bat service`: **Ни одна не подходит** запускает `netsh winsock reset`, `netsh int ip reset all`, `netsh winhttp reset proxy`, `ipconfig /flushdns` и спрашивает про перезагрузку.
- Docs: [`docs/usage.md`](docs/usage.md), [`docs/troubleshooting.md`](docs/troubleshooting.md), [`docs/dev.md`](docs/dev.md). Макеты окон — [`docs/ui/`](docs/ui/index.md) из [`dev/export-gui-docs.ps1`](dev/export-gui-docs.ps1).
- Сторож в трее: [`src/Zapman/Tray.ps1`](src/Zapman/Tray.ps1), задание `zapman-tray` при входе (`ONLOGON` / highest). `config.json` `trayWatch` (дефолт true). Opt-out в Settings / `cli.bat service`. Меню: статус (три строки), открыть GUI, Старт службы, Стоп (ненужный пункт гасится). **Снять службы** задание не трогает.
- Всплывающая подсказка трея: полный статус (до 127 символов, три строки). Свойство `NotifyIcon.Text` обрезает на 63.
- Issue-форма [`.github/ISSUE_TEMPLATE/`](.github/ISSUE_TEMPLATE/): чеклист и поля (диагностика, стратегия, фильтры, тесты). Пустые Issue выключены.
- Статус (клик по блоку Status): stderr/stdout и код выхода winws / winws2 после неудачного старта. Баннер указывает на полный текст.

### Changed

- Два входа в корне: `zapret.bat` (один `powershell.exe -STA`) и `cli.bat` (после проверки — PowerShell). `gui.bat` удалён.
- GUI: стратегии в «Стратегия…»; на главном Старт / Стоп / Снять / статус. RU/EN. `config.json` в корне, логи — `test-results/`. Нет `last_strategy.txt`.
- Тесты: `Invoke-ZapretStrategyTests` в процессе (GUI и консоль). `cli.bat tests` отдаёт ненулевой код, если прогон не стартовал или без результатов.
- Служба и IPSet — одна реализация в `src/Zapret/Bypass.ps1`.
- Обвязка: `utils/` → `src/utils/` и `src/Zapret/` без shim.
- Движки: целимся в `winws` и `winws2` сразу (не «заменить и забыть»). Выбор в окне стратегий (`config.json` `engine`). Идентичность — намерение и тесты. Argv из JSON собирает ZapretSpec.
- `general (ALT11)`: эталон таблицы перевода z1→z2 (cutoff `nN` / `--out-range=-dN`, fooling `ts` → `tcp_ts=-600000`).
- winws2 `--blob`: `name:@file` / `name:0x00` (двоеточие, как у zapret2), не `name=@file`. Имя blob не может быть `0x…` (zapret2 `item_name`). Hex в JSON (`0x00000000`) → `--blob=hex_00000000:0x00000000`.
- winws: `--dpi-desync-fake-tls-mod` после `--dpi-desync-fake-tls` (как у zapret: mod вешается на последний fake). Иначе `0x00000000` + `tls-mod` даёт `could not mod tls`.
- z2 `!` (встроенный ClientHello у z1): stock `bin:tls_clienthello_www_google_com.bin`. `tls_mod` только на последний TLS-fake профиля.
- Движки из официальных релизов: zapret **v72.13**, zapret2 **v1.0.4**. Хеши в [`bin/versions.json`](bin/versions.json); сверка при старте движка (файлы выбранного `engine`) и в `cli.bat env`. Lua сравнивается как LF, чтобы `core.autocrlf` на Windows не блокировал старт.
- Тесты гоняют текущий `engine`: JSON через ZapretSpec и `Start-ZapretStrategyFile` в том же процессе (не `powershell -File`). При сбое — тот же stderr/exit, что у «Запустить без установки». GUI качает диспетчер на ожидании.
- Служба winws2: в ImagePath пишется `--chdir` на `bin/` (SCM стартует из System32, иначе нет Lua). ImagePath собирается из шаблона стратегии, не из WMI.
- Статус / баннер показывают живой процесс (`winws` или `winws2`), не зашитое имя.
- Линтер: `src/utils/lint.ps1` → [`dev/lint.ps1`](dev/lint.ps1).
- Рабочий `user/ipset-all.txt` не в git. Заготовка — [`lists/ipset-all.default.txt`](lists/ipset-all.default.txt): нет файла — копия в `user/`, есть — оставляем.
- Settings: «Подменить fake» → «Fake…» / «Fakes…». В окне сразу оба слота (Discord UDP и Game UDP) с текущим файлом. После записи — тот же вопрос, что у IPSet (Stop/Start или повторный запуск стратегии).
- `zapret.bat`: просто запускает GUI. Нет `where` / `start` / Hidden. Проверки среды и SHA256 не на старте окна — ошибка при реальном сбое (нет WPF, старт движка). Вход — [`src/gui/gui-boot.ps1`](src/gui/gui-boot.ps1) → [`src/gui/gui.ps1`](src/gui/gui.ps1). UAC — `Start-Process -Verb RunAs` на `powershell.exe`.
- Раскладка: GUI — [`src/gui/`](src/gui/), консоль — [`src/cli/`](src/cli/). `test zapret.ps1` → [`src/cli/test-zapret.ps1`](src/cli/test-zapret.ps1). `src/utils/` удалён, без shim.
- GUI: статус и иконка — после первой отрисовки.
- Список стратегий не собирает argv: поддержка движка — разбор JSON. Argv кэшируется.
- Диалоги GUI вынесены в [`src/gui/gui-dialogs.ps1`](src/gui/gui-dialogs.ps1) (dotsource из `gui.ps1`). Статус-дамп и подсказки фильтров — ключи [`src/Zapret/Ui.ps1`](src/Zapret/Ui.ps1). Битый `config.json` даёт одну строку в статусе / `cli.bat env`, дефолты те же.
- Тесты: служба zapret снимается без WinDivert и ставится снова после прогона. «Стратегия…»: слева **Установить службу**, справа **Запустить без установки**; **Прогнать тесты** и **Ни одна не подходит** — отдельные строки. Окно тестов выше: таблица целей по умолчанию без прокрутки; подготовка, вкладки стратегий (ошибка — красная ячейка, текст ошибки копируется), итоги. Сбой старта движка пишется в `test-results/`.
- GUI старт: `gui-boot.ps1` сразу скрывает консоль, поднимает UAC и показывает каркас `Main.xaml`. Разбор `gui.ps1` и импорт модуля идут при уже видимом окне. Журнал старта: hide-console / shell / parse / import / build-form / shown / idle.
- Issue-форма: пустые поля видны как `(не заполнено)` / `(не выбран)` / `v0.0.0`, а не как живые примеры (`v0.1.0`, `general (ALT11)`).
- Набор стратегий поддерживается в согласовании с [Flowseal/zapret-discord-youtube](https://github.com/Flowseal/zapret-discord-youtube). Игры: Discussions этого репозитория и [оригинального](https://github.com/Flowseal/zapret-discord-youtube/discussions).
- **Сверить hosts:** если файл устарел — диалог **Копировать шаблон** / **Открыть hosts** (системный файл в Блокноте; можно сохранить).
- Hosts: GitHub (`githubusercontent.com`) — актуальные IPv4/IPv6 из upstream [#17088](https://github.com/Flowseal/zapret-discord-youtube/commit/6cec828910d0809863205702182a3557d9d0e8c3).
- README: только старт, предупреждения и ссылки на [`docs/`](docs/). FAQ и список файлов — в docs.
- Релиз: архив `zapman-<tag>` (`zapman.bat`, `cli.bat`, `bin/`, `lists/`, `strategies/`, `src/`, `docs/`). Lint CI — `windows-latest` + inbox `powershell.exe -STA`.
- Версия продукта: `ModuleVersion` в [`src/Zapman/Zapman.psd1`](src/Zapman/Zapman.psd1) (`0.1.0`). GUI/CLI показывают тег `v0.1.0`. Auto-Update Check сравнивает с GitHub tags. Релизный тег — `v` + `ModuleVersion`.
- Генератор стратегий: [`src/ZapretSpec/`](src/ZapretSpec/) со своей `ModuleVersion`. `cli.bat env` показывает продукт и spec.

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
- Автокомментарий к новому Issue. Чеклист и поля — в форме Issue.
- `.github/github-bot.yml` (trusted_users чужого бота).
- `.service/version.txt`. Версия только в `ModuleVersion` [`src/Zapman/Zapman.psd1`](src/Zapman/Zapman.psd1).

### Breaking

- GUI на inbox WPF, не WinForms. `Test-ZapretHostReady` (`cli.bat env`) проверяет `Test-ZapretWpf`, не `Test-ZapretWinForms`.
- Трея нет: нет иконки в области уведомлений, нет «открыть из трея».
- В корне `zapret.bat` и `cli.bat`. Удалены `gui.bat`, `service.bat`, корневые `general*.bat`. Стратегии: `strategies/*.json`.
- Совместимость только у набора стратегий — [`AGENTS.md`](AGENTS.md). Движки: оба на неопределённый срок, выбор `winws` / `winws2` в Settings — [`PLAN.md`](PLAN.md).
