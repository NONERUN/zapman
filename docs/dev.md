# Разработка

Ловушки PowerShell 5.1 / WPF, два `.bat`, что не делать в коде — [`AGENTS.md`](../AGENTS.md). Этот файл — поля JSON, обновление `bin/` и GitHub Release. Правка генератора и этого файла — в одном изменении.

Версия продукта — `ModuleVersion` в [`src/Zapman/Zapman.psd1`](../src/Zapman/Zapman.psd1). В окне и в git-теге: `v` + это число. Формат стратегий — `"spec": 2` (раздел «Формат v2» ниже); `Version` в [`internal/spec`](../internal/spec/). `cli.bat env` печатает версию продукта и `ModuleVersion` ZapretSpec.

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

Стратегия v2 (см. «Формат v2») разворачивается в профили. Генератор — [`src/ZapretSpec/`](../src/ZapretSpec/), экспорт `ConvertTo-ZapretStrategyArgList`. Lua только stock `bin/lua/zapret-lib.lua` и `bin/lua/zapret-antidpi.lua`. Свой `.lua` не подключать. Lua-атаку в флаги winws не переводить.

Неизвестный `desync` — throw. Не добавлять имя атаки вне таблицы.

Проверка JSON одна для обоих движков. Имена `desync`, `fooling`, `l7`, `cutoff` и префиксы путей — с учётом регистра (`fake`, не `FAKE`; `lists:`, не `LISTS:`). `engine` — ровно `winws` или `winws2`. Ошибка сборки:

- нет `profiles` или список пуст;
- в `l7` имя не из `http`, `tls`, `quic`, `wireguard`, `dht`, `discord`, `stun`, `unknown`, или вместе `quic` и `tls`;
- `cutoff` не `nN` и не `none`;
- путь без префикса или без имени файла после префикса;
- пустой элемент в `hostlistDomains` / `hostlistExcludeDomains`;
- значение fake не `!`, не `0x<hex>` и не путь с префиксом, или в нём пробел.

Префиксы путей: `lists:` → `lists/`, `user:` → `user/`, `bin:` → `bin/`. Регистр префикса как в таблице.

### Формат v2

`strategies/` — сток-каталог (JSON Schema — `strategies/schema/`, генерируется из структур Go: `go generate ./internal/spec`; у каждого файла `$schema` с относительным путём). Каталоги пользователя и внешние будут с той же раскладкой (пока не сделано):

```text
strategies/
  catalog.json        { "id": "stock", "title": "Zapman", "spec": 2 }
  lists.json  targets.json  actions.json   кирпичи: общие для всех стратегий каталога
  flowseal/<имя>.json  порты .bat из Flowseal/zapret-discord-youtube
  zapman/<имя>.json    свои стратегии (папки пока нет: создать с первой стратегией)
  games/<id>.json      игровые модули
  schema/              JSON Schema
```

Происхождение стратегии (`Origin`: `flowseal` / `zapman`) — папка, не поле JSON. Имя стратегии — имя файла без `.json`; одно имя в двух папках — ошибка загрузки. Другой `*.json` в корне каталога (кроме `catalog.json` и кирпичей) — ошибка: стратегия должна лежать в папке своего происхождения.

| Файл | Что внутри |
|---|---|
| `catalog.json` | `id` (имя `a-z`, `0-9`, `-`), `title`, `spec` = 2 |
| `lists.json` | именованные наборы списков: `hostlist`, `hostlistExclude`, `ipset`, `ipsetExclude` (пути `lists:`/`user:`/`bin:`) |
| `targets.json` | именованные цели: `tcp`, `udp`, `l3`, `l7`, `lists` (имена наборов с `hostlist`/`ipset`), `exclude` (наборы с `*Exclude`), `domains`, `excludeDomains`, `anyProtocol`, `payload` |
| `actions.json` | именованные действия: `desync` и поля атаки из таблиц ниже; `cutoff` — число N (= `nN`) или `"none"` |
| `flowseal/<имя>.json`, `zapman/<имя>.json` | стратегия: `"spec": 2`, версия (`upstream` + `rev` или `version`, см. «Версии»), `about`, `rules[]` — пары `{ "target", "action" }` в порядке winws (первое совпадение выигрывает) |
| `games/<id>.json` | игровой модуль: `id` (= имя файла), `title`, `version`, `order`, `updater`, свои `lists` (с `seed`), `rules[]` |

