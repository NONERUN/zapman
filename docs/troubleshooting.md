# Проблемы

Сначала [использование](usage.md) и макеты [окон GUI](ui/index.md). Игры — в [Discussions](https://github.com/NONERUN/zapman/discussions) и в [Discussions оригинального репозитория](https://github.com/Flowseal/zapret-discord-youtube/discussions), не в Issues.

## После запуска стратегии ничего не происходит

После **Запустить без установки** или **Установить службу** в панели задач должен быть `winws.exe` или `winws2.exe`. Свернуть GUI оставляет кнопку на панели задач. Крестик закрывает GUI, службу не снимает. Иконка — процесс [`src/Zapman/Tray.ps1`](../src/Zapman/Tray.ps1), задание `zapman-tray` при входе: открыть GUI, Старт службы, Стоп.

Если процесса нет: клик по **Status** — stderr/stdout winws / winws2 и код выхода. **Диагностика**, исключения антивируса для папки, сверка `bin/` (`cli.bat env`). В окне тестов тот же дамп на вкладке стратегии (можно выделить и копировать) и в `test-results/`.

## Ни одна стратегия не подходит

**Стратегия…** → **Ни одна не подходит** (то же в `cli.bat service`). Zapret Manager запускает:

```text
netsh winsock reset
netsh int ip reset all
netsh winhttp reset proxy
ipconfig /flushdns
```

После этого перезагрузите Windows.

## Не работает Telegram (веб) или бесконечное «подключение» к голосовому чату Discord

`zapman.bat` → **Сверить hosts** (или `cli.bat service`). Если hosts устарел:

1. **Копировать шаблон** — блок из репозитория в буфер.
2. **Открыть hosts** — системный файл в Блокноте (можно сохранить: GUI уже с правами администратора).
3. Вставьте блок в конец (или замените прежний) и сохраните.
4. Проверьте, что файл действительно записался.

## Обход не работает / перестал работать

Стратегии со временем могут переставать работать. В репозитории несколько вариантов (`ALT`, `FAKE` и другие). Новую стратегию собирайте из существующей `.json`. Флаги winws: [документация zapret](https://github.com/bol-van/zapret/blob/master/docs/readme.md#nfqws).

- **Диагностика** (или `cli.bat service`)
- Адрес есть в списках доменов или IP
- Другая стратегия
- Полная переустановка (ниже)

## Как переустановить полностью

1. Скопируйте папку `user/` (свои `*-user.txt`, рабочий `ipset-all.txt`, `config.json`).
2. Перезагрузите устройство.
3. `zapman.bat` → **Снять службы** (или `cli.bat service`).
4. **Диагностика** (ошибки устраните; кэш Discord — по запросу).
5. Удалите папку с Zapret Manager.
6. Скачайте архив со [страницы релизов](https://github.com/NONERUN/zapman/releases/latest).
7. Свойства архива → «Разблокировать» → распакуйте в путь без кириллицы, пробелов и спецсимволов.
8. Скопируйте сохранённую `user/` в новую папку.
9. Пробуйте стратегии. Рабочую поставьте на автозапуск: **Стратегия…** → **Установить службу**.

## Игра или приложение ломается при включённом обходе

В GUI: **Game Filter** = `disabled`, **IPSet** = `none`. Иначе фильтр может задеть лишнее. Затем снова Install или запуск стратегии.

Если без zapret всё работает, а с ним падает игра — это не баг конкретной игры в Issues. См. раздел про игры ниже.

## Античит ругается на WinDivert

См. [windivert-hide](https://github.com/bol-van/zapret-win-bundle/tree/master/windivert-hide). Подробнее про реакцию антивируса — там же и в [README zapret-win-bundle](https://github.com/bol-van/zapret-win-bundle/blob/master/readme.md#%D0%B0%D0%BD%D1%82%D0%B8%D0%B2%D0%B8%D1%80%D1%83%D1%81%D1%8B). Добавьте папку в исключения или отключите детект PUA.

## Требуется цифровая подпись драйвера WinDivert (Windows 7)

Замените `WinDivert.dll` и `WinDivert64.sys` в [`bin`](../bin) на файлы из [zapret-win-bundle/win7](https://github.com/bol-van/zapret-win-bundle/tree/master/win7). В поставке один общий WinDivert на оба exe; на Win7 подмена нужна, если ОС требует подпись драйвера.

## После «Снять службы» WinDivert остаётся

В `cmd`:

```cmd
driverquery | find "Divert"
```

Затем:

```cmd
sc stop имя_из_первого_шага
sc delete имя_из_первого_шага
```

Снимайте сначала `zapret` / `winws`, потом WinDivert. Иначе драйвер может зависнуть в `STOP_PENDING`.

## YouTube

- [Secure DNS](usage.md#secure-dns)
- Отключите блокировщик рекламы
- Другие стратегии, если раньше работало

## Discord

- [Secure DNS](usage.md#secure-dns)
- Сначала стратегия, на которой открывается YouTube
- **Диагностика** → очистка кэша Discord
- Проверьте приложение и браузер: https://discord.com/app

## Telegram

Десктоп: [tg-ws-proxy](https://github.com/Flowseal/tg-ws-proxy). Либо бесплатные MTProto-прокси.

## Игры

1. Tools → скачать ipset, включить **Game Filter** (`TCP and UDP` / `all`).
2. Если мало — **IPSet** = `any` (пустой `user/ipset-all.txt`; ломает много сайтов; не держите постоянно). Лучше выписать IP игры в `user/ipset-all.txt`.
3. Если не помогло — [Discussions этого репозитория](https://github.com/NONERUN/zapman/discussions) или [Discussions оригинального](https://github.com/Flowseal/zapret-discord-youtube/discussions). Не Issue.

## Не нашли проблему

Issue: [NONERUN/zapman](https://github.com/NONERUN/zapman/issues). Форма просит текущие имена GUI и версию из заголовка окна (`Zapret Manager v…`).
