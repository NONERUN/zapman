# Разработка

Раскладка, проверки и что не делать в коде — [`AGENTS.md`](../AGENTS.md). Этот файл — поля JSON, обновление `bin/` и GitHub Release. Правка генератора и этого файла — в одном изменении.

Версия продукта — git-тег `vMAJOR.MINOR.PATCH`; workflow Release вшивает его в exe (раздел «Релиз»). На `main` — `ModuleVersion` в `src/Zapman/Zapman.psd1`, тег `v` + это число. Формат стратегий — `"spec": 2` (раздел «Формат v2» ниже); `Version` в [`internal/spec`](../internal/spec/).

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

`wf` не задаётся руками: это объединение портов всех целей (стратегии и включённых игр), без повторов, по возрастанию первого порта; `$gameFilter` (порты Game Filter: `12`, если фильтр выключен для протокола, иначе `gameFilterTcp` / `gameFilterUdp` из `user/config.json`) — последним. winws: `--wf-tcp=` / `--wf-udp=`, winws2: `--wf-tcp-out=` / `--wf-udp-out=`.

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
| `gameFilterTcp`, `gameFilterUdp` | порты Game Filter: `N` или `N-M` через запятую, 1–65535, начало ≤ конца (правила Flowseal `service.bat`, коммит `2384961`); пробелы убираются при чтении | `1024-65535` |
| `autoUpdateCheck` | bool | `true` |
| `trayWatch` | bool | `true` |
| `autoUpdateBricks` | bool: свои стратегии берут новые сток-кирпичи после обновления | `true` |

Файл PowerShell-версии (`language`, `engine`, `gameFilter`, `autoUpdateCheck`, `trayWatch`) читается как есть. Строки `"yes"` / `"1"` вместо bool PowerShell принимал, Go — нет (ошибка поля); сам PowerShell всегда писал bool.

### `internal/engine`

- `NewLayout(root)` — папки `bin/`, `strategies/`, `lists/`, `user/`. `Paths()` даёт `spec.Paths`; `ActiveFiles` — имена `X` файлов `user/fakes/ACTIVE_<X>.bin` (префикс и `.bin` без учёта регистра, `X` — имя active-fake).
- `Exe(engine)` — нет exe выбранного движка — ошибка, другой exe не берётся.
- `EnsureUserFiles(cat, games)` создаёт недостающие `user:`-списки бандлов каталога и включённых игр, существующие не трогает. Содержимое: `seed` игры; иначе `lists/<имя>.default.txt` (так `ipset-all.txt`); иначе одна неиспользуемая запись (`domain.example.abc` / `203.0.113.113/32`): пустой список совпадает со всеми хостами.
- `CheckBin(bin)` сверяет файлы с `bin/versions.json` без сети: `ok` / `missing` / `hash mismatch`; `winws.exe` или `winws2.exe` нет в `versions.json` — `not in versions.json`. SHA256 `.lua` — после CRLF → LF, как в `dev/update-zapret`.
- `Build(cat, cfg)` — стратегия, игры, Game Filter из конфига → `Build`. `CommandLine()` = `"<bin>\<exe>" <строки argv через пробел>`; для winws2 первым идёт `--chdir="<bin>"`. Та же строка — ImagePath службы. Длиннее 32766 символов UTF-16 (предел `CreateProcess`) — ошибка.
- `Runner.Run(build)` — запуск на переднем плане, cwd = `bin/`, без окна, вывод — хвост 64 КБ. Выход в первые 2 с — ошибка (`ExitError`). Код `0xC0000142` (`STATUS_DLL_INIT_FAILED`) при первом запуске — пауза 1 с и один повтор; снова он — ошибка с кодом и советом про исключения антивируса. Запуск процесса (`Runner.Start`) подменяется в тестах. Что служба не запущена, проверяет вызывающий.

### `internal/service`

