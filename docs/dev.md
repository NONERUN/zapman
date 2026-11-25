# Разработка

Раскладка, проверки и что не делать в коде — [`AGENTS.md`](../AGENTS.md). Этот файл — поля JSON, обновление `bin/` и GitHub Release. Правка генератора и этого файла — в одном изменении.

Версия продукта на ветке `go` пока не задана (на `main` — `ModuleVersion` в `src/Zapman/Zapman.psd1`, git-тег `v` + это число). Формат стратегий — `"spec": 2` (раздел «Формат v2» ниже); `Version` в [`internal/spec`](../internal/spec/).

## Движки

В `bin/` лежат только файлы движков: оба exe, общий WinDivert / `cygwin1.dll`, Lua рядом с winws2. Теги и SHA256 — [`bin/versions.json`](../bin/versions.json). Fake и паттерны `.bin` — в каталоге стратегий, `strategies/fakes/` (см. «Формат v2»).

Обновить exe:

```text
go run ./dev/update-zapret [-zapret-tag vX] [-zapret2-tag vY] [-dry-run] [-force]
```

Windows 7: тот же `WinDivert.dll` / `WinDivert64.sys` для обоих exe. Если ОС требует подпись — подмена из [zapret-win-bundle/win7](https://github.com/bol-van/zapret-win-bundle/tree/master/win7). См. [`troubleshooting.md`](troubleshooting.md).

Выбор exe — радио в «Стратегия…» (GUI). Ключ `user/config.json` `engine`: `winws` (дефолт) или `winws2`. Нет файла выбранного exe — ошибка, второй exe сам не подставляется.

Служба `zapret`: ImagePath = `"<bin>\<exe>" <argv>`. Для winws2 установка службы дописывает `--chdir="<bin>"`, если генератор сам `--chdir` не дал (не даёт). SCM стартует из System32; без `--chdir` Lua не находится. Смена `engine` или Game Filter при установленной службе — снова **Установить службу**.

GUI и CLI строку флагов не разбирают. Нет exe — отказ.

Одинаковые пакеты на winws и winws2 не обещаем (у z2 другие range, payload, empty ACK). Одинаковые поля JSON — да. Проверка — тесты на выбранном `engine`.

## JSON → argv

Стратегия v2 (см. «Формат v2») разворачивается в профили. Генератор — [`internal/spec`](../internal/spec/), `spec.Build`. Lua только stock `bin/lua/zapret-lib.lua` и `bin/lua/zapret-antidpi.lua`. Свой `.lua` не подключать. Lua-атаку в флаги winws не переводить.

Неизвестный `desync` — throw. Не добавлять имя атаки вне таблицы.

Проверка JSON одна для обоих движков. Имена `desync`, `fooling`, `l7`, `cutoff` и префиксы путей — с учётом регистра (`fake`, не `FAKE`; `lists:`, не `LISTS:`). `engine` — ровно `winws` или `winws2`. Ошибка сборки:

- нет `profiles` или список пуст;
- в `l7` имя не из `http`, `tls`, `quic`, `wireguard`, `dht`, `discord`, `stun`, `unknown`, или вместе `quic` и `tls`;
- `cutoff` не `nN` и не `none`;
- путь без префикса или без имени файла после префикса;
- пустой элемент в `hostlistDomains` / `hostlistExcludeDomains`;
- значение fake не `!`, не `0x<hex>` и не путь с префиксом, или в нём пробел.

Префиксы путей (регистр как здесь):

| Префикс | Куда |
|---|---|
| `lists:` | `lists/` |
| `user:` | `user/` |
| `cat:` | корень каталога стратегий (сток — `strategies/`); путь через `/`, без `..`, например `cat:fakes/stun.bin` |
| `active:<ИМЯ>` | fake, который пользователь может подменить: `user\fakes\ACTIVE_<ИМЯ>.bin`, если сборке сказано, что файл есть (`Paths.ActiveFiles`; сборщик сам диск не читает), иначе дефолт из `active` в `catalog.json`. Имя — `A-Z`, `0-9`, `_`. Имя blob winws2 — `ACTIVE_<ИМЯ>` |
| `bin:` | `bin/` — только файлы движка; в сток-каталоге не используется (тест `TestCatalogSelfContained`) |

### Формат v2

`strategies/` — сток-каталог (JSON Schema — `strategies/schema/`, генерируется из структур Go: `go generate ./internal/spec`; у каждого файла `$schema` с относительным путём). Каталоги пользователя и внешние будут с той же раскладкой (пока не сделано):

```text
strategies/
  catalog.json        { "id", "title", "spec": 2, "active": {...}, "files": {...} }
  fakes/*.bin          fake и паттерны (cat:fakes/<файл>)
  lists.json  targets.json  actions.json   кирпичи: общие для всех стратегий каталога
  flowseal/<имя>.json  порты .bat из Flowseal/zapret-discord-youtube
  zapman/<имя>.json    свои стратегии (папки пока нет: создать с первой стратегией)
  games/<id>.json      игровые модули
  schema/              JSON Schema
```

Происхождение стратегии (`Origin`: `flowseal` / `zapman`) — папка, не поле JSON. Имя стратегии — имя файла без `.json`; одно имя в двух папках — ошибка загрузки. Другой `*.json` в корне каталога (кроме `catalog.json` и кирпичей) — ошибка: стратегия должна лежать в папке своего происхождения.

| Файл | Что внутри |
|---|---|
| `catalog.json` | `id` (имя `a-z`, `0-9`, `-`), `title`, `spec` = 2; `active` — `{ "<ИМЯ>": "cat:fakes/<файл>" }`, дефолт каждого `active:<ИМЯ>`; `files` — `{ "<путь через />": "<sha256 hex>" }` для каждого файла каталога, кроме `catalog.json` и `schema/` (генерируется, см. ниже) |
| `lists.json` | именованные наборы списков: `hostlist`, `hostlistExclude`, `ipset`, `ipsetExclude` (пути `lists:`/`user:`/`cat:`) |
| `targets.json` | именованные цели: `tcp`, `udp`, `l3`, `l7`, `lists` (имена наборов с `hostlist`/`ipset`), `exclude` (наборы с `*Exclude`), `domains`, `excludeDomains`, `anyProtocol`, `payload` |
| `actions.json` | именованные действия: `desync` и поля атаки из таблиц ниже; `cutoff` — число N (= `nN`) или `"none"` |
| `flowseal/<имя>.json`, `zapman/<имя>.json` | стратегия: `"spec": 2`, версия (`upstream` + `rev` или `version`, см. «Версии»), `about`, `rules[]` — пары `{ "target", "action" }` в порядке winws (первое совпадение выигрывает) |
| `games/<id>.json` | игровой модуль: `id` (= имя файла), `title`, `version`, `order`, `updater`, свои `lists` (с `seed`), `rules[]` |

`target` и `action` в правиле — имя из каталога или объект прямо в правиле. Цель выносится в `targets.json`, если используется 2+ раза. Действие выносится в `actions.json` (пресет), если используется 3+ раза; остальные действия — прямо в правиле. Пресет с одним-двумя пользователями не заводить: при правке такой стратегии действие возвращается в правило. Имена — `a-z`, `0-9`, `-`. Загрузка строгая: неизвестное поле, имя, порт, отсутствующий файл `cat:` (или `bin:`, если загрузчику дана папка `bin`), `active:<ИМЯ>` без дефолта в `catalog.json` — ошибка. Набор игры не может повторять имя из `lists.json`; `seed` — путь `lists:`, из него создаётся `user:`-список, если его нет.

Правило → профиль v0.1 → тот же генератор, что и раньше. `payload` для winws2 выводится из `l7` / `anyProtocol` (например, `tls`+`http` → две группы, `discord` → `discord_ip_discovery,stun`); явный `payload` пишут только когда вывод не подходит.

`wf` не задаётся руками: это объединение портов всех целей (стратегии и включённых игр), без повторов, по возрастанию первого порта; `$gameFilter` (порты Game Filter, `12` или `1024-65535`) — последним. winws: `--wf-tcp=` / `--wf-udp=`, winws2: `--wf-tcp-out=` / `--wf-udp-out=`.

Игры идут **до** правил стратегии (они уже, чем общие правила на тех же портах), по `order`, затем по `id`. Сейчас: `sky` (10), `sker` (20), `photon` (30).

Одноразовый конвертер v0.1 → v2 — [`dev/convert-v2`](../dev/convert-v2/); он сверяет argv до записи. Модель v0.1 (`V01Spec`) осталась внутри Go как промежуточная. Конвертер берёт `upstream` из git (`-git`, `-ref`, по умолчанию `upstream/main`) и пишет `rev: 1`, игре — `version: 1`; fake он пишет как `bin:`. Его тест сравнивает вывод с `strategies/` коммита `9f9aeed` (до переноса fake в `strategies/fakes/`) при `rev` = 1 и пропускается без `upstream/main`.

Каталог самодостаточен: всё, на что ссылаются его JSON (кроме списков `lists:` / `user:`), лежит в нём, поэтому прошлый тег `strategies/` можно подключить как внешний каталог. `files` в `catalog.json` — SHA256 байтов каждого файла, как они лежат в git (`.gitattributes`: `strategies/**/*.json` — `eol=lf`, `*.bin` — `binary`, чтобы checkout не менял байты). После любой правки файла каталога (JSON, `.bin`) — `go generate ./internal/spec`: он пишет схему и `files`. Иначе `TestCatalogFiles` падает с `run go generate ./internal/spec`.

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

Перенос `.bat` в JSON — [`dev/import-flowseal`](../dev/import-flowseal/):

```text
go run ./dev/import-flowseal [-git .] [-ref upstream/main] [-dir strategies] [-write]
```

Читает `general*.bat` из `-ref` через `git show` (без checkout): склеивает строки с `^`, раскрывает `set "X=…"`, `%GameFilterTCP%` / `%GameFilterUDP%` → `$gameFilter`, режет профили по `--new`. Каждый профиль — обратная таблица генератора winws: списки — в бандлы `lists.json`, цель и действие — имя из `targets.json` / `actions.json`, если JSON тот же, иначе inline; fake `bin\X` → `cat:fakes/X`, `bin\ACTIVE_X.bin` → `active:X`. Неизвестный флаг или значение — ошибка с именем `.bat` и флага. Затем round trip: argv нового файла (`spec.Build`, winws) должен совпасть с argv `.bat` (порты `wf` — множества, порядок флагов в профиле свободный, порядок значений одного флага и `fake-tls` / `fake-tls-mod` важен).

Статусы: `same` (argv текущего `flowseal/<имя>.json` = `.bat`), `changed` (ниже — строки различий «ours / Flowseal»), `new`, `removed` (файл есть, `.bat` нет; файл не удаляется), `error`. Код выхода: 1 при ошибке, 2 если есть не `same` и файл не записан или `removed`, иначе 0. `-write` пишет `flowseal/<имя>.json` для `changed` и `new`: `upstream` — тег `.bat`, `rev: 1`, `about` прежний; `same` не трогает (байт в байт). После записи — `go generate ./internal/spec` (и golden).

Две наши добавки v0.1 для winws2 импорт выводит сам ([`derive.go`](../dev/import-flowseal/derive.go)) и пишет в правила; при сравнении они добавляются и к argv `.bat`, любое другое различие — как раньше. На winws результат тот же.

`--filter-l7` — только если в профиле `.bat` нет ни `--filter-l7`, ни `--dpi-desync-any-protocol`. Должно подойти ровно одно правило, иначе ошибка с именем `.bat` и номером профиля:

| Фильтр | Порты | `fake.http` | `--filter-l7` |
|---|---|---|---|
| только `--filter-udp` | `443` | — | `quic` |
| только `--filter-tcp` | из `80, 443, 2053, 2083, 2087, 2096, 8443` | нет | `tls` |
| только `--filter-tcp` | те же, есть `80` | есть | `tls,http` |

`--dpi-desync-fake-tls=!` (встроенный hello) — если есть `--dpi-desync-fake-tls-mod`, нет `--dpi-desync-fake-tls` и нет `--dpi-desync-any-protocol`: winws берёт этот hello неявно, у winws2 неявного TLS fake нет. Профили Game Filter с `any-protocol` hello не получают (иначе он уйдёт на все payload).

На 1.10.4 и `flowseal/processed` все 22 файла — `same`; `TestImport` (пропуск без git или тега `1.10.4`) это проверяет.

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
| `seqovlPattern` | `--dpi-desync-split-seqovl-pattern=` | blob + `seqovl_pattern=`. На обоих движках только путь с префиксом (`cat:`/`active:`/`lists:`/`user:`/`bin:`) |
| `ipId` | `--ip-id=` | `ip_id=` |
| `tlsMod` | `--dpi-desync-fake-tls-mod=` после `fake-tls` | `tls_mod=` на последнем ref слота `tls`, только на вызове для `tls_client_hello`. Без слота `tls` — не пишется |
| `hostfakesplitMod` | `--dpi-desync-hostfakesplit-mod=` как в JSON | куски через запятую; **`altorder=*` не писать** (в stock Lua нет) |
| `fakedsplitPattern` | `--dpi-desync-fakedsplit-pattern=` (путь, не сырой `cat:`) | `pattern=` / blob |

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
| значение `!` | как в JSON (встроенный hello z1) | `cat:fakes/tls_clienthello_www_google_com.bin` — файл должен быть в каждом каталоге |
| TLS-профиль с `fake` в `desync`, слота `tls` нет | z1 берёт встроенный hello | ошибка сборки: нет blob для группы. Неявного hello нет: пишите `"tls": ["!"]` |
| `payload: "all"` / `anyProtocol`, слота `tls` нет | z1 берёт встроенный hello для TLS | TLS hello не подставляется |
| группа из нескольких слотов (`all`, `discord,stun`), нет `luaPayload` | fake по типу пакета: TCP `http`/`tls`/`unknown`, UDP `quic`/`discord`/`stun`/`unknownUdp` | каждый слот — свой `payload=` у `fake`: `tls` → `tls_client_hello`, `http` → `http_req`, `quic` → `quic_initial`, `discord` → `discord_ip_discovery`, `stun` → `stun`, `unknown` → `~empty,http_req,tls_client_hello`, `unknownUdp` → `~empty,quic_initial,wireguard_initiation,dht,discord_ip_discovery,stun`. Один ref в нескольких слотах — один вызов, фильтры через запятую. Слоты другого L4 (UDP-слот в TCP-профиле) не пишутся. `unknown` / `unknownUdp` в профиле с `tcp` и `udp` — ошибка сборки |
| hex `0x00000000` | как есть | имя blob `hex_00000000`, не `0x…` |
| порядок | `fake-tls` **до** `fake-tls-mod` | `tls_mod` только на последний ref слота `tls` в группе `tls_client_hello` или с фильтром `tls_client_hello`; этот вызов не сливается с другими слотами. На `!` из `unknown` и других слотов — нет |

На z2 в argv не должно быть `--dpi-desync`. На z1 не должно быть `--lua-desync`. `--blob=` только `name:value` (двоеточие).

## Игровые модули

Игровые модули (`strategies/games/`) не зависят от Game Filter:

- `sky` — Sky: UDP 5000–8000 с `user:ipset-sky.txt`, QUIC-fake, TTL 8, cutoff 4; TCP 443 для `live.ws.radiance.thatgamecompany.com` с multisplit на позиции 1, `luaPayload: all`, без cutoff. Список создаётся из `lists/ipset-sky.default.txt`; трей (`updater: sky-log`) дополняет его только конкретными IPv4 `/32` из `Sky.log`. DNS входа — отдельная настройка, стратегия `hosts` не меняет.
- `sker` — Sker Ritual и Hazard Levels (тот же разработчик): `user:ipset-sker.txt`, fake `active:GAME_UDP`, `luaPayload: all`, TTL 8, cutoff 4. Fake без TTL доходил до сервера Photon, и игра теряла соединение (`ClientTimeout`) через ~10 с после входа в матч. Список создаётся из `lists/ipset-sker.default.txt` (подсети Photon Cloud EU на G-Core и Vultr) и сам не обновляется. Те же подсети у PEAK в регионе EU: он тоже попадает сюда.
- `photon` — общий профиль Photon (UDP 5055–5058, 27000–27002); стоит после `sker`.

Бывшая `general (ALT11 Photon)` = `general (ALT11)` + `sky`, `sker`, `photon`; отличие только в порядке профилей и портов `wf`.

Свойства профилей игр на `general (ALT11)` проверяет `TestGames` (`internal/spec`); `-update` golden-файлов их не сотрёт.

Проверки перед коммитом (golangci-lint и govulncheck из [`tools/go.mod`](../tools/go.mod), `go vet ./...`, `go test ./...`) — [`AGENTS.md`](../AGENTS.md). Макеты [`docs/ui/`](ui/index.md) сделаны PowerShell-версией (`main`); на ветке `go` генератора нет.

## Пакеты среды выполнения

Библиотеки для будущих CLI и GUI (Wails). Своего `main` у них нет.

### `internal/config`

`user/config.json`. `Load`: нет файла — `Default()`; не JSON — дефолты и ошибка; поле с плохим значением остаётся дефолтным, ошибка (`FieldError`) называет поле. `Save` проверяет поля (`Validate`) и пишет атомарно: временный файл в `user/`, затем rename. Неизвестные поля сохраняются и пишутся после известных.

| Поле | Значения | Дефолт |
|---|---|---|
| `language` | `ru`, `en`, `""` (язык Windows) | `""` |
| `engine` | `winws`, `winws2` | `winws` |
| `strategy` | имя стратегии каталога, `""` — не выбрана | `""` |
| `games` | ID игровых модулей, которые включены (`sky`, `sker`, `photon`) | `[]` |
| `gameFilter` | `disabled`, `all`, `tcp`, `udp` | `disabled` |
| `autoUpdateCheck` | bool | `true` |
| `trayWatch` | bool | `true` |
| `autoUpdateBricks` | bool: свои стратегии берут новые сток-кирпичи после обновления | `true` |

Файл PowerShell-версии (`language`, `engine`, `gameFilter`, `autoUpdateCheck`, `trayWatch`) читается как есть. Строки `"yes"` / `"1"` вместо bool PowerShell принимал, Go — нет (ошибка поля); сам PowerShell всегда писал bool.

## Релиз

Push аннотированного тега `vMAJOR.MINOR.PATCH` на `origin` сам запускает [Release](../.github/workflows/release.yml) **с того коммита** (YAML в теге). Job собирает zip/rar/tar.gz и создаёт **черновик**. `/releases/latest` не меняется, пока не нажмёте **Publish release**.

1. `ModuleVersion` в `src/Zapman/Zapman.psd1` (ветка `main`; на ветке `go` `release.yml` ещё собирает PowerShell-версию и не работает). Если менялась схема JSON — `spec` в `strategies/`.
2. Секция в [`CHANGELOG.md`](../CHANGELOG.md) на этот тег, коммит **с актуальным** `release.yml`.
3. Аннотированный тег `v` + ModuleVersion **на этом коммите**:
   `git tag -a v0.1.1 -m v0.1.1`
4. `git push origin main` и `git push origin v0.1.1`. Не `git push --tags`: в клоне могут быть чужие теги вроде `1.10.2` без `v`.
5. Дождаться проверок на `main` и зелёного job Release.
6. Releases → черновик → проверить архивы → **Publish release**.

Повтор без нового тега (тот же YAML с `main`):

```text
gh workflow run Release --repo NONERUN/zapman --ref main -f tag=v0.1.1
```

Всегда `--repo NONERUN/zapman`: `gh` в этой копии может смотреть на Flowseal. Не создавать Release вручную в UI без этого workflow — не будет zip/rar/tar.gz.

Пока черновик — его можно удалить в UI и прогнать job снова. После Publish ассеты **immutable**, повтор job их не трогает.

Auto-Update Check смотрит **git-теги**, не черновики: тег на origin уже виден клиенту. Скачать «latest» он зовёт на опубликованный релиз.
