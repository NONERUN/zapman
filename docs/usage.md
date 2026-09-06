# Использование

GUI: [`zapman.bat`](../zapman.bat). Консоль: [`cli.bat`](../cli.bat). Макеты окон: [Окна GUI](ui/index.md).

На **Windows 10 LTSC** и новее с рабочим столом ничего ставить не нужно (inbox PowerShell 5.1, .NET 4.x, WPF). На **Windows 7 SP1 x64** сначала WMF 5.1 и .NET Framework 4.5 или новее. `pwsh` не используется.

## Secure DNS

Без безопасного DNS часть сайтов может не открываться, даже если стратегия живая.

- **Chrome:** «Использовать безопасный DNS», поставщик **не** «по умолчанию».
- **Firefox:** DNS через HTTPS, режим «Персональный», URL вручную, например `https://dns.google/dns-query` (Cloudflare иногда недоступен).
- **Windows 11:** Secure DNS в настройках ОС — [инструкция](https://remontka.pro/dns-over-https-windows-11/).
- **Keenetic:** в роутере включите «Транзит запросов», иначе DNS на компьютере может не заработать.

## Установка

1. Скачайте zip/rar со [страницы релизов](https://github.com/NONERUN/zapman/releases/latest) **этого** репозитория.
2. Свойства архива → «Разблокировать» (7-Zip и PeaZip часто не требуют этого).
3. Распакуйте в путь **без** кириллицы, пробелов и спецсимволов.
4. Обновление: скопируйте старую папку `user/` в новую распаковку.
5. Запустите `zapman.bat` (нужны права администратора).

## Главное окно

Крестик закрывает GUI (процесс). Свернуть — кнопка на панели задач. Окно в область уведомлений не прячется.

- **Старт службы** — запускает уже установленную службу `zapret`. Если службы нет, кнопка серая.
- **Стоп** — останавливает `winws.exe` / `winws2.exe` и службу, если она запущена.
- **Снять службы** — удаляет службу `zapret` и драйверы WinDivert / WinDivert14. Задание планировщика `zapman-tray` не удаляет.
- **Стратегия…** — список файлов и выбор exe. См. ниже.
- **Settings** на этом же окне (отдельного окна нет): Game Filter, IPSet, Auto-Update Check, галка трея, **Fake…**.
- **Tools:** скачать ipset, сверить hosts, проверить версию, диагностика.
- Клик по блоку **Status** — полный дамп, последняя ошибка exe и журнал действий.

Язык RU/EN — радио справа вверху. Выбор пишется в `user/config.json`.

## Стратегия…

Список — файлы `strategies/*.json`. Серая строка не поддерживает выбранный exe.

Радио **winws** / **winws2** выбирает, какой exe будет в следующем действии. В `user/config.json` это попадает при **Установить службу**, **Запустить без установки** или **Прогнать тесты**. Закрыть диалог крестиком после смены радио — `engine` в файле не меняется.

- **Установить службу** — пишет ImagePath службы `zapret` (автозапуск), стартует службу, закрывает диалог. Game Filter и выбранный exe входят в эту командную строку. Потом смена фильтра или exe без этой кнопки на службу не действует.
- **Запустить без установки** — останавливает службу и чужой winws, стартует выбранный JSON. После перезагрузки Windows этого процесса не будет.
- **Прогнать тесты** — снимает службу `zapret` на время прогона, WinDivert не трогает. После прогона ставит службу снова, если JSON из снимка ещё на диске; нет файла — ошибка. Если в реестре нет имени стратегии — службу не возвращает. Окно: подготовка, вкладка на каждую стратегию (ошибка — красная ячейка, текст можно копировать), итоги.
- **Ни одна не подходит** — `netsh winsock reset`, `netsh int ip reset all`, `netsh winhttp reset proxy`, `ipconfig /flushdns`, затем вопрос про перезагрузку.

## Settings

**Game Filter** (`cmbGame`): подставляет порты `$gf.Tcp` / `$gf.Udp` в стратегию. В GUI подписи комбобокса, в `user/config.json` и в `cli.bat service` — ключи `disabled` / `all` / `tcp` / `udp`.

| GUI | `user/config.json` | Порты TCP/UDP в argv |
|---|---|---|
| `disabled` | `disabled` | `12` / `12` |
| `TCP and UDP` | `all` | `1024-65535` / `1024-65535` |
| `TCP only` | `tcp` | `1024-65535` / `12` |
| `UDP only` | `udp` | `12` / `1024-65535` |

После смены, если служба стоит, GUI спрашивает, открыть ли «Стратегия…» для повторного Install.

**IPSet** — файл `user/ipset-all.txt` (winws читает его при старте; достаточно Стоп/Старт):

| Значение | Файл |
|---|---|
| `none` | одна строка `203.0.113.113/32` |
| `loaded` | скачанный или свой список (после `none`/`any` GUI поднимает backup) |
| `any` | пустой файл (фильтр по IP не режет) |

**Иконка в трее** (`chkTray`): процесс [`src/Zapman/Tray.ps1`](../src/Zapman/Tray.ps1), задание `zapman-tray` при входе в Windows. По умолчанию включено. Меню: три строки статуса, открыть GUI, Старт службы, Стоп. Стратегию отсюда не меняют. Снять галку — нет иконки и нет задания при следующем входе. **Снять службы** задание не трогает.

**Fake…** — оба слота сразу (Discord UDP и Game UDP). Копирует выбранные `.bin` в `ACTIVE_*.bin`. Как IPSet: winws читает при старте, достаточно Стоп/Старт.

## Консоль

```text
cli.bat
cli.bat service
cli.bat tests [-TestType standard|dpi] [-Strategies all|name,...] [-NoPause]
cli.bat env
```

- без аргументов — меню. Пункт 4 — язык RU/EN (`user/config.json` `language`).
- `service` — меню: служба, фильтры, тесты, язык (пункт 11, RU/EN). Пункты 5/6 (`winws` / `winws2`) сразу пишут `user/config.json` `engine`. Без прав — UAC, исходное окно закрывается.
- `tests` — прогон стратегий. Без прав — UAC. `-NoPause` — не ждать клавишу. Код ≠ 0, если прогон не стартовал или нет завершённых результатов.
- `env` — WPF, оба набора файлов из `bin/versions.json`, версии продукта и ZapretSpec.

## Файлы

| Путь | Роль |
|---|---|
| `zapman.bat` | GUI (`powershell.exe -STA`) |
| `cli.bat` | Консоль |
| `strategies/*.json` | Стратегия. Argv для winws и winws2 собирает ZapretSpec |
| `bin/` | `winws.exe`, `winws2.exe`, Lua, WinDivert. Хеши — `bin/versions.json` |
| `lists/` | Сток-хостлисты и `ipset-all.default.txt` |
| `user/` | Свои списки, рабочий `ipset-all.txt`, `config.json` (gitignore) |
| `src/Zapman/` | Модуль. Версия в окне: `v` + `ModuleVersion` из `Zapman.psd1` |
| `src/ZapretSpec/` | Сборка argv. `specVersion` в JSON = его `ModuleVersion` |
| `src/Zapret/` | Старт exe, служба, фильтры |
| `test-results/` | Логи тестов, stderr/exit если exe не стартовал |

Движки — официальные [zapret](https://github.com/bol-van/zapret) и [zapret2](https://github.com/bol-van/zapret2). Сверка хеша при старте выбранного exe и в `cli.bat env`.

Имена файлов стратегий могут не совпадать с [Flowseal/zapret-discord-youtube](https://github.com/Flowseal/zapret-discord-youtube). Смысл атак тот же, пока набор не сменили явно.

## Свои адреса

Файлы в `user/` создаются при первом запуске `zapman.bat`.

- `user/list-general-user.txt` — домены (поддомены тоже).
- `user/list-exclude-user.txt` — исключить домен (если сеть уже в ipset).
- `user/ipset-all.txt` — IP и подсети. Нет файла — копия с `lists/ipset-all.default.txt`.
- `user/ipset-exclude-user.txt` — исключить IP и подсети.

Обновление: скопируйте папку `user/` в новую распаковку. Файлы из старого `lists/` (`*-user.txt`, рабочий `ipset-all.txt`) и корневой `config.json` программа не читает.