`target` и `action` в правиле — имя из каталога или объект прямо в правиле. Цель выносится в `targets.json`, если используется 2+ раза. Действие выносится в `actions.json` (пресет), если используется 3+ раза; остальные действия — прямо в правиле. Пресет с одним-двумя пользователями не заводить: при правке такой стратегии действие возвращается в правило. Имена — `a-z`, `0-9`, `-`. Загрузка строгая: неизвестное поле, имя, порт или отсутствующий файл `bin:` — ошибка. Набор игры не может повторять имя из `lists.json`; `seed` — путь `lists:`, из него создаётся `user:`-список, если его нет.

Правило → профиль v0.1 → тот же генератор, что и раньше. `payload` для winws2 выводится из `l7` / `anyProtocol` (например, `tls`+`http` → две группы, `discord` → `discord_ip_discovery,stun`); явный `payload` пишут только когда вывод не подходит.

`wf` не задаётся руками: это объединение портов всех целей (стратегии и включённых игр), без повторов, по возрастанию первого порта; `$gameFilter` (порты Game Filter, `12` или `1024-65535`) — последним. winws: `--wf-tcp=` / `--wf-udp=`, winws2: `--wf-tcp-out=` / `--wf-udp-out=`.

Игры идут **до** правил стратегии (они уже, чем общие правила на тех же портах), по `order`, затем по `id`. Сейчас: `sky` (10), `sker` (20), `photon` (30).

Одноразовый конвертер v0.1 → v2 — [`dev/convert-v2`](../dev/convert-v2/); он сверяет argv до записи. Модель v0.1 (`V01Spec`) осталась внутри Go как промежуточная. Конвертер берёт `upstream` из git (`-git`, `-ref`, по умолчанию `upstream/main`) и пишет `rev: 1`, игре — `version: 1`; его тест сравнивает вывод с `strategies/` при `rev` = 1 и пропускается без `upstream/main`.

### Версии

Версия говорит, чей argv и с какой правки. Go: `Strategy.Version()` / `Game.Version()` — строка для людей (`Flowseal 1.10.4 r2`, `Zapman 3`), `Origin()` — `flowseal` / `zapman` (у игр всегда `zapman`).

| Где | Поля | Смысл |
|---|---|---|
| `flowseal/` | `upstream`, `rev` (оба обязательны, `version` нельзя) | `upstream` — первый по дате тег Flowseal, в котором есть последний коммит upstream, менявший `<имя>.bat`. `rev` — 1 + число наших изменений argv с этого тега; новый тег `upstream` → `rev` снова 1 |
| `zapman/` | `version` (обязательно, `upstream`/`rev` нельзя) | целое ≥ 1, +1 при каждом изменении argv |
| `games/` | `version` (обязательно) | целое ≥ 1, +1 при каждом изменении argv модуля |

Когда поднимать: argv стратегии или игры поменялся (правка JSON, кирпича, который она использует, или генератора) → `rev` / `version` +1. Без изменения argv — не трогать (переименование пресета, вынос в кирпич, `about`). Изменение генератора поднимает версию каждой затронутой стратегии.

Страж — первая строка каждого golden-файла: `# version: flowseal 1.10.4 r2` или `# version: zapman 3`; у `general (ALT11)+games` после стратегии — игры в порядке сборки (`; sky 1; sker 1; photon 1`). `go test ./internal/spec -run Golden -update` пишет строку заново. Без `-update` тест падает с `argv changed: bump rev/version`, если argv другой, а строка версии та же; любое другое расхождение (или нет строки версии) — тоже ошибка. Порядок: поднять версию → `-update` → проверить diff.

Синхронизация с upstream — [`dev/upstream-versions`](../dev/upstream-versions/) (нужен `git fetch upstream`):

```text
go run ./dev/upstream-versions [-git .] [-ref upstream/main] [-dir strategies] [-write]
```

