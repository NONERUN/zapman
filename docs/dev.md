# Разработка

Ловушки PowerShell 5.1 / WPF, два `.bat`, что не делать в коде — [`AGENTS.md`](../AGENTS.md). Этот файл — поля JSON, обновление `bin/` и GitHub Release. Правка генератора и этого файла — в одном изменении.

Версия продукта — `ModuleVersion` в [`src/Zapman/Zapman.psd1`](../src/Zapman/Zapman.psd1). В окне и в git-теге: `v` + это число. Схема JSON — `ModuleVersion` в [`src/ZapretSpec/ZapretSpec.psd1`](../src/ZapretSpec/ZapretSpec.psd1). Поле `specVersion` в каждом `strategies/*.json` должно совпадать. `cli.bat env` печатает обе.

## Движки

В `bin/` лежат оба exe, общий WinDivert / `cygwin1.dll`, Lua рядом с winws2. Теги и SHA256 — [`bin/versions.json`](../bin/versions.json).

Обновить exe (fake `.bin` не трогает):

```text
powershell.exe -NoProfile -ExecutionPolicy Bypass -File dev\update-zapret.ps1
```

Windows 7: тот же `WinDivert.dll` / `WinDivert64.sys` для обоих exe. Если ОС требует подпись — подмена из [zapret-win-bundle/win7](https://github.com/bol-van/zapret-win-bundle/tree/master/win7). См. [`troubleshooting.md`](troubleshooting.md).

Выбор exe — радио в «Стратегия…» (GUI) или пункты 5/6 в `cli.bat service`. Ключ `user/config.json` `engine`: `winws` (дефолт) или `winws2`. Нет файла выбранного exe — ошибка, второй exe сам не подставляется.

Служба `zapret`: ImagePath = `"<bin>\<exe>" <argv>`. Для winws2 `Install-ZapretService` дописывает `--chdir="<bin>"`, если генератор сам `--chdir` не дал (не даёт). SCM стартует из System32; без `--chdir` Lua не находится. Смена `engine` или Game Filter при установленной службе — снова **Установить службу**.

GUI и CLI строку флагов не разбирают. Нет exe — отказ.

Одинаковые пакеты на winws и winws2 не обещаем (у z2 другие range, payload, empty ACK). Одинаковые поля JSON — да. Проверка — тесты на выбранном `engine`.

## JSON → argv

Один файл `strategies/*.json`. Генератор — [`src/ZapretSpec/`](../src/ZapretSpec/), экспорт `ConvertTo-ZapretStrategyArgList`. Lua только stock `bin/lua/zapret-lib.lua` и `bin/lua/zapret-antidpi.lua`. Свой `.lua` не подключать. Lua-атаку в флаги winws не переводить.

Неизвестный `desync` — throw. Не добавлять имя атаки вне таблицы.

Префиксы путей: `lists:` → `lists/`, `user:` → `user/`, `bin:` → `bin/`. Регистр префикса как в таблице.

### Корень файла

| Поле | winws | winws2 |
|---|---|---|
| `specVersion` | Должно равняться ZapretSpec `ModuleVersion` | то же |
| `wf.tcp` | `--wf-tcp=` | `--wf-tcp-out=` |
| `wf.udp` | `--wf-udp=` | `--wf-udp-out=` |

В `wf.*` токены `$gf.Tcp` / `$gf.Udp` заменяются на порты Game Filter (`12` или `1024-65535`).

### Профиль (`profiles[]`)

`desync` — только эти имена: `fake`, `multisplit`, `multidisorder`, `hostfakesplit`, `fakedsplit`, `syndata`.

| Поле | winws | winws2 |
|---|---|---|
| `tcp` / `udp` / `l3` / `l7` | `--filter-tcp=` / `--filter-udp=` / `--filter-l3=` / `--filter-l7=` | то же |
| `payload` | не пишется (фильтр из `l7`) | `--payload=` |
| `hostlist` | `--hostlist=` | то же |
| `hostlistExclude` | `--hostlist-exclude=` | то же |
| `hostlistDomains` | `--hostlist-domains=` | то же |
| `hostlistExcludeDomains` | `--hostlist-exclude-domains=` | то же |
| `ipset` | `--ipset=` | то же |
| `ipsetExclude` | `--ipset-exclude=` | то же |
| `desync` | `--dpi-desync=` | `--lua-desync=<атака>:…` на каждую атаку |
| `repeats` | `--dpi-desync-repeats=` | `repeats=` у Lua (не на `multisplit` / `multidisorder`) |
| `anyProtocol` | `--dpi-desync-any-protocol=1` | `--payload=all` если нет явного `payload` |
| `cutoff` | `--dpi-desync-cutoff=` как в JSON | `nN` → `--out-range=-dN`; иначе при TCP/QUIC → `--out-range=-d10` |
| `splitPos` | `--dpi-desync-split-pos=` | `pos=` у split-атак |
| `seqovl` | `--dpi-desync-split-seqovl=` | `seqovl=` |
| `seqovlPattern` | `--dpi-desync-split-seqovl-pattern=` (путь `bin:`/`lists:`/`user:`) | blob + `seqovl_pattern=` |
| `ipId` | `--ip-id=` | `ip_id=` |
| `tlsMod` | `--dpi-desync-fake-tls-mod=` после `fake-tls` | `tls_mod=` на последнем TLS-fake |
| `hostfakesplitMod` | `--dpi-desync-hostfakesplit-mod=` как в JSON | куски через запятую; **`altorder=*` не писать** (в stock Lua нет) |
| `fakedsplitPattern` | `--dpi-desync-fakedsplit-pattern=` (путь, не сырой `bin:`) | `pattern=` / blob |

### Fooling

| JSON | winws | winws2 |
|---|---|---|
| `fooling` содержит `ts` | `--dpi-desync-fooling=…ts…` | `tcp_ts=-600000` |
| `fooling` содержит `md5sig` | `--dpi-desync-fooling=…md5sig…` | `tcp_md5` |
| `fooling` содержит `badseq` | `--dpi-desync-fooling=…badseq…` и `--dpi-desync-badseq-increment=` | `tcp_seq=` и `tcp_ack=` |
| нет `badseqIncrement` при `badseq` | increment **−10** | **−10** |
| другое имя fooling | throw | throw |

Fooling не вешается на Lua-атаку `multisplit`.

### Fake

Слоты: `quic`, `tls`, `http`, `discord`, `stun`, `unknown`, `unknownUdp`.

| JSON | winws | winws2 |
|---|---|---|
| `fake.tls` / `http` / … | `--dpi-desync-fake-tls=` (и аналоги) | `--blob=name:@file` или `name:0xHEX`, затем `blob=` в `--lua-desync=fake` |
| значение `!` | как в JSON (встроенный hello z1) | stock `bin:tls_clienthello_www_google_com.bin` |
| TLS-профиль с `fake` в `desync`, слота `tls` нет | z1 подставляет встроенный `!` | то же, что `!` (google hello). Не `blob=empty` |
| hex `0x00000000` | как есть | имя blob `hex_00000000`, не `0x…` |
| порядок | `fake-tls` **до** `fake-tls-mod` | `tls_mod` только на последний TLS-ref |

На z2 в argv не должно быть `--dpi-desync`. На z1 не должно быть `--lua-desync`. `--blob=` только `name:value` (двоеточие).

## Линтер и макеты

После правок `.ps1` / стратегий / `dev/`:

```text
powershell.exe -NoProfile -ExecutionPolicy Bypass -File dev\lint.ps1
```

После XAML или `src/Zapman/Ui.ps1`:

```text
powershell.exe -NoProfile -ExecutionPolicy Bypass -File dev\export-gui-docs.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File dev\lint.ps1
```

Файлы в [`docs/ui/`](ui/index.md) руками не править. README показывает [`docs/ui/Main.svg`](ui/Main.svg). Кириллица в `src/Zapman/Ui.ps1` — UTF-8 с BOM.

## Релиз

Push аннотированного тега `vMAJOR.MINOR.PATCH` на `origin` сам запускает [Release](../.github/workflows/release.yml) **с того коммита** (YAML в теге). Job собирает zip/rar/tar.gz и создаёт **черновик**. `/releases/latest` не меняется, пока не нажмёте **Publish release**.

1. `ModuleVersion` в [`src/Zapman/Zapman.psd1`](../src/Zapman/Zapman.psd1). Если менялась схема JSON — ZapretSpec и `specVersion` в `strategies/*.json`.
2. Секция в [`CHANGELOG.md`](../CHANGELOG.md) на этот тег, коммит **с актуальным** `release.yml`.
3. Аннотированный тег `v` + ModuleVersion **на этом коммите**:
   `git tag -a v0.1.1 -m v0.1.1`
4. `git push origin main` и `git push origin v0.1.1`. Не `git push --tags`: в клоне могут быть чужие теги вроде `1.10.2` без `v`.
5. Дождаться Lint на `main` и зелёного job Release.
6. Releases → черновик → проверить архивы → **Publish release**.

Повтор без нового тега (тот же YAML с `main`):

```text
gh workflow run Release --repo NONERUN/zapman --ref main -f tag=v0.1.1
```

Всегда `--repo NONERUN/zapman`: `gh` в этой копии может смотреть на Flowseal. Не создавать Release вручную в UI без этого workflow — не будет zip/rar/tar.gz.

Пока черновик — его можно удалить в UI и прогнать job снова. После Publish ассеты **immutable**, повтор job их не трогает.

Auto-Update Check смотрит **git-теги**, не черновики: тег на origin уже виден клиенту. Скачать «latest» он зовёт на опубликованный релиз.