Служба `zapret` (то же имя, что у PowerShell-версии и Flowseal `service.bat`). Она запускает `winws.exe` / `winws2.exe` напрямую: ImagePath = `Build.CommandLine()` как есть, без второго экранирования (`CreateService` вызывается напрямую, не через `mgr.CreateService`). Вызовы SCM, реестра и процессов — за интерфейсом `Host`; тесты используют подделку и не трогают службы машины.

- `PlanFor(build)` → `Plan{Strategy, Version, Games, ImagePath}`. `Install(plan)` — шаги PowerShell-версии: копия старой записи → стоп службы и всех `winws` / `winws2` → удаление и ожидание, пока служба пропадёт (8 с) → создание (8 попыток через 400 мс, автозапуск, описание `Zapret DPI bypass software`) → запись значений и сверка чтением → старт и ожидание `Running` (15 с) и ещё 2 с. Ошибка после удаления — новая служба удаляется, старая создаётся снова и запускается, если работала до установки (остановленная остаётся остановленной). Код выхода `0xC0000142` — один повтор старта. ImagePath длиннее 32766 символов, без кавычек у exe или без стратегии — ошибка до любых вызовов.
- `Remove()` — стоп службы, стоп `winws` / `winws2`, удаление `zapret`, затем стоп и удаление `WinDivert` и `WinDivert14` (в этом порядке, иначе драйвер остаётся в `STOP_PENDING`). Задачу трея не трогает.
- `Start()` — уже запущена — ничего; ImagePath не меняется. `Stop()` — стоп и ожидание `Stopped`.
- TCP timestamps (RFC 1323): без них не работает `fooling=ts` (многие сток-стратегии), в Windows они по умолчанию выключены. `TCPTimestamps()` — только чтение, для диагностики, без прав администратора. `EnableTCPTimestamps()` включает их (`netsh interface tcp set global timestamps=enabled`), если они не включены или состояние неизвестно; его вызывают `Install` (до любых изменений; ошибка — служба не трогается) и `Start`, перед запуском на переднем плане — вызывающий. Документированного API или значения реестра для текущего состояния нет, поэтому читается вывод `netsh interface tcp show global` (`ParseTCPTimestamps`): строка с `timestamps` или `RFC 1323` (английский и русский вывод), значение после последнего `:` — `enabled` / `disabled`, иначе ошибка. Вывод может быть в OEM-кодировке; смотрятся только ASCII-части.
- `Snapshot()` — точная копия службы: ImagePath, имя, описание, тип запуска (с отложенным автозапуском), все значения записи (и `zapret-discord-youtube`), работала ли. `Restore(snapshot)` создаёт её снова такой же и запускает, если работала; служба из копии — и чужая (`service.bat`). Ими пользуются тесты стратегий; `Install` берёт ту же копию для отката.
- `Install`, `Remove`, `Start`, `Stop`, `Restore` без прав администратора — `ErrNotElevated` до любых вызовов. `Status` и `Snapshot` прав не требуют.

Служба не меняется до переустановки: Game Filter, движок, стратегия, active-fake и папка zapman — в ImagePath. Запись о стратегии — значения в ключе службы `HKLM\SYSTEM\CurrentControlSet\Services\zapret`:

| Значение | Тип | Что |
|---|---|---|
| `zapman` | REG_SZ | имя стратегии (его же пишет PowerShell-версия) |
| `zapmanVersion` | REG_SZ | `Strategy.Version()`, например `Flowseal 1.10.4 r3` |
| `zapmanGames` | REG_MULTI_SZ | включённые игры с версиями, например `sky 2` |
| `zapmanHash` | REG_SZ | SHA256 (hex) ImagePath |

Значения удаляются вместе со службой, читаются без прав администратора и не видны в описании службы. Старое значение `zapret-discord-youtube` при установке удаляется; его читает только `Snapshot`, чтобы вернуть службу как была.

`Status(want)` — `Installed`, `State`, `ExitCode`, `Engine` (из ImagePath; `winws2.exe` проверяется первым), запись, `Edited` (хеш не совпадает с ImagePath), `Outdated` и `Reason` — первая подходящая причина:

