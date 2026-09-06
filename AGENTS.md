# Инструкции для агентов

**Zapret Manager** (`zapman`) запускает [zapret](https://github.com/bol-van/zapret) (`winws.exe`) и [zapret2](https://github.com/bol-van/zapret2) (`winws2.exe`) из `bin/`. Какие поля JSON дают какие флаги — [`docs/dev.md`](docs/dev.md).

В корне два `.bat`: `zapman.bat` (GUI), `cli.bat` (консоль). Третий `.bat` не добавлять. Новая логика — только `.ps1`.

Текущий интерфейс — этот файл, [`docs/usage.md`](docs/usage.md), [`docs/dev.md`](docs/dev.md). [`CHANGELOG.md`](CHANGELOG.md) — дельты, не описание «как сейчас».

## Не делать

- Не читать `lists/*-user.txt`, старый `lists/ipset-all.txt` и корневой `config.json`. Свои списки и настройки — только `user/`.
- Не оставлять `foo.bat` рядом с `foo.ps1`. Не добавлять алиас `*-Zapret*` для `*-Zapman*`.
- Не держать второй `strategies/*.json` «для winws2». Один JSON, два argv из ZapretSpec.
- Не разбирать строку флагов в GUI, CLI или `Bypass.ps1`. Argv собирает только [`src/ZapretSpec/`](src/ZapretSpec/).
- Не добавлять чтение второго файла «если первого нет», тихий fallback на другой `engine`, предупреждение об устаревании, которое оставляет оба поведения.
- Не копировать политику 5.1 / стратегий в комментарии каждого `.ps1`. Комментарии в коде — [ASD-STE100](https://www.asd-ste100.org/) Simplified Technical English.
- Не ставить зависимость, без которой GUI на Windows 10 LTSC не открывается: нет требования Visual Studio, .NET SDK, `pwsh`, WinUI.
- Не писать атаки вне списка в [`docs/dev.md`](docs/dev.md). Не подменять чужой `specVersion` другим числом.

Новые стратегии не выдумывать. Смысл атак брать из [Flowseal/zapret-discord-youtube](https://github.com/Flowseal/zapret-discord-youtube). Свой набор файлов — только после явного решения в чате или issue.

Смена имени `strategies/*.json` — строка в CHANGELOG Changed (`старое` → `новое`). Удаление стратегии без преемника — Breaking. Документацию (`README.md`, этот файл, `docs/`, примеры в `lists/`, JSON стратегий) править в том же изменении, что и код.

`bin/` всегда содержит и `winws.exe`, и `winws2.exe`. Нет exe выбранного `engine` — ошибка, не переход на другой exe.

## Среда выполнения

Интерпретатор продукта — **Windows PowerShell 5.1** (`powershell.exe`), не PowerShell 7 (`pwsh`). Не писать `??`, `?:`, `&&` / `||`, `-Parallel`.

Таблица — что **запускает** продукт. Разработка, агент и `dev\lint.ps1` (STA + `XamlReader.Load`) могут быть на более новой десктопной сборке. Машину агента не считать LTSC.

| Платформа | Ожидание |
|---|---|
| Windows 10 LTSC (и новее с рабочим столом) | Запуск без допнастроек: inbox PS 5.1, .NET 4.x, WPF (`PresentationFramework`), CIM |
| Windows 7 SP1 x64 | WMF 5.1 и .NET 4.5+ |
| PowerShell 7 | Не цель |

## Код (ловушки 5.1)

Уже ломали GUI:

- `return @(...)` разворачивает массив из 0/1 элемента. Для `.Count` оборачивайте в `@()` **в месте использования**.
- `Set-StrictMode -Version Latest` запрещает обращение к несуществующим свойствам.
- `Get-Item HKLM:\...` открывает ключ только на чтение. Для записи — `OpenSubKey(..., $true)`.
- `$PSScriptRoot`, `Get-CimInstance`, `-LiteralPath` требуют минимум PowerShell **3.0**.
- WPF нужен **STA**: `powershell.exe -STA`. Вход — `zapman.bat`.
- Не писать `New-Object System.Collections.Generic.List[object]`. `List[string]` можно.

[`src/Zapman/Ui.ps1`](src/Zapman/Ui.ps1) с кириллицей — **UTF-8 с BOM**. Без BOM PowerShell 5.1 ломает литералы. XAML в [`src/gui/`](src/gui/) — ASCII; тексты через `Get-ZapmanUiString`.

## Линтер

После правок `.ps1`, `.bat`, `src/gui/`, `src/cli/`, `src/Zapman/`, `src/Zapret/`, `src/ZapretSpec/`, `strategies/` или `dev/`:

```text
powershell.exe -NoProfile -ExecutionPolicy Bypass -File dev\lint.ps1
```

Все FAIL исправить до конца задачи. После правки XAML или строк UI ещё:

```text
powershell.exe -NoProfile -ExecutionPolicy Bypass -File dev\export-gui-docs.ps1
```

`dev/lint.ps1` всегда ловит:

- `List[object]`; у главного окна нет `$window.IsEnabled = $false` / `Hide()` / `ShowDialog()` / `ShowInTaskbar = $false`
- нет Forms в `src/gui/`
- XAML только файлом (`XamlReader.Load`, `xmlns:x` если есть `x:`, без `x:Class`)
- BOM у кириллицы / `Ui.ps1`
- синтаксис `pwsh`
- `-STA` в `zapman.bat`
- `docs/ui` совпадает с генератором (`dev\export-gui-docs.ps1 -Check`)
- `strategies/*.json`: `specVersion` и argv обоих движков; нет `--dpi-desync` на z2 и `--lua-desync` на z1; z2 `--blob=name:value`; имя blob не `0x…`; нет `blob=empty`; z2 `tls_mod` только вместе с `blob=`; winws: `fake-tls` до `fake-tls-mod`; z1 `fakedsplit-pattern` с `bin:` — путь

Опционально: PSScriptAnalyzer; Blinter для `.bat` (`pipx install Blinter`, [`blinter.ini`](blinter.ini)).

## Раскладка

### Входы

| Путь | Роль |
|---|---|
| `zapman.bat` | GUI: inbox `powershell.exe -STA` → `src/gui/gui-boot.ps1` → `src/gui/gui.ps1`. UAC — `Start-Process -Verb RunAs` на inbox `powershell.exe`. Без предстартовых проверок: ошибка остаётся в этом окне |
| `cli.bat` | Консоль: проверка `powershell.exe`, затем это окно — PowerShell (`src/cli/cli.ps1`) |

### GUI

| Путь | Роль |
|---|---|
| `src/gui/gui-boot.ps1` | Короткий `-File`: скрыть консоль, UAC (этот файл), показать `Main.xaml` с [`app.ico`](src/gui/app.ico), затем разбор `gui.ps1`. `Application.Run` после разбора `gui.ps1` |
| `src/gui/gui.ps1` | WPF code-behind. Разметка — соседние `*.xaml`. Диалоги — `gui-dialogs.ps1` (dotsource) |

### Консоль

| Путь | Роль |
|---|---|
| `src/cli/cli.ps1` | Меню или `service` / `tests` / `env` в том же процессе |
| `src/cli/service.ps1` | Консольное меню. Elevate — этот файл, не `cli.ps1` |
| `src/cli/test-zapret.ps1` | `cli.bat tests` → `Invoke-ZapmanStrategyTests`. Exit ≠ 0: нет прав, нет `curl.exe`, нет JSON на выбранный `engine`, нет завершённых результатов. Прогон прошёл (в том числе все ячейки красные) — exit 0 |

Служба, ImagePath, IPSet, Game Filter, старт winws — [`src/Zapret/Bypass.ps1`](src/Zapret/Bypass.ps1). Пункты меню CLI могут отличаться от GUI (UAC, когда пишется `engine`). Не копировать UX GUI в CLI «чтобы было одинаково» без задачи.

### Модуль и данные

| Путь | Роль |
|---|---|
| `src/Zapman/` | Модуль (`Zapman.psd1`): config, UI, тесты, диагностика, трей. GUI и консоль импортируют его. Тег продукта — `v` + `ModuleVersion`. Auto-Update Check сравнивает с GitHub tags. [`Tray.ps1`](src/Zapman/Tray.ps1) — отдельный процесс (`-File`), не dotsource в `Zapman.psm1` |
| `src/ZapretSpec/` | JSON → argv winws / winws2. Своя `ModuleVersion`. Zapman грузит по пути. Не стартует winws |
| `src/Zapret/` | `Bypass.ps1`: winws, служба `zapret`, стратегии, фильтры, ipset, fake. Dotsource из Zapman, не отдельный модуль |
| `test-results/` | Логи тестов (на лету) |
| `strategies/*.json` | Поля стратегии (`specVersion` = ZapretSpec). Сток — `lists:`, свои файлы — `user:`, fake — `bin:` |
| `lists/` | Сток: `list-general.txt`, `list-exclude.txt`, `list-google.txt`, `ipset-exclude.txt`, `ipset-all.default.txt` |
| `user/` | Свои списки, рабочий `ipset-all.txt`, `user/config.json` (язык, `engine`, фильтры, Auto-Update Check, `trayWatch`). Списки создаются при старте, gitignore. Корневой `config.json` не читать. При обновлении копируют папку в новую распаковку |
| `docs/` | `usage.md`, `troubleshooting.md`, `dev.md`. Макеты — `docs/ui/` (генератор, не править руками) |
| `bin/` | `winws.exe`, `winws2.exe` + `lua/`, общий WinDivert. Теги и SHA256 — [`bin/versions.json`](bin/versions.json). GUI не сверяет при открытии окна; сверка при старте выбранного `engine` и в `cli.bat env`. Lua в хеше как LF |

### Dev

| Путь | Роль |
|---|---|
| `dev/lint.ps1` | 5.1/WPF + PSSA + Blinter; JSON через ZapretSpec; `-Check` макетов `docs/ui` |
| `dev/export-gui-docs.ps1` | XAML + `Ui.ps1` → `docs/ui/*.svg` и `index.md` / `index.html` |
| `dev/update-zapret.ps1` | Официальные zip zapret / zapret2 → `bin/` + [`bin/versions.json`](bin/versions.json). Fake `.bin` не трогает |
| `.github/workflows/release.yml` | GitHub Release: dispatch, checkout **существующего** тега, zip с этого дерева. Шаги — [`docs/dev.md`](docs/dev.md) |
| `.githooks/commit-msg` | `Co-authored-by: Cursor` → `Assisted-by`. Включение: `git config core.hooksPath .githooks` |

## Служба и фильтры

Код — `src/Zapret/Bypass.ps1` (вызывают GUI и `src/cli/service.ps1`).

- **Снять службы:** сначала `zapret` (дождаться Stopped), потом процессы `winws` / `winws2`, потом WinDivert / WinDivert14. Иначе драйвер зависает в `STOP_PENDING`. Задание `zapman-tray` не удалять.
- **Install:** argv, pin (`bin/versions.json`) и ImagePath **до** `sc delete`; дождаться удаления; ImagePath из шаблона (`"exe" args`; для winws2 ещё `--chdir` на `bin/`); имя стратегии — значение `zapman` в ключе службы (старое `zapret-discord-youtube` не читать, при записи удалить); проверить реестр; старт **службой**. Не проверять установку запуском процесса с `WorkingDirectory=bin`. Сбой после create — `sc delete` и запись прежнего ImagePath, если он был.
- **Game Filter** и **`engine`** при установленной службе сидят в ImagePath: снова **Установить службу**, не Стоп/Старт.
- **IPSet** читается из `user/ipset-all.txt` при старте winws: достаточно Стоп/Старт. Режимы — в [`docs/usage.md`](docs/usage.md). Смена `loaded` → другое: rename файла в backup, не вечная копия.
- **Fake…** копирует `ACTIVE_*.bin`; winws читает при старте. Как IPSet: Стоп/Старт, не Install.

## GUI

Главное окно (`Main.xaml`): нет списка стратегий. Кнопки **Старт службы** (выключена, если службы нет), **Стоп**, **Снять службы**, **Стратегия…**. Settings — GroupBox на главном, не отдельное окно: `cmbGame`, `cmbIpset`, `chkAuto`, `chkTray`, `btnFakes`. Смена стратегии = Install из «Стратегия…».

**Стратегия…:** список `strategies/*.json`, радио `winws` / `winws2`. Радио пишет `config.json` `engine` только при **Установить службу**, **Запустить без установки** или старте тестов. **Установить службу** закрывает диалог. **Запустить без установки** останавливает службу/winws и стартует JSON (без автозапуска). **Прогнать тесты** снимает службу `zapret` на время прогона (WinDivert не трогает) и ставит снова, если JSON из снимка ещё на диске; нет файла — throw. Нет имени стратегии в реестре — службу не возвращает. **Ни одна не подходит** — `netsh winsock reset`, `netsh int ip reset all`, `netsh winhttp reset proxy`, `ipconfig /flushdns`, затем вопрос про перезагрузку. Список помечает запущенную или установленную.

**Трей:** процесс [`src/Zapman/Tray.ps1`](src/Zapman/Tray.ps1), задание планировщика `zapman-tray` (`ONLOGON`, highest). По умолчанию включено (`config.json` `trayWatch`). Меню: три строки статуса, открыть GUI, Старт службы, Стоп. Старт включён только если служба есть и `Stopped`. Стоп включён если обход идёт или служба не `Stopped`. Стратегию и фильтры меняют только в GUI. Opt-out — `chkTray`.

Язык RU/EN — радио на главном и пункты CLI (`cli.bat` пункт 4, `cli.bat service` пункт 11), строки `Get-ZapmanUiString`. Язык, `engine`, Game Filter, Auto-Update Check, `trayWatch` — `user/config.json`. Цели HTTP/ping тестов — [`src/Zapman/Tests.ps1`](src/Zapman/Tests.ps1) (`Get-ZapmanTestTargets`), не config.

### Окно (WPF)

- Крестик = выход процесса GUI. Свернуть = панель задач. Главное окно в трей не прячется.
- Главный цикл — `Application.Run`, не `$window.ShowDialog()`. Дочерний `ShowDialog` с `Owner` можно. Не `Hide()` и не `$window.IsEnabled = $false` у главного. `ShowInTaskbar` у главного — `True`.
- `gui-boot.ps1` делает `$window.Show()` до разбора `gui.ps1`. Сбой после скрытия консоли — снова показать консоль (`Show-GuiBootConsole`).
- Тесты: не вешать события `Process` (`Exited`, `*DataReceived`) в scriptblock GUI.
