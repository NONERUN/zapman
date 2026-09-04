# Разработка

Человеческая инструкция. Правила для агента (ловушки 5.1/WPF, политика alpha) — в [`AGENTS.md`](../AGENTS.md). Контракт движков — [`PLAN.md`](../PLAN.md). При смене политики правьте оба файла в одном изменении.

## Среда

Целевой интерпретатор — **Windows PowerShell 5.1** (`powershell.exe`), не PowerShell 7 (`pwsh`). Не используйте `??`, `?:`, `&&` / `||`, `-Parallel`.

**Windows 10 LTSC — цель рантайма, не разработки.** Разработка и `dev\lint.ps1` могут идти на более новой десктопной сборке. Не добавляйте зависимость, без которой продукт на LTSC не открывается (нет требования Visual Studio / .NET SDK).

В корне **два** `.bat`: `zapman.bat` (GUI, `-STA`) и `cli.bat` (консоль). Новый `.bat` не добавлять. Логика — только `.ps1`.

## Совместимость (alpha)

Обратная совместимость **только у набора стратегий** (что они делают с трафиком). Набор поддерживается в согласовании с [Flowseal/zapret-discord-youtube](https://github.com/Flowseal/zapret-discord-youtube). Имена файлов и подписи в GUI менять можно. Обвязку (CLI, служба, раскладка, флаги) не сохраняйте через shim: удаляйте старый путь.

Ломающие изменения — [`CHANGELOG.md`](../CHANGELOG.md). Документацию (`README.md`, этот файл, `AGENTS.md`, `PLAN.md`, `docs/`) обновляйте в том же изменении.

Версия продукта — `ModuleVersion` в [`src/Zapman/Zapman.psd1`](../src/Zapman/Zapman.psd1) (`0.1.0`). В окне и в git-теге: `v` + это число.

## Раскладка

| Путь | Роль |
|---|---|
| `zapman.bat` | GUI → `src/gui/gui-boot.ps1` → `gui.ps1` |
| `cli.bat` | Консоль → `src/cli/cli.ps1` |
| `src/gui/` | WPF: `*.xaml` (ASCII) + code-behind. Тексты — `Get-ZapmanUiString` |
| `src/Zapman/` | Модуль обвязки |
| `src/Zapret/Bypass.ps1` | Движок (dotsource, не отдельный модуль) |
| `strategies/*.ps1` | Два argv в одном файле (`winws` / `winws2`). Сток — `$lists`, свои файлы — `$user` |
| `lists/` | Сток-хостлисты и `ipset-all.default.txt` |
| `user/` | Свои списки и рабочий ipset (gitignore). `config.json` в корне |
| `docs/` | Использование, проблемы, эта страница; `docs/ui` — генератор |

Кириллица в [`src/Zapman/Ui.ps1`](../src/Zapman/Ui.ps1) — UTF-8 **с BOM**. Комментарии в коде — [ASD-STE100](https://www.asd-ste100.org/) Simplified Technical English.

## Линтер

После правок `.ps1`, `.bat`, `src/gui/`, `src/cli/`, `src/Zapman/`, `src/Zapret/`, `strategies/` или `dev/`:

```text
powershell.exe -NoProfile -ExecutionPolicy Bypass -File dev\lint.ps1
```

Исправить все FAIL до конца задачи. Опционально: PSScriptAnalyzer; Blinter для `.bat` (`pipx install Blinter`).

## Макеты окон

Генератор читает `src/gui/*.xaml` и строки UI, пишет [`docs/ui/`](ui/index.md). После правки XAML, `Ui.ps1` или привязок в `gui.ps1` / `gui-dialogs.ps1`:

```text
powershell.exe -NoProfile -ExecutionPolicy Bypass -File dev\export-gui-docs.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File dev\lint.ps1
```

Линтер в режиме `-Check` падает, если `docs/ui` устарел. Файлы в `docs/ui` не править руками.

Движки в `bin/` обновлять так: `dev\update-zapret.ps1` (официальные zip zapret / zapret2; fake `.bin` не трогает).