Печатает таблицу: имя, тег в файле, вычисленный тег, статус (`ok`, `outdated`, `no .bat upstream`, `no tag yet`); код выхода 2, если есть не `ok`. `-write` ставит вычисленный `upstream` и `rev: 1` только там, где тег сменился; затем перенести изменения `.bat` в JSON и обновить golden. Теги — только из `-ref` (`--merged`), наши `v*` не считаются.

### Цель и действие (поля профиля)

`desync` — только эти имена: `fake`, `multisplit`, `multidisorder`, `hostfakesplit`, `fakedsplit`, `syndata`.

| Поле | winws | winws2 |
|---|---|---|
| `tcp` / `udp` / `l3` / `l7` | `--filter-tcp=` / `--filter-udp=` / `--filter-l3=` / `--filter-l7=` | то же |
| `payload` | не пишется (фильтр из `l7`) | `--payload=` |
| `luaPayload` | не пишется | `payload=` на каждой Lua-атаке; для неизвестных игровых пакетов нужен `all`, одного `--payload=all` недостаточно. У `fake` заменяет фильтры слотов (см. Fake) |
| `hostlist` | `--hostlist=` | то же |
| `hostlistExclude` | `--hostlist-exclude=` | то же |
| `hostlistDomains` | `--hostlist-domains=` | то же |
| `hostlistExcludeDomains` | `--hostlist-exclude-domains=` | то же |
| `ipset` | `--ipset=` | то же |
| `ipsetExclude` | `--ipset-exclude=` | то же |
| `desync` | `--dpi-desync=` | `--lua-desync=<атака>:…` на каждую атаку |
| `repeats` | `--dpi-desync-repeats=` | `repeats=` у Lua (не на `multisplit` / `multidisorder`) |
| `anyProtocol` | `--dpi-desync-any-protocol=1` | `--payload=all` если нет явного `payload` |
| `cutoff` | `nN` → `--dpi-desync-cutoff=nN` | `nN` → `--out-range=-nN`; иначе при TCP/QUIC → `--out-range=-d10` |
| `cutoff: "none"` | cutoff не пишется | `--out-range` не пишется: обработка продолжается после TLS-handshake |
| `ttl` | `--dpi-desync-ttl=` | `ip_ttl=` на атаках, принимающих fooling; число 1–255 |
| `splitPos` | `--dpi-desync-split-pos=` | `pos=` у split-атак |
| `seqovl` | `--dpi-desync-split-seqovl=` | `seqovl=` |
| `seqovlPattern` | `--dpi-desync-split-seqovl-pattern=` | blob + `seqovl_pattern=`. На обоих движках только путь `bin:`/`lists:`/`user:` |
| `ipId` | `--ip-id=` | `ip_id=` |
| `tlsMod` | `--dpi-desync-fake-tls-mod=` после `fake-tls` | `tls_mod=` на последнем ref слота `tls`, только на вызове для `tls_client_hello`. Без слота `tls` — не пишется |
| `hostfakesplitMod` | `--dpi-desync-hostfakesplit-mod=` как в JSON | куски через запятую; **`altorder=*` не писать** (в stock Lua нет) |
| `fakedsplitPattern` | `--dpi-desync-fakedsplit-pattern=` (путь, не сырой `bin:`) | `pattern=` / blob |

Cutoff на winws2 — `-nN` (счёт пакетов), как `--dpi-desync-cutoff=nN` у winws. На UDP счёт тот же. На TCP winws2 по умолчанию не перехватывает пустые ACK (`--wf-tcp-empty=0`, zapret2 `manual.en.md:1071`, `readme.md:314`), поэтому cutoff наступает на 1–2 пакета с данными позже, чем у winws, но раньше прежнего `-dN`. Точное совпадение требует глобального `--wf-tcp-empty=1` — это примерно 2× CPU на загрузках при Game Filter; его не пишем и не предупреждаем. Неявный `-d10` для TCP/QUIC остаётся.

### Fooling

