<div align="center">

# <img src="https://cdn-icons-png.flaticon.com/128/5968/5968756.png" height=28 /> <a href="https://github.com/NONERUN/">NONERUN</a><a href="https://github.com/NONERUN/zapman">/zapman</a> <img src="https://cdn-icons-png.flaticon.com/128/1384/1384060.png" height=28 />

**NEW**: Ускорение Telegram Desktop — https://github.com/Flowseal/tg-ws-proxy  
Оригинальный проект: <a href="https://github.com/Flowseal/zapret-discord-youtube">zapret-discord-youtube</a>  
Альтернатива: https://github.com/bol-van/zapret-win-bundle  
Поддержать автора zapret: [bol-van/zapret](https://github.com/bol-van/zapret?tab=readme-ov-file#%D0%BF%D0%BE%D0%B4%D0%B4%D0%B5%D1%80%D0%B6%D0%B0%D1%82%D1%8C-%D1%80%D0%B0%D0%B7%D1%80%D0%B0%D0%B1%D0%BE%D1%82%D1%87%D0%B8%D0%BA%D0%B0)
</div>

> [!CAUTION]
>
> ### ФЕЙКИ
> Я не веду другие страницы и каналы в Telegram / YouTube.  
> Если вы наткнулись на что-то вне этого GitHub от моего лица — **фейк**.

> [!WARNING]
>
> ### АНТИВИРУСЫ
> WinDivert может вызвать реакцию антивируса: это фильтр трафика, не вирус. Добавьте папку в исключения или отключите детект PUA. Подробности — [Проблемы](docs/troubleshooting.md#античит-ругается-на-windivert).

> [!IMPORTANT]
> Файлы в [`bin`](./bin) — официальные [zapret](https://github.com/bol-van/zapret) (`winws.exe`) и [zapret2](https://github.com/bol-van/zapret2) (`winws2.exe`). Тег и SHA256 — [`bin/versions.json`](./bin/versions.json).

## Использование

1. Включите [Secure DNS](docs/usage.md#secure-dns) (не поставщик «по умолчанию»).
2. Скачайте архив со [страницы релизов](https://github.com/NONERUN/zapman/releases/latest) **этого** репозитория.
3. Свойства архива → «Разблокировать» (7-Zip / PeaZip часто не требуют).
4. Распакуйте в путь без кириллицы, пробелов и спецсимволов. Обновление: скопируйте старую папку `user/` в новую распаковку.
5. Запустите [`zapman.bat`](./zapman.bat) (права администратора).

На **Windows 10 LTSC** и новее с рабочим столом ничего ставить не нужно. На **Windows 7 SP1 x64** — WMF 5.1 и .NET 4.5+.

Кнопки, служба, трей (`zapman-tray`), IPSet, консоль `cli.bat` — [использование](docs/usage.md).

## Документация

- [Использование](docs/usage.md)
- [Проблемы](docs/troubleshooting.md)
- [Окна GUI](docs/ui/index.md)
- [Разработка](docs/dev.md) — JSON → флаги, `dev\update-zapret.ps1`
- [AGENTS.md](AGENTS.md) — правила для агента (5.1, WPF, что не делать)

## Разработка

Интерпретатор продукта — `powershell.exe` **5.1**, не `pwsh`.

```text
powershell.exe -NoProfile -ExecutionPolicy Bypass -File dev\lint.ps1
```

После смены XAML или строк UI:

```text
powershell.exe -NoProfile -ExecutionPolicy Bypass -File dev\export-gui-docs.ps1
```

## Поддержка проекта

Поставьте :star: репозиторию (сверху справа).

Материально — [автору zapret](https://github.com/bol-van/zapret?tab=readme-ov-file#%D0%BF%D0%BE%D0%B4%D0%B4%D0%B5%D1%80%D0%B6%D0%B0%D1%82%D1%8C-%D1%80%D0%B0%D0%B7%D1%80%D0%B0%D0%B1%D0%BE%D1%82%D1%87%D0%B8%D0%BA%D0%B0).

## Лицензия

[MIT](LICENSE.txt). Иконка окон и трея — [`src/gui/app.LICENSE.txt`](src/gui/app.LICENSE.txt) (Heroicons, MIT).

## Благодарность

[![Contributors](https://contrib.rocks/image?repo=NONERUN/zapman)](https://github.com/NONERUN/zapman/graphs/contributors)

Оригинальный репозиторий: [Flowseal/zapret-discord-youtube](https://github.com/Flowseal/zapret-discord-youtube).

Отдельная благодарность [bol-van](https://github.com/bol-van) за [zapret](https://github.com/bol-van/zapret).