| `Reason` | Когда |
|---|---|
| `foreign` | нет `zapman`: службу создала другая программа (например `service.bat`) |
| `strategy` | выбрана другая стратегия |
| — | ImagePath совпадает с планом: не устарела |
| `version` | другая версия стратегии или игр |
| `legacy` | нет хеша: службу ставила PowerShell-версия, ImagePath другой |
| `edited` | ImagePath поменяли после установки |
| `argv` | версии те же, другая командная строка (движок, Game Filter, active-fake, папка) |

### `internal/tools/...`

Инструменты GUI. Типы `internal/gui/backend` они не импортируют (иначе цикл импорта): у них свои типы с теми же полями. Программы, сеть, ping и система — за полями структур и `command.Func`; тесты без сети, прав администратора и процессов. Отмена `ctx` — ошибка с `errors.Is(err, context.Canceled)`.

- `command` — `Run(ctx, exe, args...)`: запуск без окна, вывод (stdout + stderr) из OEM-кодировки в UTF-8, код выхода в `Result.Code`; ненулевой код — не ошибка. `System32(name)` — путь в `%SystemRoot%\System32`.
- `netreset` — `Reset`: `netsh winsock reset`, `netsh int ip reset all`, `netsh winhttp reset proxy`, `ipconfig /flushdns`, лог как у PowerShell-версии; шаг с ошибкой — `ErrFailed`, остальные шаги выполняются. `Reboot` — `shutdown /r /f /t 0`.
- `web` — `Client`: `LatestVersion` (теги GitHub, 8 с), `DownloadIPSet` (в `user/ipset-all.txt`, атомарно, с прогрессом), `CheckHosts` (первая и последняя строка шаблона Flowseal в system hosts), `OpenHosts` (Notepad).
- `diag` — `Checker.Report`: 18 проверок PowerShell-версии (ID, тексты, порядок); `Checker.Do` — действия `remove-services`, `remove-windivert`, `discord-cache`. Система — интерфейс `System` (`NewSystem()` — настоящая).
- `stratest` — тесты стратегий без службы (`Invoke-ZapmanStrategyTestsCore`). `New(layout, catalog, config)` — настоящий `Tester`; `Run(ctx, Request, Sink)`. Проверки до тестов: права администратора, `curl.exe` (PATH, затем System32), служба `zapret` не установлена. Каждая стратегия: стоп всех `winws` / `winws2`, TCP timestamps, `Layout.Build` + `Runner.Run` (готовность — окно старта 2 с), тесты, стоп. Стандартные тесты — 17 целей, curl HTTP / TLS1.2 / TLS1.3 и ping (ICMP, IPv4); DPI — набор TCP 16-20 [hyperion-cs/dpi-checkers](https://github.com/hyperion-cs/dpi-checkers) (`MONITOR_HOST` — одна своя цель без загрузки набора). Лучшая стратегия: больше `OK`, при равенстве — больше удачных ping (как в PowerShell-версии; стратегия с 0 `OK` тоже может быть лучшей). Файл `test-results/test_results_<дата>.txt` (UTF-8 с BOM, CRLF). Режим IPSet и возврат службы и движка, что работали до тестов, — на вызывающем (`internal/app`, `RunTests`).

Ручная проверка на Windows: `go run ./.local/toolscheck` (без прав — версия, hosts, отчёт диагностики, ping); `go run ./.local/toolscheck -tests standard -strategies "general"` из консоли администратора — тесты стратегий (служба `zapret` должна быть удалена). Папка `.local/` в `.gitignore`.

## GUI (Wails)

GUI на [Wails v3](https://v3.wails.io/) `v3.1.0` (версия закреплена в `go.mod`). CLI `wails3` не нужен, npm тоже: хватает `go build`.

```text
go build -o zapman.exe ./cmd/zapman   # exe в корень репозитория: рядом bin/, strategies/, lists/
.\zapman.exe                          # главное окно (UAC: exe просит права администратора)
.\zapman.exe --tray                   # старт в трее: главное окно скрыто, если включена «Иконка в трее»
.\zapman.exe -fake                    # фейк в памяти: службы, winws и система не трогаются
.\zapman.exe -root D:\zapman          # другая папка с bin/, strategies/, lists/, user/
```

Корень — папка `zapman.exe`. У `go run` exe лежит во временной папке `go-build…`, поэтому корень — рабочая папка. Без прав администратора (например, `-fake` в обычном терминале): `$env:__COMPAT_LAYER = 'RunAsInvoker'` перед запуском — Windows не смотрит на `requireAdministrator`; действия со службой и winws тогда дают «нужны права администратора».

Манифест и иконка exe — `cmd/zapman/rsrc_windows_amd64.syso` (в git). Его делает [go-winres](https://github.com/tc-hib/go-winres) (закреплён в `tools/go.mod`) из `cmd/zapman/winres.json`: `requireAdministrator`, Common Controls 6, DPI per-monitor v2, версия файла, `assets/app.ico` с ID 3 (этот ID грузят Wails и трей). После правки `winres.json` или `app.ico`:

```text
go generate ./cmd/zapman
```

go-winres, а не `akavel/rsrc`: тот же `tc-hib/winres` использует `wails3 generate syso`; манифест — поля JSON, а не XML руками; иконка и версия — в том же файле. CLI `wails3` в `tools/go.mod` тянуть не нужно. Из-за манифеста тестовый бинарник `cmd/zapman` тоже просил бы права, поэтому тестов в `cmd/zapman` нет: логика — в `internal/app` и `internal/gui`.

| Путь | Роль |
|---|---|
| `cmd/zapman/` | Точка входа: окна Wails, диалоги, трей (`shell.go` — реализация `gui.Shell`) |
| `cmd/zapman/frontend/` | Страницы: простые HTML, CSS и JS (ES-модули), по странице на окно; `index.html` — главное окно |
| `internal/gui/` | Логика окон PS GUI (`src/gui` на `main`) без Wails: виды окон, кнопки, тексты, трей. Тесты — с фейковым `Shell` |
| `internal/gui/backend/` | Интерфейс `Backend` (всё, что GUI просит у движка, службы и конфига) и детерминированный фейк в памяти |
| `internal/gui/i18n/` | Строки RU / EN (копия таблиц `Ui.ps1`) и строки, которых нет в PS-версии |
| `internal/app/` | Настоящий `Backend` поверх `internal/config`, `internal/engine`, `internal/service` и `internal/tools` (`wired.go`); задача трея `zapman-tray` (`schtasks.exe`) |
| `assets/assets.go` | `app.ico` для окон и трея (`go:embed`) |

Как это связано:

- `frontend/` вшит в бинарник (`//go:embed all:frontend`) и отдаётся через `application.AssetFileServerFS`. Этому серверу нужен `index.html`, поэтому главное окно — `index.html`, остальные — `<имя окна>.html`.
- Рантайм Wails отдаёт сам asset server по адресу `/wails/runtime.js`. Страницы импортируют его как ES-модуль (`import { Call, Events, Clipboard } from "/wails/runtime.js"` в `common.js`); копии рантайма в репозитории нет.
- `gui.App` зарегистрирован как сервис Wails. JS зовёт его методы по имени: `Call.ByName("github.com/NONERUN/zapman/internal/gui.App.<Метод>", ...)`. Методы — виды окон (`Main`, `StrategyView`, `TestsView`, …) и действия (`ClickStart`, `StrategyRun`, `CloseDialog`, …). Длинная работа идёт в горутине; метод сразу возвращается.
- Когда вид меняется, Go шлёт событие без данных (`main`, `strategy`, `tests`, `download`, `diag`; `lang` — смена языка), и страница заново читает свой вид. Все решения — в Go; JS только рисует вид.
- Модальных окон в Wails нет: пока открыт диалог, остальные окна выключены (`SetEnabled(false)`), как у `ShowDialog` в WPF. Какое окно включено, решает только `gui.Modal` (стек диалогов и счётчик окон сообщений; включено одно окно — последний диалог, без диалогов — главное; пока открыто окно сообщения — ни одно). `shell.sync` в `cmd/zapman` — единственное место с `SetEnabled`: после каждого открытия, закрытия, отказа в закрытии и после каждого окна сообщения он заново выставляет состояние всех окон. Все пути закрытия диалога (кнопки, крестик, `Stop`) идут через `closeDialog`: сначала закрывается окно, потом просыпается поток диалога. `TestModalPaths` проходит каждый диалог до ошибки, отмены и крестика и проверяет, что главное окно снова включено и не «занято»; диалог без такого случая (страница в `frontend/` без теста) роняет тест. Закрытое окно уничтожается и создаётся заново при следующем открытии.
- Один процесс — и окна, и трей (в PS-версии трей — отдельная задача планировщика). «Иконка в трее» показывает или прячет иконку; второй запуск открывает главное окно (с `--tray` — ничего не делает).

### `internal/app`

`app.New(Options)` — настоящий `Backend`; его создаёт `cmd/zapman/main.go` (`newBackend`). Пакет отдельно от `internal/gui/backend`: там остаются интерфейс и фейк без вызовов системы, а SCM, процессы и `schtasks.exe` — только здесь.

- Настройки — `user/config.json` через `internal/config`; неизвестные поля и поля, которых нет в GUI, сохраняются. Плохой файл — дефолты и ошибка (`ConfigError`); «Запуск» и «Установка» его не перезаписывают.
- `CheckEngine` — exe (у winws2 и `lua/`) есть и SHA256 файлов движка совпадают с `bin/versions.json` (`engine.CheckBin`; файлы другого движка не смотрятся). Иначе — текст `EngineBinBad` со списком `путь: состояние`.
- «Запуск» — проверка exe и сборки, стоп службы и всех `winws` / `winws2`, TCP timestamps, `Runner.Run` на переднем плане. Ошибка старта — текст `EngineStartFailText` с выводом движка. Процесс остаётся после выхода GUI (как в PS-версии: её GUI тоже не останавливает `winws` при закрытии); после перезапуска GUI он — «чужой» `winws`.
- «Установка» — `service.Install(PlanFor(build))`; стратегия и движок пишутся в конфиг после успеха. Ошибка — текст `ServiceInstallFail` с текстом ошибки (в PS-версии — сырой текст исключения).
- `SuspendService` / `RestoreService` (тесты стратегий) — `service.Manager.Snapshot`, удаление службы и WinDivert; затем `Restore` той же копии. Служба возвращается точно такой, как была, и чужая тоже; запускается, только если работала. Ошибка — текст `ServiceRestoreFail`.
- Fake — список `strategies/fakes/*.bin`; активный — `user/fakes/ACTIVE_DISCORD_UDP.bin` / `ACTIVE_GAME_UDP.bin` (в PS-версии — в `bin/`). Выбор = копия файла; совпадение ищется по SHA256.
- «Иконка в трее» при входе — задача планировщика `zapman-tray` (имя PS-версии): `schtasks /Create /SC ONLOGON /RL HIGHEST /F /TR "\"<zapman.exe>\" --tray"`; выключение — `/Delete`, если задача есть.
- Сеть, hosts, загрузки, диагностика и тесты стратегий — интерфейс `app.Tools`. `app.NewTools(b)` (`wired.go`) — реализация на `internal/tools`; `newBackend` передаёт её в `Backend.SetTools`. Без неё (`Options.Tools` пустой) методы дают `NotWired` с именем метода. `LatestVersion` без тега в ответе GitHub — пустая строка без ошибки (GUI пишет «не удалось проверить»). Действия диагностики со службами требуют прав администратора.
- `RunTests` — как `Tests.ps1`: `CheckEngine`, копия и удаление службы (если GUI не сделал этого раньше), стоп процесса «Запуска», недостающие `user:`-списки. Для DPI-тестов режим IPSet — `any`: `user/ipset-all.txt` копируется в `user/ipset-all.test-backup.txt`. Копия, оставшаяся от прерванных тестов, возвращается до начала. В тесты идут только стратегии, у которых есть флаги для движка (правило `StrategyInfo.Supports`). После тестов, и при ошибке или отмене: ipset, затем служба, затем «Запуск» той же стратегии (без записи в конфиг). Пока идут тесты, остальные действия со службой и движком ждут.

## Релиз

Push аннотированного тега `vMAJOR.MINOR.PATCH` на `origin` сам запускает [Release](../.github/workflows/release.yml) **с того коммита** (YAML в теге). Job `build` (windows-latest) гоняет `go test ./...` и собирает `zapman.exe`; job `release` (ubuntu-latest) кладёт его в zip/rar/tar.gz и создаёт **черновик**. `/releases/latest` не меняется, пока не нажмёте **Publish release**.

Версия продукта — только git-тег, в файлах репозитория её нет. Сборка релиза:

- `go-winres make ... --file-version <тег без v> --product-version <тег без v>` в `cmd/zapman` — версия файла в свойствах exe (в git `.syso` с `0.0.0.0`; манифест не меняется);
- `go build -tags production -trimpath -ldflags "-s -w -H windowsgui -X github.com/NONERUN/zapman/internal/app.Version=<тег>" ./cmd/zapman`. `-tags production` выключает devtools WebView2, адрес dev-сервера из переменной среды и отладочный лог Wails; `-H windowsgui` — без окна консоли. CLI `wails3` и npm не нужны: `frontend/` вшит через `go:embed`.

`app.BuildVersion()` (заголовок окна, «Статус», проверка версии): `app.Version` из `-X`, если задан; иначе версия модуля из build info (у `go build` в git-копии — тег или псевдоверсия, `+dirty` при правках); иначе `dev-<коммит>`. Проверка версии сравнивает её с последним тегом GitHub как строку, поэтому в `-X` — тег целиком, с `v`.

В архиве: `zapman.exe`, `bin/`, `strategies/`, `lists/`, `docs/`, `README.md`, `LICENSE.txt`, `CHANGELOG.md`, `assets/app.LICENSE.txt` (лицензия иконки; сама иконка — в exe). `.service/` не нужен: «Скачать ipset» и «Сверить hosts» берут файлы с GitHub (ветка `main`). `user/` и `test-results/` exe создаёт сам.

1. Если менялась схема JSON — `spec` в `strategies/`.
2. Секция `## vX.Y.Z - <дата>` в [`CHANGELOG.md`](../CHANGELOG.md) на этот тег (из неё — текст релиза), коммит **с актуальным** `release.yml`.
3. Аннотированный тег **на этом коммите**:
   `git tag -a v0.2.0 -m v0.2.0`
4. Push ветки и тега: `git push origin <ветка>` и `git push origin v0.2.0`. Не `git push --tags`: в клоне могут быть чужие теги вроде `1.10.2` без `v`.
5. Дождаться проверок и зелёных job `build` и `release`.
6. Releases → черновик → проверить архивы → **Publish release**.

Повтор без нового тега: YAML берётся с `--ref`, код — с тега. Пока `go` не влит в `main`, на `main` лежит `release.yml` PowerShell-версии: для тега Go-версии — `--ref go`.

```text
gh workflow run Release --repo NONERUN/zapman --ref go -f tag=v0.2.0
```

Всегда `--repo NONERUN/zapman`: `gh` в этой копии может смотреть на Flowseal. Не создавать Release вручную в UI без этого workflow — не будет zip/rar/tar.gz.

Пока черновик — его можно удалить в UI и прогнать job снова. После Publish ассеты **immutable**, повтор job их не трогает.

Auto-Update Check смотрит **git-теги**, не черновики: тег на origin уже виден клиенту. Скачать «latest» он зовёт на опубликованный релиз.