| JSON | winws | winws2 |
|---|---|---|
| `fooling` содержит `ts` | `--dpi-desync-fooling=…ts…` | `tcp_ts=-600000` |
| `fooling` содержит `md5sig` | `--dpi-desync-fooling=…md5sig…` | `tcp_md5` |
| `fooling` содержит `badseq` | `--dpi-desync-fooling=…badseq…`; `--dpi-desync-badseq-increment=` только если есть `badseqIncrement` | `tcp_seq=<badseqIncrement>` (при 0 не пишется) и `tcp_ack=-66000` |
| нет `badseqIncrement` при `badseq` | флага нет: дефолт winws seq **−10000**, ack **−66000** (`nfq/params.h`) | `tcp_seq=-10000:tcp_ack=-66000` |
| другое имя fooling | throw | throw |

Fooling не вешается на Lua-атаку `multisplit`.

### Fake

Слоты: `quic`, `tls`, `http`, `discord`, `stun`, `unknown`, `unknownUdp`.

| JSON | winws | winws2 |
|---|---|---|
| `fake.tls` / `http` / … | `--dpi-desync-fake-tls=` (и аналоги) | `--blob=name:@file` или `name:0xHEX`, затем `blob=` в `--lua-desync=fake` |
| значение `!` | как в JSON (встроенный hello z1) | stock `bin:tls_clienthello_www_google_com.bin` |
| TLS-профиль с `fake` в `desync`, слота `tls` нет | z1 берёт встроенный hello | ошибка сборки: нет blob для группы. Неявного hello нет: пишите `"tls": ["!"]` |
| `payload: "all"` / `anyProtocol`, слота `tls` нет | z1 берёт встроенный hello для TLS | TLS hello не подставляется |
| группа из нескольких слотов (`all`, `discord,stun`), нет `luaPayload` | fake по типу пакета: TCP `http`/`tls`/`unknown`, UDP `quic`/`discord`/`stun`/`unknownUdp` | каждый слот — свой `payload=` у `fake`: `tls` → `tls_client_hello`, `http` → `http_req`, `quic` → `quic_initial`, `discord` → `discord_ip_discovery`, `stun` → `stun`, `unknown` → `~empty,http_req,tls_client_hello`, `unknownUdp` → `~empty,quic_initial,wireguard_initiation,dht,discord_ip_discovery,stun`. Один ref в нескольких слотах — один вызов, фильтры через запятую. Слоты другого L4 (UDP-слот в TCP-профиле) не пишутся. `unknown` / `unknownUdp` в профиле с `tcp` и `udp` — ошибка сборки |
| hex `0x00000000` | как есть | имя blob `hex_00000000`, не `0x…` |
| порядок | `fake-tls` **до** `fake-tls-mod` | `tls_mod` только на последний ref слота `tls` в группе `tls_client_hello` или с фильтром `tls_client_hello`; этот вызов не сливается с другими слотами. На `!` из `unknown` и других слотов — нет |

На z2 в argv не должно быть `--dpi-desync`. На z1 не должно быть `--lua-desync`. `--blob=` только `name:value` (двоеточие).

## Линтер и макеты

Игровые модули (`strategies/games/`) не зависят от Game Filter:

- `sky` — Sky: UDP 5000–8000 с `user:ipset-sky.txt`, QUIC-fake, TTL 8, cutoff 4; TCP 443 для `live.ws.radiance.thatgamecompany.com` с multisplit на позиции 1, `luaPayload: all`, без cutoff. Список создаётся из `lists/ipset-sky.default.txt`; трей (`updater: sky-log`) дополняет его только конкретными IPv4 `/32` из `Sky.log`. DNS входа — отдельная настройка, стратегия `hosts` не меняет.
- `sker` — Sker Ritual и Hazard Levels (тот же разработчик): `user:ipset-sker.txt`, fake `ACTIVE_GAME_UDP`, `luaPayload: all`, TTL 8, cutoff 4. Fake без TTL доходил до сервера Photon, и игра теряла соединение (`ClientTimeout`) через ~10 с после входа в матч. Список создаётся из `lists/ipset-sker.default.txt` (подсети Photon Cloud EU на G-Core и Vultr) и сам не обновляется. Те же подсети у PEAK в регионе EU: он тоже попадает сюда.
- `photon` — общий профиль Photon (UDP 5055–5058, 27000–27002); стоит после `sker`.

Бывшая `general (ALT11 Photon)` = `general (ALT11)` + `sky`, `sker`, `photon`; отличие только в порядке профилей и портов `wf`.

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
