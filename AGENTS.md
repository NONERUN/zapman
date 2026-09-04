# Инструкции для агентов

**Zapret Manager** (`zapman`) — Windows-обвязка, которая запускает [zapret](https://github.com/bol-van/zapret) / zapret2. Критерии готовности и контракт движков — [`PLAN.md`](PLAN.md).

**Поддерживаемость.** Ясный текущий интерфейс важнее совместимости обвязки, обходных путей и «на всякий случай». Если правка усложняет чтение или правку без необходимости — не делайте её так. Агент и человек опираются только на **текущий** интерфейс.

В корне **два** `.bat`: `zapman.bat` (GUI) и `cli.bat` (консоль). Всё остальное — `.ps1`. Новый `.bat` не добавлять.

## Совместимость (alpha)

Пока пользователь явно не отменит эту политику:

1. **Обратная совместимость только у стратегий.** Сохраняйте набор стратегий обхода (что они делают с трафиком), в том числе при переносе на zapret2 / Lua / `.ps1`. Набор **согласуйте** с [Flowseal/zapret-discord-youtube](https://github.com/Flowseal/zapret-discord-youtube): не уходите своим набором без явного решения. Имена файлов и подписи в GUI **можно менять**. Алиасы старых имён не нужны (`general (ALT5).bat` → то же имя навсегда).
2. **Во всём остальном совместимости нет.** Не сохраняйте старые флаги CLI, поля API, ключи пресетов, переменные окружения, имена служб и раскладку файлов «ради существующих пользователей».
3. **Удаляйте, не делайте прокладки.** Старый путь обвязки убирайте. Без алиасов, тихих fallback, двойного чтения и предупреждений об устаревании, которые оставляют оба поведения. Не держите `foo.bat` рядом с `foo.ps1`.
4. **Ломающие изменения** — в [`CHANGELOG.md`](CHANGELOG.md), датированная секция или `Unreleased` (Added / Changed / Removed / Breaking). Смена имени стратегии — Changed (старое → новое). Потеря стратегии без преемника — Breaking, и только осознанно.
5. **Документацию обновляйте в том же изменении** — `README.md`, этот файл, [`PLAN.md`](PLAN.md), [`docs/`](docs/), примеры в `lists/` и файлы стратегий. В них только текущий интерфейс.

**Стратегия** — сценарий обхода в `strategies/*.ps1` (оба движка). **Обвязка** — всё остальное; её старую форму выбрасывайте.

**Исключение для `bin/`:** нужны и `winws`, и `winws2` — [`PLAN.md`](PLAN.md). Стратегию не плодите вторым файлом «для другого движка»: два argv в одном `.ps1`.

## Среда выполнения

Целевой интерпретатор — **Windows PowerShell 5.1** (`powershell.exe`), не PowerShell 7 (`pwsh`).

**Windows 10 LTSC — цель рантайма, не разработки.** Таблица — что **запускает** продукт. Разработка, агент и `dev\lint.ps1` (в т.ч. STA + `XamlReader.Load`) могут быть на более новой десктопной сборке. Не считать машину агента LTSC. Не добавлять зависимость, без которой **рантайм на LTSC** перестаёт открываться (нет требования Visual Studio / .NET SDK).

| Платформа | Ожидание |
|---|---|
| Windows 10 LTSC (и новее с рабочим столом) | Запускает продукт без допнастроек: inbox PS 5.1, .NET 4.x, WPF (`PresentationFramework`), CIM |
| Windows 7 SP1 x64 | Нужны допнастройки: WMF 3.0+ (лучше WMF 5.1) и .NET 4.5+ |
| PowerShell 7 | Не цель. Не используйте синтаксис только для `pwsh` (`??`, `?:`, `&&` / `\|\|`, `-Parallel`) |

## Код

Ограничения 5.1, которые уже ломали GUI:

- `return @(...)` разворачивает массив из 0/1 элемента. Для `.Count` оборачивайте в `@()` **в месте использования**.
- `Set-StrictMode -Version Latest` запрещает обращение к несуществующим свойствам.
- `Get-Item HKLM:\...` открывает ключ только на чтение. Для записи — `OpenSubKey(..., $true)`.
- `$PSScriptRoot`, `Get-CimInstance`, `-LiteralPath` требуют минимум PowerShell **3.0**.
- WPF нужен **STA**: `powershell.exe -STA`. Вход — `zapman.bat`.
- В PS 5.1 не писать `New-Object System.Collections.Generic.List[object]`.

Тексты:

- Комментарии в коде — [ASD-STE100](https://www.asd-ste100.org/) Simplified Technical English. Политику (PS 5.1, не `pwsh`, совместимость стратегий) пишите в этом файле, не копируйте её в каждый `.ps1`.
- [`src/Zapman/Ui.ps1`](src/Zapman/Ui.ps1) с русскими строками — **UTF-8 с BOM**. Без BOM PowerShell 5.1 ломает литералы. XAML в [`src/gui/`](src/gui/) — ASCII; тексты через `Get-ZapmanUiString`.

## Линтер

После правок `.ps1`, `.bat`, `src/gui/`, `src/cli/`, `src/Zapman/`, `src/Zapret/`, `strategies/` или `dev/` прогоните линтер и исправьте все FAIL до конца задачи:

```text
powershell.exe -NoProfile -ExecutionPolicy Bypass -File dev\lint.ps1
```

После правки XAML или строк UI ещё:

```text
powershell.exe -NoProfile -ExecutionPolicy Bypass -File dev\export-gui-docs.ps1
```

`dev/lint.ps1` всегда ловит:

- PS 5.1 / WPF: `List[object]`; у главного окна нет `$window.IsEnabled = $false` / `Hide()` / `ShowDialog()` / `ShowInTaskbar = $false`
- нет Forms в `src/gui/`
- XAML только файлом (`XamlReader.Load`, `xmlns:x` если есть `x:`, без `x:Class`)
- BOM у кириллицы / `Ui.ps1`
- синтаксис `pwsh`
- `-STA` в `zapman.bat`
- `docs/ui` совпадает с генератором (`dev\export-gui-docs.ps1 -Check`)

Опционально: PSScriptAnalyzer; Blinter для `.bat` (`pipx install Blinter`, [`blinter.ini`](blinter.ini)).

## Раскладка

Новая логика — только `.ps1`. GUI не парсит флаги: запускает файл или ставит службу. Exe — из `engine` в [`config.json`](config.json) ([`PLAN.md`](PLAN.md)).

### Входы

| Путь | Роль |
|---|---|
| `zapman.bat` | GUI: inbox `powershell.exe -STA` → `src/gui/gui-boot.ps1` → `src/gui/gui.ps1`. UAC — `Start-Process -Verb RunAs` на `powershell.exe`. Без предстартовых проверок: ошибка остаётся в этом окне |
| `cli.bat` | Консоль: проверка `powershell.exe`, затем это окно — PowerShell (`src/cli/cli.ps1`) |

### GUI

| Путь | Роль |
|---|---|
| `src/gui/gui-boot.ps1` | Короткий `-File`: скрыть консоль, UAC (этот файл), показать `Main.xaml`, затем разбор `gui.ps1`. `Application.Run` после обвязки |
| `src/gui/gui.ps1` | WPF code-behind. Разметка — соседние `*.xaml`. Диалоги — `gui-dialogs.ps1` (dotsource) |

### Консоль

| Путь | Роль |
|---|---|
| `src/cli/cli.ps1` | Меню или `service` / `tests` / `env` в том же процессе |
| `src/cli/service.ps1` | Консольное меню (как GUI). Elevate — этот файл, не `cli.ps1` |
| `src/cli/test-zapret.ps1` | Консоль тестов → `Invoke-ZapmanStrategyTests` (ненулевой exit, если прогон не удался) |

### Модуль и данные

| Путь | Роль |
|---|---|
| `src/Zapman/` | Модуль обвязки (`Zapman.psd1`): config, UI, тесты, диагностика. GUI и консоль импортируют его. В `src` только исходники. Тег продукта — `v` + `ModuleVersion` (`0.1.0` → `v0.1.0`). Auto-Update Check сравнивает с GitHub tags |
| `src/Zapret/` | Движок: `Bypass.ps1` (winws, служба `zapret`, стратегии, фильтры, ipset, fake). Dotsource из Zapman, не отдельный модуль |
| `config.json` | Язык, `engine` (`winws` / `winws2`, окно стратегий), фильтры, Auto-Update Check, цели тестов. Корень пакета, не `user/` |
| `test-results/` | Логи тестов (на лету) |
| `strategies/*.ps1` | Стратегии: import `Zapman.psd1`, argv для выбранного `engine`. Prep / winws / filter — `*-Zapret*`. Сток — `$lists`, свои файлы — `$user` |
| `lists/` | Сток: `list-general.txt`, `list-exclude.txt`, `list-google.txt`, `ipset-exclude.txt`, `ipset-all.default.txt` |
| `user/` | Свои списки и рабочий `ipset-all.txt` (создаются на лету, в gitignore). При обновлении копируют папку в новую распаковку. Старый `lists/*-user.txt` не читается |
| `docs/` | Использование, troubleshooting, инструкция разработчику. Макеты окон — `docs/ui/` (генератор, не править руками) |
| `bin/` | `winws.exe` (zapret v72.13), `winws2.exe` + `lua/` (zapret2 v1.0.4), общий WinDivert. Хеши — [`bin/versions.json`](bin/versions.json). GUI не сверяет при открытии окна; сверка при старте движка (файлы выбранного `engine`) и в `cli.bat env`. Lua в хеше как LF |

### Dev

| Путь | Роль |
|---|---|
| `dev/lint.ps1` | 5.1/WPF + PSSA + Blinter; `-Check` макетов `docs/ui` |
| `dev/export-gui-docs.ps1` | XAML + `Ui.ps1` → `docs/ui/*.svg` и `index.md` / `index.html` |
| `dev/update-zapret.ps1` | Официальные zip zapret / zapret2 → `bin/` + [`bin/versions.json`](bin/versions.json). Fake `.bin` не трогает. |
| `.githooks/commit-msg` | Меняет `Co-authored-by: Cursor` на `Assisted-by`. Включение: `git config core.hooksPath .githooks` |

## Служба и фильтры

Одинаково для GUI и `src/cli/service.ps1` (служба и фильтры — `src/Zapret/Bypass.ps1`; остальное — `src/Zapman/`).

- **Remove Services:** сначала `zapret` и `winws`, потом WinDivert / WinDivert14. Иначе драйвер зависает в `STOP_PENDING`.
- **Install Service:** дождаться удаления старой службы; ImagePath из шаблона (`"exe" args`; для winws2 ещё `--chdir` на `bin/`); проверить запись в реестр; при сбое откатить `sc delete`.
- **Game Filter** и **движок** при установленной службе зашиты в ImagePath: нужен повторный Install, не просто Stop/Start.
- **IPSet** читается из файла при старте: достаточно Stop/Start. Режимы: `none` / `loaded` / `any`. Backup — rename, не «вечная» копия.
- **Fake…** копирует `ACTIVE_*.bin`; winws читает их при старте. Как IPSet: Stop/Start, не Install.

## GUI

- **Главное окно:** без списка стратегий. **Старт службы** (выключен, если службы нет) / **Стоп** / **Снять службы** / **Стратегия…**. Смена стратегии = Install из окна стратегий.
- **Стратегия…:** список `strategies/*.ps1` и выбор `winws` / `winws2`. **Установить службу** закрывает диалог. **Запустить без установки** останавливает службу/winws и стартует файл (без автозапуска). **Прогнать тесты** снимает службу zapret на время прогона и ставит её снова; WinDivert не трогает. **Ни одна не подходит** запускает сброс Winsock / IP / WinHTTP / DNS и просит перезагрузку. Список выделяет запущенную или установленную.
- **Settings:** Game Filter (диалог с кнопкой применения), IPSet, Auto-Update Check, **Fake…** (оба слота сразу).
- **Tools:** скачать ipset, сверить hosts (шаблон в буфер / открыть системный hosts в Блокноте), проверить версию, диагностика (список с галочками; **Запуск** у чисток, **ОК** пропуск). Клик по Status — полный статус и журнал.
- **Язык:** RU/EN, `Get-ZapmanUiString` в модуле. Переключатель на главном. Язык, `engine`, Game Filter, Auto-Update Check и цели тестов — [`config.json`](config.json) в корне.

### Окно (WPF)

- Крестик = выход; свернуть = панель задач. Трея нет.
- Главный цикл — `Application.Run`, не `$window.ShowDialog()` (дочерний `ShowDialog` выключает владельца и роняет UI). Не `Hide()` и не `$window.IsEnabled = $false`. `gui-boot.ps1` делает `$window.Show()` до разбора `gui.ps1`, чтобы каркас был на экране во время импорта модуля.
- Дочерние — `ShowDialog` с `Owner`.
- Тесты: без событий `Process` в scriptblock (пул потоков + STA).

## Что не делать

- Не добавлять зависимость от `pwsh`, WinUI, .NET SDK или Visual Studio, если **рантайм на LTSC** без них не открывается.
- Не сохранять обвязку (старые `.bat` лаунчера/службы, ключи реестра, раскладку `lists/`) через shim. Набор стратегий переносите; имена стратегий менять можно.
- Не писать эксплойты и не изобретать DPI-стратегии. Не писать компилятор Lua→флаги winws1, пока в [`PLAN.md`](PLAN.md) нет доказанного подмножества. Не подменять отсутствующий argv другого движка.
