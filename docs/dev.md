# Разработка

Этот файл объясняет, как устроена Go-версия zapman: откуда берутся флаги `winws` / `winws2`, как описаны стратегии и игры, что делают пакеты в `internal/`, как устроен GUI и как выпустить релиз. В конце каждой большой части есть пошаговые рецепты для частых задач.

Короткие правила для агентов, раскладка папок, четыре проверки перед коммитом и стиль коммитов — в [`AGENTS.md`](../AGENTS.md); здесь они не повторяются, а только объясняются там, где это нужно. Если меняете генератор флагов, правьте этот файл в том же коммите: тогда описание и код не расходятся.

## Как всё связано

Путь от файла стратегии до работающего обхода такой:

1. **Каталог стратегий** (`strategies/`) описывает стратегии и игровые модули в JSON. Один JSON годится для обоих движков.
2. **Генератор** [`internal/spec`](../internal/spec/) (`spec.Build`) читает каталог и собирает argv — строки флагов — для выбранного движка: `winws` (zapret) или `winws2` (zapret2).
3. [`internal/engine`](../internal/engine/) добавляет к argv путь exe из `bin/`, проверяет длину командной строки и умеет запустить движок на переднем плане.
4. [`internal/service`](../internal/service/) ставит ту же командную строку в службу Windows `zapret`.
5. [`internal/app`](../internal/app/) связывает всё это с настройками пользователя (`user/config.json`) и отдаёт GUI ([`cmd/zapman`](../cmd/zapman/), [`internal/gui`](../internal/gui/)).

Строку флагов разбирает и собирает только `internal/spec`. GUI, служба и всё остальное получают готовые строки и не пытаются их править: так у формата одно место, где он определён, и одно место, которое проверяют тесты.

Версия продукта — git-тег `vMAJOR.MINOR.PATCH`; workflow Release вшивает его в exe (см. [«Релиз»](#релиз)). В файлах репозитория номера версии нет. У PowerShell-версии на ветке `main` всё иначе: там версия — `ModuleVersion` в `src/Zapman/Zapman.psd1`, а тег — `v` + это число.

Формат стратегий — `"spec": 2` (см. [«Каталог стратегий»](#каталог-стратегий)); номер формата в коде — константа `Version` в [`internal/spec`](../internal/spec/).

## Движки (`bin/`)

В `bin/` лежат только файлы самих движков: `winws.exe`, `winws2.exe`, общие для них WinDivert и `cygwin1.dll`, а рядом с winws2 — папка `lua/`. Теги релизов zapret / zapret2 и SHA256 каждого файла записаны в [`bin/versions.json`](../bin/versions.json). Fake-пакеты и паттерны `.bin` в `bin/` не лежат: они часть каталога стратегий, `strategies/fakes/` (см. [«Самодостаточный каталог»](#самодостаточный-каталог)).

В `bin/` всегда есть оба exe. Пользователь выбирает движок радиокнопкой в окне «Стратегия…»; выбор хранится в `user/config.json` в ключе `engine`: `winws` (по умолчанию) или `winws2`. Если файла выбранного exe нет, zapman сообщает об ошибке и не подставляет второй exe: у движков разное поведение, и молча запустить «не тот» движок хуже, чем честно отказать.

### Обновить движки

```text
go run ./dev/update-zapret [-zapret-tag vX] [-zapret2-tag vY] [-dry-run] [-force]
```

Программа скачивает официальные zip-архивы релизов zapret и zapret2, сверяет их с `sha256sum.txt` релиза и копирует в `bin/` файлы движка (exe, dll, sys, lua). Затем она пишет теги и SHA256 в `bin/versions.json`. Без тега берётся последний релиз. Если теги в `versions.json` уже такие же, программа ничего не делает; `-force` распаковывает файлы и пишет `versions.json` заново. `-dry-run` только показывает, что будет сделано. Скачанные zip остаются в `.local/releases/` как кеш. Перед заменой файлов программа останавливает службу `zapret` и процессы `winws` / `winws2`, иначе Windows не даст перезаписать занятые файлы.

SHA256 файлов `.lua` считается после замены CRLF на LF; так же их сверяет `engine.CheckBin` (см. [`internal/engine`](#internalengine)).

### winws и winws2: что одинаково, а что нет

Один JSON стратегии даёт два argv: для winws и для winws2. Обещание здесь такое: **поля JSON значат одно и то же** на обоих движках. Одинаковых пакетов в сети мы не обещаем: у zapret2 другие механизмы (range, payload, обработка пустых ACK), и точное поведение может отличаться. Поэтому стратегию проверяют тестами на том движке, который выбран.

Windows 7 Go-версия не поддерживает (нужна Windows 10+ с WebView2), поэтому особого WinDivert для Windows 7 в `bin/` нет: оба exe используют один и тот же `WinDivert.dll` / `WinDivert64.sys`.

### Служба и `--chdir`

Служба `zapret` запускает exe напрямую: её ImagePath — `"<bin>\<exe>" <argv>`. Служебный менеджер Windows (SCM) стартует процесс с рабочей папкой System32, а winws2 ищет свои Lua-файлы от рабочей папки. Поэтому для winws2 командная строка (`engine.Build.CommandLine`) начинается с `--chdir="<bin>"`, если генератор сам `--chdir` не дал (сейчас не даёт). Та же строка идёт и в запуск на переднем плане, и в службу.

Служба не перечитывает настройки: стратегия, движок, Game Filter, active-fake и путь к папке zapman зашиты в её ImagePath. После смены `engine` или Game Filter при установленной службе нужно снова нажать **Установить службу**. Как zapman понимает, что служба устарела, — в [`internal/service`](#internalservice).

## Каталог стратегий

`strategies/` — сток-каталог, который идёт с программой. Позже появятся каталоги пользователя и внешние каталоги с той же раскладкой (пока не сделано).

```text
strategies/
  catalog.json        { "id", "title", "spec": 2, "active": {...}, "files": {...} }
  fakes/*.bin          fake и паттерны (cat:fakes/<файл>)
  lists.json  targets.json  actions.json   кирпичи: общие для всех стратегий каталога
  flowseal/<имя>.json  порты .bat из Flowseal/zapret-discord-youtube
  zapman/<имя>.json    свои стратегии (папки пока нет: она появится с первой своей стратегией)
  games/<id>.json      игровые модули
  schema/              JSON Schema (генерируется)
```

Происхождение стратегии (`Origin()`: `flowseal` или `zapman`) определяется папкой, а не полем JSON. Имя стратегии — имя файла без `.json`. Поэтому одно имя в двух папках — ошибка загрузки (непонятно, какую стратегию выбрал пользователь), и любой другой `*.json` в корне каталога, кроме `catalog.json` и кирпичей, тоже ошибка: стратегия должна лежать в папке своего происхождения.

JSON Schema в `strategies/schema/` генерируется из структур Go (см. [«Что запускать после правки каталога»](#что-запускать-после-правки-каталога)); у каждого файла каталога есть `$schema` с относительным путём к схеме, поэтому редактор подсказывает поля.

### Файлы каталога

| Файл | Что внутри |
|---|---|
| `catalog.json` | `id` (имя из `a-z`, `0-9`, `-`), `title`, `spec` = 2. `active` — `{ "<ИМЯ>": "cat:fakes/<файл>" }`: дефолт для каждого `active:<ИМЯ>` (значение должно быть путём `cat:`). `files` — `{ "<путь через />": "<sha256 hex>" }` для каждого файла каталога, кроме `catalog.json` и `schema/`; генерируется |
| `lists.json` | именованные наборы списков: `hostlist`, `hostlistExclude`, `ipset`, `ipsetExclude` (пути `lists:`, `user:`, `cat:`) |
| `targets.json` | именованные цели: `tcp`, `udp`, `l3`, `l7`, `lists` (имена наборов с `hostlist` / `ipset`), `exclude` (наборы с `*Exclude`), `domains`, `excludeDomains`, `anyProtocol`, `payload` |
| `actions.json` | именованные действия: `desync` и поля атаки из таблиц ниже; `cutoff` — число N (то же, что `nN`) или `"none"` |
| `flowseal/<имя>.json`, `zapman/<имя>.json` | стратегия: `"spec": 2`, версия (`upstream` + `rev` или `version`, см. [«Версии»](#версии-стратегий-и-игр)), `about`, `rules[]` — пары `{ "target", "action" }` |
| `games/<id>.json` | игровой модуль: `id` (= имя файла), `title`, `about`, `version`, `order`, `updater`, `options`, свои `lists` (у набора может быть `when`), `rules[]` |

`about` игры — объект `{ "ru": "…", "en": "…" }`: текст под названием игры в окне «Игры». В argv он не попадает, поэтому его правка не меняет `version`.

Правила стратегии идут в порядке профилей winws: на пакет срабатывает первое подходящее правило.

### Цели, действия и кирпичи

`target` и `action` в правиле — либо имя из каталога (`targets.json`, `actions.json`), либо объект прямо в правиле. Кирпичи нужны, чтобы не копировать одно и то же по всем стратегиям, но слишком мелкие кирпичи только мешают читать стратегию. Поэтому в каталоге принято так:

- цель выносится в `targets.json`, если её используют 2 и больше раз;
- действие выносится в `actions.json` (пресет), если его используют 3 и больше раз; остальные действия пишутся прямо в правиле. Если после правки у пресета остался один-два пользователя, действие возвращается в правила.

Загрузчик этих порогов не проверяет — это соглашение каталога.

Имена кирпичей — из `a-z`, `0-9`, `-`. Пример правила с именованной целью и встроенным действием:

```json
{ "target": "quic-general", "action": { "desync": "fake", "repeats": 6, "fake": { "quic": ["cat:fakes/quic_initial_www_google_com.bin"] } } }
```

### Строгая загрузка

Загрузчик каталога строгий: ошибка в JSON должна всплыть при загрузке, а не превратиться в странный argv. Ошибкой считаются:

- неизвестное поле, неизвестное имя цели, действия или набора, неверный порт;
- путь `cat:`, которого нет в каталоге (и `bin:`, если загрузчику дана папка `bin`);
- `active:<ИМЯ>` без дефолта в `active` из `catalog.json`;
- набор списков игры с тем же именем, что в `lists.json`;
- у игры больше 4 опций, `when` набора с неизвестной опцией, опция без набора или опция, которая не меняет правила (проверяются все 2ⁿ сочетаний значений).

Набор игры обычно берёт два списка: стандартный `lists:` (содержимое релиза) и свой `user:` (только добавления пользователя), например `["lists:ipset-sky.txt", "user:ipset-sky.txt"]`. Начальной копии нет: новый `user:`-список игры получает комментарий и неиспользуемый адрес `203.0.113.113/32`.

### Префиксы путей

Все пути в JSON пишутся с префиксом; регистр префикса важен (`lists:`, не `LISTS:`).

| Префикс | Куда |
|---|---|
| `lists:` | `lists/` |
| `user:` | `user/` |
| `cat:` | корень каталога стратегий (у сток-каталога — `strategies/`); путь через `/`, без `..`, например `cat:fakes/stun.bin` |
| `active:<ИМЯ>` | fake, который пользователь может подменить: `user\fakes\ACTIVE_<ИМЯ>.bin`, если сборке сказано, что этот файл есть (`Paths.ActiveFiles`; сам сборщик диск не читает), иначе дефолт из `active` в `catalog.json`. Имя — из `A-Z`, `0-9`, `_`. В argv winws2 имя blob — `ACTIVE_<ИМЯ>` |
| `bin:` | `bin/`, только файлы движка. В сток-каталоге не используется: это проверяет `TestCatalogSelfContained` |

### Самодостаточный каталог

Всё, на что ссылаются JSON каталога (кроме списков `lists:` и `user:`), лежит в самом каталоге. Благодаря этому прошлый тег `strategies/` можно подключить как внешний каталог, и он будет работать.

`files` в `catalog.json` — SHA256 байтов каждого файла каталога в том виде, в каком они лежат в git. Чтобы checkout на любой машине не менял эти байты, в `.gitattributes` для `strategies/**/*.json` задано `eol=lf`, а для `*.bin` — `binary`.

### Что запускать после правки каталога

После любой правки файла каталога (JSON, `.bin`) или типов в `internal/spec`:

```text
go generate ./internal/spec
```

Он запускает [`dev/schemagen`](../dev/schemagen/) (пишет `strategies/schema/`) и [`dev/catalogfiles`](../dev/catalogfiles/) (пишет `files` в `catalog.json`). Если забыть, `TestCatalogFiles` упадёт с подсказкой `run go generate ./internal/spec`, а `TestSchemaFiles` — если схема не совпадает с типами Go.

## Как JSON становится argv

Каждое правило стратегии `spec.Build` превращает в профиль старого формата v0.1 (`V01Spec` — внутренняя промежуточная модель), а профиль — в флаги тем же генератором, что был до формата v2. Поэтому argv при переходе на v2 не изменился.

Несколько частей argv выводятся автоматически, и руками их не задают:

- **`wf`** — объединение портов всех целей стратегии и включённых игр, без повторов, по возрастанию первого порта. Последним идёт `$gameFilter` — порты Game Filter: `12`, если фильтр для этого протокола выключен, иначе `gameFilterTcp` / `gameFilterUdp` из `user/config.json`. winws получает `--wf-tcp=` / `--wf-udp=`, winws2 — `--wf-tcp-out=` / `--wf-udp-out=`.
- **`payload` для winws2** выводится из `l7` и `anyProtocol`: например, `tls` + `http` дают две группы, `discord` — `discord_ip_discovery,stun`. Явный `payload` пишут, только когда вывод не подходит.
- **Игры** идут **до** правил стратегии: их цели уже, чем общие правила на тех же портах, и иначе общее правило перехватило бы игровой трафик. Между собой игры идут по `order`, затем по `id`. Сейчас: `sky` (10), `photon` (30).

Lua для winws2 — только стоковые `bin/lua/zapret-lib.lua` и `bin/lua/zapret-antidpi.lua` из релиза zapret2. Своих `.lua` генератор не подключает, а Lua-атаки не переводятся в флаги winws: обе стороны генератора описывают одни и те же атаки стоковыми средствами каждого движка.

### Проверка JSON при сборке

Проверка одна для обоих движков: стратегия, которая собирается для winws, соберётся и для winws2, и наоборот. Имена `desync`, `fooling`, `l7`, `cutoff` и префиксы путей сравниваются с учётом регистра (`fake`, не `FAKE`). `engine` — ровно `winws` или `winws2`.

Сборка возвращает ошибку, если:

- в стратегии нет профилей (`profiles`) или их список пуст;
- в `l7` имя не из `http`, `tls`, `quic`, `wireguard`, `dht`, `discord`, `stun`, `unknown`, или в одном `l7` вместе `quic` и `tls`;
- `cutoff` не `nN` и не `none`;
- путь без префикса или без имени файла после префикса;
- в `hostlistDomains` / `hostlistExcludeDomains` есть пустой элемент;
- значение fake — не `!`, не `0x<hex>` и не путь с префиксом, или в нём есть пробел;
- `desync` или `fooling` — имя не из таблиц ниже.

Список атак в генераторе закрыт: имя `desync` вне таблицы — ошибка, а не «передать как есть». Новая атака добавляется в генератор для обоих движков сразу, вместе с тестами и строкой в этом файле.

### Цель и действие (поля профиля)

`desync` — одно из: `fake`, `multisplit`, `multidisorder`, `hostfakesplit`, `fakedsplit`, `syndata`.

| Поле | winws | winws2 |
|---|---|---|
| `tcp` / `udp` / `l3` / `l7` | `--filter-tcp=` / `--filter-udp=` / `--filter-l3=` / `--filter-l7=` | то же |
| `payload` | не пишется (фильтр из `l7`) | `--payload=` |
| `luaPayload` | не пишется | `payload=` на каждой Lua-атаке. Для неизвестных игровых пакетов нужен `all`: одного `--payload=all` недостаточно. У `fake` заменяет фильтры слотов (см. [Fake](#fake)) |
| `hostlist` | `--hostlist=` | то же |
| `hostlistExclude` | `--hostlist-exclude=` | то же |
| `hostlistDomains` | `--hostlist-domains=` | то же |
| `hostlistExcludeDomains` | `--hostlist-exclude-domains=` | то же |
| `ipset` | `--ipset=` | то же |
| `ipsetExclude` | `--ipset-exclude=` | то же |
| `desync` | `--dpi-desync=` | `--lua-desync=<атака>:…` на каждую атаку |
| `repeats` | `--dpi-desync-repeats=` | `repeats=` у Lua-атак (кроме `multisplit` / `multidisorder`) |
| `anyProtocol` | `--dpi-desync-any-protocol=1` | `--payload=all`, если нет явного `payload` |
| `cutoff` | `nN` → `--dpi-desync-cutoff=nN` | `nN` → `--out-range=-nN`; без `cutoff` при TCP/QUIC → `--out-range=-d10` |
| `cutoff: "none"` | cutoff не пишется | `--out-range` не пишется: обработка продолжается после TLS-handshake |
| `ttl` | `--dpi-desync-ttl=` | `ip_ttl=` на атаках, которые принимают fooling; число 1–255 |
| `splitPos` | `--dpi-desync-split-pos=` | `pos=` у split-атак |
| `seqovl` | `--dpi-desync-split-seqovl=` | `seqovl=` |
| `seqovlPattern` | `--dpi-desync-split-seqovl-pattern=` | blob + `seqovl_pattern=`. На обоих движках — только путь с префиксом (`cat:` / `active:` / `lists:` / `user:` / `bin:`) |
| `ipId` | `--ip-id=` | `ip_id=` |
| `tlsMod` | `--dpi-desync-fake-tls-mod=` после `fake-tls` | `tls_mod=` на последнем ref слота `tls`, только на вызове для `tls_client_hello`. Без слота `tls` не пишется |
| `hostfakesplitMod` | `--dpi-desync-hostfakesplit-mod=` как в JSON | куски через запятую; `altorder=*` не пишется, потому что в стоковом Lua его нет |
| `fakedsplitPattern` | `--dpi-desync-fakedsplit-pattern=` (готовый путь, не сырой `cat:`) | `pattern=` / blob |

#### Почему cutoff на winws2 — `-nN`

`--out-range=-nN` у winws2 считает пакеты, как `--dpi-desync-cutoff=nN` у winws. На UDP счёт совпадает полностью. На TCP winws2 по умолчанию не перехватывает пустые ACK (`--wf-tcp-empty=0`, zapret2 `manual.en.md:1071`, `readme.md:314`), поэтому cutoff наступает на 1–2 пакета с данными позже, чем у winws, — но раньше, чем при прежнем `-dN`. Точное совпадение дал бы глобальный `--wf-tcp-empty=1`, но он примерно вдвое поднимает нагрузку на CPU при загрузках с включённым Game Filter. Поэтому его не пишем и об этой разнице не предупреждаем. Неявный `-d10` для TCP/QUIC без `cutoff` остаётся.

### Fooling

| JSON | winws | winws2 |
|---|---|---|
| `fooling` содержит `ts` | `--dpi-desync-fooling=…ts…` | `tcp_ts=-600000` |
| `fooling` содержит `md5sig` | `--dpi-desync-fooling=…md5sig…` | `tcp_md5` |
| `fooling` содержит `badseq` | `--dpi-desync-fooling=…badseq…`; `--dpi-desync-badseq-increment=` — только если задан `badseqIncrement` | `tcp_seq=<badseqIncrement>` (при 0 не пишется) и `tcp_ack=-66000` |
| `badseq` без `badseqIncrement` | флага нет: работает дефолт winws — seq **−10000**, ack **−66000** (`nfq/params.h`) | `tcp_seq=-10000:tcp_ack=-66000` — те же значения явно |
| другое имя | ошибка сборки | ошибка сборки |

На Lua-атаку `multisplit` fooling не вешается.

`fooling=ts` работает, только если в Windows включены TCP timestamps; zapman включает их сам (см. [`internal/service`](#internalservice)).

### Fake

Слоты fake: `quic`, `tls`, `http`, `discord`, `stun`, `unknown`, `unknownUdp`.

| JSON | winws | winws2 |
|---|---|---|
| `fake.tls` / `http` / … | `--dpi-desync-fake-tls=` (и аналоги) | `--blob=name:@file` или `name:0xHEX`, затем `blob=` в `--lua-desync=fake` |
| значение `!` | как в JSON (встроенный hello zapret) | `cat:fakes/tls_clienthello_www_google_com.bin` — этот файл должен быть в каждом каталоге |
| TLS-профиль с `fake` в `desync`, слота `tls` нет | zapret берёт встроенный hello | ошибка сборки: у группы нет blob. Неявного hello у winws2 нет, пишите `"tls": ["!"]` |
| `payload: "all"` / `anyProtocol`, слота `tls` нет | zapret берёт встроенный hello для TLS | TLS hello не подставляется |
| группа из нескольких слотов (`all`, `discord,stun`), нет `luaPayload` | fake выбирается по типу пакета: TCP — `http` / `tls` / `unknown`, UDP — `quic` / `discord` / `stun` / `unknownUdp` | у каждого слота свой `payload=` у `fake`: `tls` → `tls_client_hello`, `http` → `http_req`, `quic` → `quic_initial`, `discord` → `discord_ip_discovery`, `stun` → `stun`, `unknown` → `~empty,http_req,tls_client_hello`, `unknownUdp` → `~empty,quic_initial,wireguard_initiation,dht,discord_ip_discovery,stun`. Один ref в нескольких слотах — один вызов с фильтрами через запятую. Слоты другого L4 (UDP-слот в TCP-профиле) не пишутся. `unknown` / `unknownUdp` в профиле одновременно с `tcp` и `udp` — ошибка сборки |
| hex `0x00000000` | как есть | имя blob `hex_00000000` (имя blob не может начинаться с `0x`) |
| порядок | `fake-tls` **до** `fake-tls-mod` | `tls_mod` — только на последнем ref слота `tls` в группе `tls_client_hello` или с фильтром `tls_client_hello`; этот вызов не сливается с другими слотами. На `!` из `unknown` и других слотов `tls_mod` нет |

Итог для обоих движков: в argv winws2 нет `--dpi-desync`, в argv winws нет `--lua-desync`, а `--blob=` всегда в виде `name:value` (через двоеточие). Эти правила проверяет `TestGolden` (см. [«Тесты»](#тесты-генератора)).

## Игровые модули

Игровые модули лежат в `strategies/games/` и включаются в окне «Игры…» поверх любой стратегии. От Game Filter они не зависят: их порты всегда попадают в `wf`.

- **`sky`** — Sky: Children of the Light. UDP 5000–8000 по спискам `lists:ipset-sky.txt` и `user:ipset-sky.txt` с QUIC-fake, TTL 8 и cutoff 4. TCP 443 только для `live.ws.radiance.thatgamecompany.com`: multisplit на позиции 1 с `luaPayload: all` и без cutoff, потому что WebSocket-трафик идёт и после handshake. Трей (`updater: sky-log`) дополняет `user:ipset-sky.txt` только конкретными IPv4-адресами `/32` из `Sky.log`. DNS входа в игру — отдельная настройка: стратегия файл `hosts` не меняет.
- **`photon`** — игры на Photon, UDP 5055–5058 и 27000–27002: fake `active:GAME_UDP`, `unknownUdp`, `anyProtocol`, `luaPayload: all`, 6 повторов, TTL 8, cutoff 4. TTL здесь обязателен: fake без TTL доходил до сервера Photon, и игра теряла соединение (`ClientTimeout`) примерно через 10 с после входа в матч. Опция `ipset` (по умолчанию выключена) включает набор `ipset-photon` (`when: "ipset"`): `lists:ipset-photon.txt` (подсети Photon Cloud EU) и `user:ipset-photon.txt`. Выключена — правило для всех IP: так работал прежний общий профиль, и так покрыты регионы вне EU.

Раньше была отдельная стратегия `general (ALT11 Photon)`; теперь это `general (ALT11)` + `sky`, `photon`. Отличаются они только порядком профилей и портов `wf`.

### Опции игр

Опции игры описаны в `options` её файла (не больше 4):

```json
"options": {
  "ipset": {
    "title": { "ru": "Только подсети Photon Cloud EU из списка", "en": "Only the Photon Cloud EU subnets from the list" },
    "about": { "ru": "…", "en": "…" },
    "default": false
  }
},
"lists": { "ipset-photon": { "ipset": ["lists:ipset-photon.txt", "user:ipset-photon.txt"], "when": "ipset" } }
```

Набор с `when` работает, только когда опция включена; при выключенной опции он пустой, и цель с этим набором идёт без `--ipset` (все IP). Загрузчик заранее собирает правила для всех сочетаний опций (`Game.With` выбирает готовый вариант) и требует, чтобы каждая опция меняла правила.

Пользователь меняет опции в окне «Игры…». В `user/config.json` попадают только значения, отличные от `default`:

```json
{ "games": ["photon", "sky"], "gameOptions": { "photon": { "ipset": true } } }
```

`gameOptions` может хранить опции выключенной игры. Неизвестная игра или опция в `gameOptions` — ошибка поля `gameOptions` при сборке (`engine.Games`). Значения опций входят в версию игры для службы (`photon 3 ipset=on`, см. [`zapmanGames`](#запись-о-стратегии-и-служба-устарела)), поэтому смена опции делает службу устаревшей. `EnsureUserFiles` создаёт списки всех наборов включённой игры, в том числе набора с выключенной опцией: пользователь может заполнить список до включения опции.

Свойства игровых профилей, проверенные в самих играх (TTL, cutoff, порядок профилей, один fake на профиль и т. д.), закреплены в `TestGames` (`internal/spec`) на `general (ALT11)`. Golden-файлы можно перезаписать через `-update`, а `TestGames` — нет, поэтому эти свойства не потеряются при случайном обновлении golden.

## Версии стратегий и игр

Версия отвечает на вопрос «чей это argv и с какой правки». В Go: `Strategy.Version()` / `Game.Version()` — строка для людей (`Flowseal 1.10.4 r2`, `Zapman 3`), `Origin()` — `flowseal` или `zapman` (у игр всегда `zapman`). Версия видна в GUI и записывается в ключ службы, чтобы zapman понимал, что служба устарела.

| Где | Поля | Смысл |
|---|---|---|
| `flowseal/` | `upstream` и `rev` (оба обязательны; `version` здесь запрещён) | `upstream` — первый по дате тег Flowseal, в котором есть последний коммит upstream, менявший `<имя>.bat`. `rev` — 1 + число наших изменений argv после этого тега; с новым тегом `upstream` `rev` снова 1 |
| `zapman/` | `version` (обязателен; `upstream` / `rev` запрещены) | целое ≥ 1, +1 при каждом изменении argv |
| `games/` | `version` (обязателен) | целое ≥ 1, +1 при каждом изменении argv модуля |

Когда поднимать: если поменялся argv стратегии или игры — из-за правки её JSON, кирпича, который она использует, или генератора, — `rev` / `version` растёт на 1. Если argv не поменялся (переименовали пресет, вынесли действие в кирпич, поправили `about`), версию не трогают. Правка генератора поднимает версию у каждой стратегии, чей argv она затронула.

Смена имени стратегии и удаление стратегии — тоже изменения для пользователя; как их записать в CHANGELOG, сказано в [`AGENTS.md`](../AGENTS.md).

### Тесты генератора

Golden-файлы — эталонный argv каждой стратегии: `testdata/golden/<движок>/<gf-off|gf-all>/<имя>.txt`. `gf-off` — Game Filter выключен (порты `12`), `gf-all` — `1024-65535` для TCP и UDP. Стратегии собираются без игр; `general (ALT11)` дополнительно собирается со всеми играми под именем `general (ALT11)+games` (опции по умолчанию) и `general (ALT11)+games+options` (все опции включены). Пути в golden — фиксированные (`C:\zapman\…`), active-fake — дефолты каталога. Переменная `ZAPMAN_SPEC_ROOT` позволяет прогнать тест на другом дереве с `strategies/`, `bin/` и `testdata/golden/`.

Первая строка golden-файла — версия: `# version: flowseal 1.10.4 r2` или `# version: zapman 3`; у `general (ALT11)+games` после стратегии перечислены игры в порядке сборки со значениями опций (`; sky 3; photon 3 ipset=off`). Эта строка — страж: если argv изменился, а версия та же, `TestGolden` падает с `argv changed: bump rev/version`. Любое другое расхождение (или golden без строки версии) — тоже ошибка с подсказкой.

Как менять argv стратегии:

1. Поправить JSON (или генератор).
2. Поднять `rev` / `version` у затронутых стратегий и игр.
3. `go generate ./internal/spec`, если менялся каталог.
4. Перезаписать golden и посмотреть diff:

   ```text
   go test ./internal/spec -run "Golden|Edge" -update
   git diff testdata/
   ```

5. Убедиться, что diff — ровно то изменение, которое задумано.

`-update` перезаписывает только golden-файлы и `internal/spec/testdata/edge.*.txt` (редкие случаи из `testdata/edge.json`, `TestEdge`). Проверки, которые от golden не зависят, продолжают работать и с `-update`:

- `TestGolden` на каждом argv проверяет правила движков: в winws нет `--lua-desync`, а `fake-tls` идёт до `fake-tls-mod`; в winws2 нет `--dpi-desync`, `--blob=` только `name:value`, имя blob не начинается с `0x`, нет `blob=empty`, у `fake` с `tls_mod` есть `blob=`; ни у одного движка в argv не остаётся неразрешённого префикса (`bin:`, `cat:`, …);
- `TestGames` — свойства игровых профилей (см. [«Игровые модули»](#игровые-модули));
- `TestCatalogFiles`, `TestCatalogSelfContained`, `TestSchemaFiles` — каталог и схема.

## Синхронизация с Flowseal

Стратегии в `flowseal/` — переносы `general*.bat` из [Flowseal/zapret-discord-youtube](https://github.com/Flowseal/zapret-discord-youtube). Для работы с ними нужен remote `upstream` с этим репозиторием и `git fetch upstream`. Обе программы читают `.bat` из git (`git show`), без checkout.

### Какие стратегии отстали: `dev/upstream-versions`

```text
go run ./dev/upstream-versions [-git .] [-ref upstream/main] [-dir strategies] [-write]
```

Для каждого `flowseal/<имя>.json` программа находит последний коммит `-ref`, менявший `<имя>.bat`, и первый по дате тег upstream, в котором он есть. Учитываются только теги из `-ref` (`--merged`); наши теги `v*` не считаются. Она печатает таблицу: имя, тег в файле, вычисленный тег и статус — `ok`, `outdated`, `no .bat upstream` или `no tag yet`. Если есть статус не `ok`, код выхода 2.

`-write` ставит вычисленный `upstream` и `rev: 1` только там, где тег сменился. Сами изменения `.bat` после этого нужно перенести в JSON (следующая программа) и обновить golden.

### Перенос `.bat` в JSON: `dev/import-flowseal`

```text
go run ./dev/import-flowseal [-git .] [-ref upstream/main] [-dir strategies] [-write]
```

Программа читает `general*.bat` из `-ref`: склеивает строки с `^`, раскрывает `set "X=…"`, заменяет `%GameFilterTCP%` / `%GameFilterUDP%` на `$gameFilter` и режет профили по `--new`. Каждый профиль она переводит обратно в правило — по сути это таблица генератора winws, прочитанная справа налево:

- списки уходят в наборы `lists.json`;
- цель и действие получают имя из `targets.json` / `actions.json`, если там есть такой же JSON, иначе пишутся прямо в правило;
- fake `bin\X` становится `cat:fakes/X`, `bin\ACTIVE_X.bin` — `active:X`.

Неизвестный флаг или значение — ошибка с именем `.bat` и флага. Затем идёт проверка «туда и обратно»: argv нового файла (`spec.Build`, winws) должен совпасть с argv `.bat`. Порты `wf` сравниваются как множества, порядок флагов внутри профиля свободный, а порядок значений одного флага и пары `fake-tls` / `fake-tls-mod` важен.

Статусы:

| Статус | Значение |
|---|---|
| `same` | argv текущего `flowseal/<имя>.json` совпадает с `.bat` |
| `changed` | argv другой; ниже печатаются строки различий «ours / Flowseal» |
| `new` | `.bat` есть, файла стратегии нет |
| `removed` | файл стратегии есть, `.bat` нет; файл не удаляется |
| `error` | `.bat` не удалось перевести |

Код выхода: 1 при ошибке; 2, если есть статус не `same` и файл не записан, или есть `removed`; иначе 0. `-write` пишет `flowseal/<имя>.json` для `changed` и `new`: `upstream` — тег `.bat`, `rev: 1`, `about` сохраняется. Файлы со статусом `same` не трогаются (остаются байт в байт). После записи — `go generate ./internal/spec` и golden.

На теге `1.10.4` (и на ветке `flowseal/processed`) все 22 файла дают `same`; тег `1.10.4` проверяет `TestImport` (он пропускается, если нет git или этого тега).

#### Две добавки для winws2

В `.bat` нет двух вещей, которые нужны winws2, и импорт выводит их сам ([`derive.go`](../dev/import-flowseal/derive.go)) и пишет в правила. При сравнении они добавляются и к argv `.bat`, поэтому `same` остаётся `same`; любое другое различие видно как обычно. На argv winws эти добавки не влияют.

`--filter-l7` выводится, только если в профиле `.bat` нет ни `--filter-l7`, ни `--dpi-desync-any-protocol`. Должна подойти ровно одна строка таблицы, иначе — ошибка с именем `.bat` и номером профиля:

| Фильтр | Порты | `fake.http` | `--filter-l7` |
|---|---|---|---|
| только `--filter-udp` | `443` | — | `quic` |
| только `--filter-tcp` | из `80, 443, 2053, 2083, 2087, 2096, 8443` | нет | `tls` |
| только `--filter-tcp` | те же, есть `80` | есть | `tls,http` |

`--dpi-desync-fake-tls=!` (встроенный hello) добавляется, если в профиле есть `--dpi-desync-fake-tls-mod`, но нет `--dpi-desync-fake-tls` и нет `--dpi-desync-any-protocol`: winws берёт этот hello неявно, а у winws2 неявного TLS fake нет. Профили Game Filter с `any-protocol` hello не получают — иначе он ушёл бы на все payload.

### Одноразовый конвертер v0.1 → v2

[`dev/convert-v2`](../dev/convert-v2/) один раз перевёл файлы формата v0.1 в v2 и сверил argv до записи. Он берёт `upstream` из git (`-git`, `-ref`, по умолчанию `upstream/main`), пишет стратегиям `rev: 1`, играм — `version: 1`, а fake — как `bin:`. Его тест сравнивает вывод с `strategies/` коммита `9f9aeed` (до переноса fake в `strategies/fakes/`) при `rev` = 1 и пропускается без `upstream/main`. После миграции программу можно удалить.

## Частые задачи с каталогом

**Поправить существующую стратегию.** Измените правило или кирпич, поднимите `rev` / `version` у всех стратегий, чей argv изменился, затем `go generate ./internal/spec` и обновление golden (см. [«Тесты генератора»](#тесты-генератора)). Если правите кирпич, проверьте в diff golden, какие ещё стратегии он задел.

**Добавить свою стратегию.** Свои стратегии появляются только после явного решения в чате или issue, а смысл атак берётся из Flowseal (см. [`AGENTS.md`](../AGENTS.md)). Файл кладётся в `strategies/zapman/<имя>.json` (папку создайте с первой стратегией) с `"spec": 2`, `"version": 1`, `about` и `rules`. Дальше — `go generate ./internal/spec` и `-update` golden: появятся новые golden-файлы.

**Добавить игру.** Создайте `strategies/games/<id>.json` с `id`, `title`, `about` (`ru` и `en`), `version: 1`, `order` (определяет место среди других игр), правилами и, если нужен свой список, набором `lists` из стандартного `lists:ipset-<id>.txt` и своего `user:ipset-<id>.txt`. Положите стандартный список в `lists/`. Если правило нужно в двух вариантах (например, все IP или только список), добавьте опцию в `options` и `when` у набора (см. [«Опции игр»](#опции-игр)). Проверенные в игре свойства закрепите в `TestGames`. Затем `go generate ./internal/spec` и golden (`general (ALT11)+games` и `+options`).

**Обновить стратегии Flowseal.** `git fetch upstream` → `dev/upstream-versions` (что отстало) → `dev/import-flowseal -write` → `go generate ./internal/spec` → golden.

## Пакеты среды выполнения

Это библиотеки, на которых стоит GUI (и будущий CLI). Своего `main` у них нет.

### `internal/config`

Работает с `user/config.json`.

- `Load`: нет файла — `Default()`; файл не JSON — дефолты и ошибка; поле с плохим значением остаётся дефолтным, а ошибка (`FieldError`) называет это поле. Так один испорченный ключ не сбрасывает остальные настройки.
- `Save` сначала проверяет поля (`Validate`), затем пишет атомарно: временный файл в `user/`, потом rename. При сбое старый файл остаётся целым.
- Неизвестные поля сохраняются и пишутся после известных, чтобы новая версия программы не теряла настройки другой.

| Поле | Значения | Дефолт |
|---|---|---|
| `language` | `ru`, `en`, `""` (язык Windows) | `""` |
| `engine` | `winws`, `winws2` | `winws` |
| `strategy` | имя стратегии каталога; `""` — не выбрана | `""` |
| `games` | ID включённых игровых модулей (`sky`, `photon`) | `[]` |
| `gameOptions` | опции игр, отличные от `default`: `{ "<игра>": { "<опция>": bool } }`; проверяется только синтаксис имён, наличие игры и опции в каталоге проверяет `engine.Games` (см. [«Опции игр»](#опции-игр)) | нет поля |
| `gameFilter` | `disabled`, `all`, `tcp`, `udp` | `disabled` |
| `gameFilterTcp`, `gameFilterUdp` | порты Game Filter: `N` или `N-M` через запятую, 1–65535, начало ≤ конца (правила Flowseal `service.bat`, коммит `2384961`); пробелы убираются при чтении | `1024-65535` |
| `autoUpdateCheck` | bool | `true` |
| `trayWatch` | bool | `true` |
| `autoUpdateBricks` | bool: свои стратегии берут новые сток-кирпичи после обновления | `true` |

Файл PowerShell-версии (`language`, `engine`, `gameFilter`, `autoUpdateCheck`, `trayWatch`) читается как есть. Единственная разница: PowerShell принимал строки `"yes"` / `"1"` вместо bool, а Go их не принимает (ошибка поля). Сам PowerShell всегда писал настоящий bool, так что на практике это не мешает.

### `internal/engine`

- `NewLayout(root)` — папки `bin/`, `strategies/`, `lists/`, `user/`. `Paths()` даёт `spec.Paths`; в `ActiveFiles` попадают имена `X` файлов `user/fakes/ACTIVE_<X>.bin` (префикс `ACTIVE_` и `.bin` — без учёта регистра).
- `Exe(engine)` — путь exe выбранного движка; нет файла — ошибка (второй exe не берётся, см. [«Движки»](#движки-bin)).
- `EnsureUserFiles(cat, games)` создаёт недостающие `user:`-списки наборов каталога и включённых игр (всех наборов игры, в том числе с выключенной опцией); существующие не трогает. Содержимое нового списка каталога: `lists/<имя>.default.txt` (так появляется `ipset-all.txt`); иначе одна заведомо неиспользуемая запись — `domain.example.abc` или `203.0.113.113/32`. Новый ipset-список игры получает комментарий и `203.0.113.113/32`: стандартный список игры — отдельный `lists:`-файл. `UserFile(name)` даёт путь `user:`-файла и проверяет, что он внутри `user/`. Пустым список делать нельзя: пустой список совпадает со всеми хостами. Ещё он вызывает `EnsureActiveFakes(cat)`: если нет `user/fakes/ACTIVE_<X>.bin` (поиск как у `Paths`), туда копируется дефолт `X` из `active` в `catalog.json`; существующий файл не трогается. `internal/app` вызывает `EnsureUserFiles` до сборки argv, поэтому уже первый запуск берёт файл пользователя.
- `CheckBin(bin)` сверяет файлы с `bin/versions.json` без сети. Состояние файла: `ok`, `missing` или `hash mismatch`; если `winws.exe` или `winws2.exe` нет в самом `versions.json` — `not in versions.json`. SHA256 `.lua` считается после CRLF → LF, как в `dev/update-zapret`.
- `Build(cat, cfg)` собирает стратегию, игры и Game Filter из конфига через `spec.Build`. Нет стратегии в конфиге — `ErrNoStrategy`; игра не из каталога — ошибка поля `games`; игры и их опции даёт `Games(cat, cfg)`, неизвестная игра или опция в `gameOptions` — ошибка поля `gameOptions`. `CommandLine()` = `"<bin>\<exe>" <строки argv через пробел>`; для winws2 первым идёт `--chdir="<bin>"` (см. [«Служба и `--chdir`»](#служба-и---chdir)). Та же строка становится ImagePath службы. Если она длиннее 32766 символов UTF-16 (предел `CreateProcess`), `Build` возвращает ошибку с советом выключить часть игр или взять путь к папке короче.
- `Runner.Run(build)` запускает движок на переднем плане: рабочая папка `bin/`, без окна, сохраняется хвост вывода 64 КБ. Если процесс вышел в первые 2 с, это ошибка (`ExitError`). Код `0xC0000142` (`STATUS_DLL_INIT_FAILED`) при первом запуске даёт паузу 1 с и один повтор; если он повторился — ошибка с кодом и советом про исключения антивируса. Сам запуск процесса (`Runner.Start`) в тестах подменяется. Что служба сейчас не запущена, проверяет вызывающий.
- Режим IPSet (`IPSetMode`) — не настройка, а состояние `user/ipset-all.txt`: `none` (файла нет или в нём только неиспользуемый адрес), `loaded` (список адресов), `any` (пустой файл — подходит любой адрес). `SetIPSetMode` переключает режим, а загруженный список хранит в копии (`ipset-all.txt.backup`), как PowerShell-версия.

### `internal/service`

Служба `zapret` — то же имя, что у PowerShell-версии и у Flowseal `service.bat`. Она запускает `winws.exe` / `winws2.exe` напрямую: ImagePath — `Build.CommandLine()` как есть, без второго экранирования. Для этого `CreateService` вызывается напрямую, а не через `mgr.CreateService`, который экранировал бы строку ещё раз. Все вызовы SCM, реестра и процессов спрятаны за интерфейсом `Host`, поэтому тесты работают с подделкой и не трогают службы машины.

- `PlanFor(build)` → `Plan{Strategy, Version, Games, ImagePath}`.
- `Install(plan)` повторяет шаги PowerShell-версии: копия старой записи службы → стоп службы и всех `winws` / `winws2` → удаление и ожидание, пока служба пропадёт (8 с) → создание (8 попыток через 400 мс, автозапуск, описание `Zapret DPI bypass software`) → запись значений и сверка чтением → старт и ожидание `Running` (15 с), затем ещё 2 с. Если после удаления что-то пошло не так, новая служба удаляется, а старая создаётся снова и запускается, если работала до установки (остановленная остаётся остановленной). Код выхода `0xC0000142` даёт один повтор старта. ImagePath длиннее 32766 символов, ImagePath без кавычек вокруг exe или план без стратегии отклоняются до любых вызовов.
- `Remove()` — стоп службы, стоп `winws` / `winws2`, удаление `zapret`, затем стоп и удаление `WinDivert` и `WinDivert14`. Порядок важен: иначе драйвер остаётся в `STOP_PENDING`. Задачу трея `Remove` не трогает.
- `Start()` — если служба уже запущена, ничего не делает; ImagePath не меняет. `Stop()` останавливает и ждёт `Stopped`.
- `Snapshot()` — точная копия службы: ImagePath, имя, описание, тип запуска (с отложенным автозапуском), все значения записи (включая `zapret-discord-youtube`) и то, работала ли служба. `Restore(snapshot)` создаёт службу снова такой же и запускает, если она работала. Это работает и для чужой службы (например, от `service.bat`). Этими функциями пользуются тесты стратегий; `Install` берёт ту же копию для отката.
- `Install`, `Remove`, `Start`, `Stop` и `Restore` без прав администратора сразу возвращают `ErrNotElevated`, ничего не вызывая. `Status` и `Snapshot` прав не требуют.

#### TCP timestamps

Без TCP timestamps (RFC 1323) не работает `fooling=ts`, а он есть во многих сток-стратегиях. В Windows timestamps по умолчанию выключены.

- `TCPTimestamps()` только читает состояние (для диагностики) и не требует прав администратора.
- `EnableTCPTimestamps()` включает их (`netsh interface tcp set global timestamps=enabled`), если они выключены или состояние неизвестно. Его вызывают `Install` (до любых изменений; при ошибке служба не трогается) и `Start`; перед запуском на переднем плане его вызывает вызывающий код.

Документированного API или значения реестра для текущего состояния нет, поэтому читается вывод `netsh interface tcp show global` (`ParseTCPTimestamps`): ищется строка с `timestamps` или `RFC 1323` (так покрыт английский и русский вывод), берётся значение после последнего `:` — `enabled` или `disabled`, иначе ошибка. Вывод может быть в OEM-кодировке, поэтому смотрятся только ASCII-части.

#### Запись о стратегии и «служба устарела»

Служба не меняется до переустановки: Game Filter, движок, стратегия, active-fake и папка zapman зашиты в ImagePath. Чтобы GUI мог сказать, что служба отстала от настроек, при установке в ключ службы `HKLM\SYSTEM\CurrentControlSet\Services\zapret` пишутся значения:

| Значение | Тип | Что |
|---|---|---|
| `zapman` | REG_SZ | имя стратегии (его же пишет PowerShell-версия) |
| `zapmanVersion` | REG_SZ | `Strategy.Version()`, например `Flowseal 1.10.4 r3` |
| `zapmanGames` | REG_MULTI_SZ | включённые игры с версиями и значениями опций, например `sky 3`, `photon 3 ipset=off` |
| `zapmanHash` | REG_SZ | SHA256 (hex) ImagePath |

Значения удаляются вместе со службой, читаются без прав администратора и не видны в описании службы. Старое значение `zapret-discord-youtube` при установке удаляется; его читает только `Snapshot`, чтобы вернуть службу как была.

`Status(want)` возвращает `Installed`, `State`, `ExitCode`, `Engine` (из ImagePath; `winws2.exe` проверяется первым, потому что `winws.exe` — его подстрока), запись о стратегии, `Edited` (хеш не совпадает с ImagePath), `Outdated` и `Reason`. `Reason` — первая подходящая причина сверху вниз:

| `Reason` | Когда |
|---|---|
| `foreign` | нет значения `zapman`: службу создала другая программа (например, `service.bat`) |
| `strategy` | выбрана другая стратегия |
| — | ImagePath совпадает с планом: служба не устарела |
| `version` | другая версия стратегии или игр |
| `legacy` | нет хеша: службу ставила PowerShell-версия, ImagePath другой |
| `edited` | ImagePath поменяли после установки |
| `argv` | версии те же, но командная строка другая (движок, Game Filter, active-fake, папка) |

### `internal/tools/...`

Инструменты GUI: сеть, hosts, загрузки, диагностика, тесты стратегий. Типы `internal/gui/backend` они не импортируют (получился бы цикл импорта), поэтому у них свои типы с теми же полями. Программы, сеть, ping и система спрятаны за полями структур и `command.Func`, так что тесты обходятся без сети, прав администратора и процессов. Отмена `ctx` возвращает ошибку, для которой `errors.Is(err, context.Canceled)`.

- `command` — `Run(ctx, exe, args...)`: запуск без окна, вывод (stdout + stderr) переводится из OEM-кодировки в UTF-8, код выхода — в `Result.Code`; ненулевой код не считается ошибкой. `System32(name)` — путь в `%SystemRoot%\System32`.
- `netreset` — `Reset`: `netsh winsock reset`, `netsh int ip reset all`, `netsh winhttp reset proxy`, `ipconfig /flushdns` с логом как у PowerShell-версии; если шаг не удался, остальные всё равно выполняются, а итог — `ErrFailed`. `Reboot` — `shutdown /r /f /t 0`.
- `web` — `Client`: `LatestVersion` (теги GitHub, лимит 8 с), `DownloadIPSet` (в `user/ipset-all.txt`, атомарно, с прогрессом), `CheckHosts` (первая и последняя строка шаблона Flowseal в системном hosts), `OpenHosts` (Notepad).
- `diag` — `Checker.Report`: те же 18 проверок, что в PowerShell-версии (ID, тексты, порядок); `Checker.Do` — действия `remove-services`, `remove-windivert`, `discord-cache`. Система — интерфейс `System` (`NewSystem()` — настоящая).
- `stratest` — тесты стратегий без службы (аналог `Invoke-ZapmanStrategyTestsCore`). `New(layout, catalog, config)` — настоящий `Tester`; запуск — `Run(ctx, Request, Sink)`. До тестов проверяются права администратора, `curl.exe` (сначала PATH, затем System32) и то, что служба `zapret` не установлена. Для каждой стратегии: стоп всех `winws` / `winws2`, TCP timestamps, `Layout.Build` + `Runner.Run` (готовность — окно старта 2 с), тесты, стоп. Стандартные тесты — 17 целей: curl HTTP / TLS1.2 / TLS1.3 и ping (ICMP, IPv4). DPI-тесты — набор TCP 16-20 из [hyperion-cs/dpi-checkers](https://github.com/hyperion-cs/dpi-checkers) (`MONITOR_HOST` — одна своя цель без загрузки набора). Лучшая стратегия — та, у которой больше `OK`, при равенстве — больше удачных ping (как в PowerShell-версии; лучшей может оказаться и стратегия с 0 `OK`). Результат пишется в `test-results/test_results_<дата>.txt` (UTF-8 с BOM, CRLF). Режим IPSet и возврат службы и движка, что работали до тестов, — забота вызывающего (`internal/app`, `RunTests`).

Проверять эти инструменты вживую на Windows удобно маленькими программами в `.local/` (папка в `.gitignore`, в репозитории их нет): без прав администратора — версия, hosts, отчёт диагностики, ping; из консоли администратора — тесты стратегий (служба `zapret` должна быть удалена). Проще всего — запустить сам `zapman.exe` (см. ниже).

## GUI (Wails)

GUI написан на [Wails v3](https://v3.wails.io/) `v3.1.0` (версия закреплена в `go.mod`). Ни CLI `wails3`, ни npm не нужны: страницы — обычные HTML, CSS и JS, они вшиваются в exe, и хватает `go build`.

```text
go build -ldflags "-H windowsgui" -o zapman.exe ./cmd/zapman   # exe в корень репозитория: рядом bin/, strategies/, lists/
.\zapman.exe                          # главное окно (UAC: exe просит права администратора)
.\zapman.exe --tray                   # старт в трее: главное окно скрыто, если включена «Иконка в трее»
.\zapman.exe -fake                    # фейк в памяти: службы, winws и система не трогаются
.\zapman.exe -root D:\zapman          # другая папка с bin/, strategies/, lists/, user/
```

`-H windowsgui` собирает exe без окна консоли. Без этого флага Go собирает консольное приложение: при запуске из проводника рядом откроется окно консоли и будет жить, пока работает GUI. `go run ./cmd/zapman` тоже собирает без флага — для отладки это даже удобно: лог Wails и паника видны в терминале. Дочерние процессы (winws и системные утилиты) всегда запускаются без окна: `HideWindow` и `CREATE_NO_WINDOW`.

Корень программы — папка `zapman.exe`. У `go run` exe лежит во временной папке `go-build…`, поэтому там корнем считается рабочая папка. Чтобы запустить без прав администратора (например, `-fake` в обычном терминале), перед запуском задайте `$env:__COMPAT_LAYER = 'RunAsInvoker'`: Windows не будет смотреть на `requireAdministrator` в манифесте. Действия со службой и winws тогда ответят «нужны права администратора».

Второй экземпляр ловит блокировка Wails (`SingleInstance`); логика — в `cmd/zapman/instance.go`. Второй запуск передаёт первому путь своего exe. Первый сравнивает идентификатор этого файла (серийный номер тома и индекс файла) с тем, что запомнил при старте. Совпало — просто открывает главное окно. Не совпало (exe пересобран на месте или запущен из другой папки) — запускает новый exe с теми же аргументами и `-after-pid=<свой PID>` и выходит. Флаг нужен, потому что мьютекс Wails освобождается только с концом процесса: новый процесс ждёт, пока старый завершится, и только потом создаёт приложение. winws и служба при этом не трогаются.

### Манифест и иконка

Манифест и иконка exe — `cmd/zapman/rsrc_windows_amd64.syso` (лежит в git). Его делает [go-winres](https://github.com/tc-hib/go-winres) (закреплён в `tools/go.mod`) из `cmd/zapman/winres.json`: `requireAdministrator`, Common Controls 6, DPI per-monitor v2, версия файла и `assets/app.ico` с ID 3 (этот ID грузят Wails и трей). После правки `winres.json` или `app.ico`:

```text
go generate ./cmd/zapman
```

Почему go-winres, а не `akavel/rsrc`: на той же библиотеке `tc-hib/winres` работает `wails3 generate syso`; манифест задаётся полями JSON, а не XML руками; иконка и версия — в том же файле. Тянуть CLI `wails3` в `tools/go.mod` для этого не нужно.

Из-за манифеста тестовый бинарник пакета `cmd/zapman` тоже просил бы права администратора. Поэтому в `cmd/zapman` тестов нет, а вся логика, которую стоит тестировать, лежит в `internal/app` и `internal/gui`.

### Из чего состоит GUI

| Путь | Роль |
|---|---|
| `cmd/zapman/` | Точка входа: окна Wails, диалоги, трей (`shell.go` — реализация `gui.Shell`) |
| `cmd/zapman/frontend/` | Страницы: простые HTML, CSS и JS (ES-модули), по странице на окно; `index.html` — главное окно |
| `internal/gui/` | Логика окон PS GUI (`src/gui` на `main`) без Wails: виды окон, кнопки, тексты, трей. Тесты — с фейковым `Shell` |
| `internal/gui/backend/` | Интерфейс `Backend` (всё, что GUI просит у движка, службы и конфига) и детерминированный фейк в памяти |
| `internal/gui/i18n/` | Строки RU / EN (копия таблиц `Ui.ps1`) и строки, которых нет в PS-версии |
| `internal/app/` | Настоящий `Backend` поверх `internal/config`, `internal/engine`, `internal/service` и `internal/tools` (`wired.go`); задача трея `zapman-tray` (`schtasks.exe`) |
| `assets/assets.go` | `app.ico` для окон и трея (`go:embed`) |

Макеты окон в [`docs/ui/`](ui/index.md) сделаны PowerShell-версией (`main`); на ветке `go` генератора макетов нет.

### Как страницы говорят с Go

- `frontend/` вшит в бинарник (`//go:embed all:frontend`) и отдаётся через `application.AssetFileServerFS`. Этому серверу нужен `index.html`, поэтому главное окно — `index.html`, остальные — `<имя окна>.html` (`strategy`, `status`, `tests`, `testssetup`, `hosts`, `fake`, `games`, `download`, `diag`).
- Рантайм Wails отдаёт сам asset server по адресу `/wails/runtime.js`. Страницы импортируют его как ES-модуль (`import { Call, Events, Clipboard } from "/wails/runtime.js"` в `common.js`); своей копии рантайма в репозитории нет.
- `gui.App` зарегистрирован как сервис Wails. JS вызывает его методы по имени: `Call.ByName("github.com/NONERUN/zapman/internal/gui.App.<Метод>", ...)`. Методы — это виды окон (`Main`, `StrategyView`, `TestsView`, …) и действия (`ClickStart`, `StrategyRun`, `CloseDialog`, …). Длинная работа идёт в горутине, а метод сразу возвращается, чтобы окно не зависало.
- Когда вид меняется, Go шлёт событие без данных (`main`, `strategy`, `tests`, `download`, `diag`; `lang` — смена языка), и страница заново читает свой вид. Все решения принимает Go; JS только рисует то, что получил. Благодаря этому логику окон можно тестировать в `internal/gui` без браузера.

### Диалоги

Модальных окон в Wails нет, поэтому zapman делает модальность сам, как `ShowDialog` в WPF: пока открыт диалог, остальные окна выключены (`SetEnabled(false)`).

- Какое окно включено, решает только `gui.Modal` — стек диалогов и счётчик окон сообщений. Включено одно окно: последний диалог, а без диалогов — главное. Пока открыто окно сообщения, не включено ни одно.
- `shell.sync` в `cmd/zapman` — единственное место, где вызывается `SetEnabled`. После каждого открытия, закрытия, отказа в закрытии и после каждого окна сообщения он заново выставляет состояние **всех** окон; так случайно сбившееся состояние исправляется при следующем вызове.
- Владелец (owner) каждого диалога — главное окно, а не диалог под ним: диалог под ним может закрыться или спрятаться, а спрятанный владелец не выводит свои диалоги вперёд. Owner-окно держит диалог поверх главного и убирает его с панели задач. Окно сообщения принадлежит окну, которое сейчас впереди.
- Все пути закрытия диалога (кнопки, крестик, `Stop`) идут через `closeDialog`: сначала закрывается окно, потом просыпается поток, который ждал диалог.
- `TestModalPaths` проводит каждый диалог через ошибку, отмену и крестик и проверяет, что главное окно снова включено и не «занято». Если для страницы в `frontend/` нет такого случая, тест падает — так новый диалог не останется без проверки.
- Закрытое окно уничтожается и создаётся заново при следующем открытии.

Окна и трей живут в одном процессе (в PS-версии трей — отдельная задача планировщика). «Иконка в трее» показывает или прячет иконку. Второй запуск открывает главное окно уже работающей копии; второй запуск с `--tray` ничего не делает.

### `internal/app`

`app.New(Options)` — настоящий `Backend`; его создаёт `cmd/zapman/main.go` (`newBackend`). Пакет отделён от `internal/gui/backend` намеренно: там остаются интерфейс и фейк без вызовов системы, а SCM, процессы и `schtasks.exe` живут только здесь.

- **Настройки** — `user/config.json` через `internal/config`; неизвестные поля и поля, которых нет в GUI, сохраняются. Если файл испорчен, GUI работает на дефолтах и показывает ошибку (`ConfigError`); «Запуск» и «Установка» такой файл не перезаписывают, чтобы пользователь мог его починить.
- **`CheckEngine`** проверяет, что exe (у winws2 — и `lua/`) на месте и SHA256 файлов движка совпадают с `bin/versions.json` (`engine.CheckBin`; файлы другого движка не смотрятся). Иначе — текст `EngineBinBad` со списком `путь: состояние`.
- **«Запуск»** — проверка exe и сборки, стоп службы и всех `winws` / `winws2`, TCP timestamps, `Runner.Run` на переднем плане. Ошибка старта — текст `EngineStartFailText` с выводом движка. Процесс остаётся работать после выхода из GUI, как в PS-версии (её GUI тоже не останавливает `winws` при закрытии); после перезапуска GUI этот процесс считается «чужим» `winws`.
- **«Установка»** — `service.Install(PlanFor(build))`; стратегия и движок пишутся в конфиг только после успеха. Ошибка — текст `ServiceInstallFail` с текстом ошибки (в PS-версии показывался сырой текст исключения).
- **`SuspendService` / `RestoreService`** (для тестов стратегий) — `service.Manager.Snapshot`, удаление службы и WinDivert, а потом `Restore` той же копии. Служба возвращается точно такой, как была, даже чужая, и запускается, только если работала. Ошибка — текст `ServiceRestoreFail`.
- **Fake.** Окно fake показывает список `strategies/fakes/*.bin`; активные fake — `user/fakes/ACTIVE_DISCORD_UDP.bin` и `ACTIVE_GAME_UDP.bin` (в PS-версии они лежали в `bin/`). Выбор fake — это копия файла; какой fake выбран, определяется по SHA256. Если файла пользователя нет, окно показывает дефолт из `active` в `catalog.json` (его же берёт сборка) и файл не создаёт; создаёт его `EnsureUserFiles` при запуске, установке и тестах.
- **«Иконка в трее» при входе** — задача планировщика `zapman-tray` (имя PS-версии): `schtasks /Create /SC ONLOGON /RL HIGHEST /F /TR "\"<zapman.exe>\" --tray"`; при выключении — `/Delete`, если задача есть.
- **Сеть, hosts, загрузки, диагностика и тесты стратегий** — интерфейс `app.Tools`. `app.NewTools(b)` (`wired.go`) — реализация на `internal/tools`; `newBackend` передаёт её в `Backend.SetTools`. Без неё (`Options.Tools` пустой) методы возвращают `NotWired` с именем метода. `LatestVersion` без тега в ответе GitHub даёт пустую строку без ошибки (GUI пишет «не удалось проверить»). Действия диагностики со службами требуют прав администратора.
- **`RunTests`** повторяет `Tests.ps1`: `CheckEngine`, копия и удаление службы (если GUI не сделал этого раньше), стоп процесса «Запуска», недостающие `user:`-списки. Для DPI-тестов режим IPSet ставится в `any`, а `user/ipset-all.txt` копируется в `user/ipset-all.test-backup.txt`; копия, оставшаяся от прерванных тестов, возвращается до начала. В тесты идут только стратегии, у которых есть флаги для выбранного движка (`StrategyInfo.Supports`). После тестов — и при ошибке или отмене тоже — возвращаются ipset, затем служба, затем «Запуск» той же стратегии (без записи в конфиг). Пока идут тесты, остальные действия со службой и движком ждут.

## Релиз

Релиз собирает workflow [Release](../.github/workflows/release.yml). Его запускает push аннотированного тега `vMAJOR.MINOR.PATCH` на `origin`, и YAML берётся **из того же коммита**, что и тег. Job `build` (windows-latest) гоняет `go test ./...` и собирает `zapman.exe`; job `release` (ubuntu-latest) кладёт его в zip, rar и tar.gz и создаёт **черновик** релиза. Пока черновик не опубликован кнопкой **Publish release**, `/releases/latest` не меняется.

### Как версия попадает в exe

Версия продукта — только git-тег; в файлах репозитория её нет. Сборка релиза:

- `go-winres make ... --file-version <тег без v> --product-version <тег без v>` в `cmd/zapman` пишет версию в свойства exe (в git `.syso` лежит с `0.0.0.0`; манифест не меняется);
- `go build -tags production -trimpath -ldflags "-s -w -H windowsgui -X github.com/NONERUN/zapman/internal/app.Version=<тег>" ./cmd/zapman`. `-tags production` выключает devtools WebView2, адрес dev-сервера из переменной среды и отладочный лог Wails; `-H windowsgui` убирает окно консоли. CLI `wails3` и npm не нужны: `frontend/` вшит через `go:embed`.

`app.BuildVersion()` (заголовок окна, «Статус», проверка версии) берёт `app.Version` из `-X`, если он задан; иначе — версию модуля из build info (у `go build` в git-копии это тег или псевдоверсия, с `+dirty` при незакоммиченных правках); иначе — `dev-<коммит>`. Проверка версии сравнивает её с последним тегом GitHub как числа `vMAJOR.MINOR.PATCH`, поэтому в `-X` передаётся тег целиком, с `v`. Версия не в этом виде (`dev-<коммит>`, псевдоверсия, `+dirty`) означает сборку для разработки: обновление ей не предлагается, а «Проверить версию» только показывает последний релиз.

### Что в архиве

`zapman.exe`, `bin/`, `strategies/`, `lists/`, `docs/`, `README.md`, `LICENSE.txt`, `CHANGELOG.md` и `assets/app.LICENSE.txt` (лицензия иконки; сама иконка — внутри exe).

Этого файла (`docs/dev.md`) в архиве нет: workflow удаляет его после копирования `docs/`, потому что он нужен разработчикам, а не пользователям. `README.md` ссылается на него по адресу на GitHub. Поэтому в файлах, которые попадают в архив (`README.md`, `docs/usage.md`, `docs/troubleshooting.md`, `docs/ui/`), ссылки на `dev.md` должны быть полными адресами GitHub, а не относительными.

`.service/` в архив не нужен: «Скачать ipset» и «Сверить hosts» берут эти файлы с GitHub (ветка `main`). `user/` и `test-results/` exe создаёт сам.

### Шаги релиза

1. Если менялась схема JSON — поднять `spec` в `strategies/`.
2. Добавить в [`CHANGELOG.md`](../CHANGELOG.md) секцию `## vX.Y.Z - <дата>` для этого тега (из неё берётся текст релиза) и закоммитить — вместе **с актуальным** `release.yml`.
3. Поставить аннотированный тег **на этот коммит**:
   `git tag -a v0.2.0 -m v0.2.0`
4. Запушить ветку и тег (это делает человек): `git push origin <ветка>` и `git push origin v0.2.0`. Не `git push --tags`: в клоне могут быть чужие теги вроде `1.10.2` без `v`.
5. Дождаться проверок и зелёных job `build` и `release`.
6. Releases → черновик → проверить архивы → **Publish release**.

Release создаётся только этим workflow: релиз, созданный вручную в UI, останется без zip / rar / tar.gz.

### Повтор и исправления

Запустить сборку снова без нового тега можно вручную. YAML тогда берётся с ветки `--ref`, а код — с тега. Пока `go` не влит в `main`, на `main` лежит `release.yml` PowerShell-версии, поэтому для тега Go-версии нужен `--ref go`:

```text
gh workflow run Release --repo NONERUN/zapman --ref go -f tag=v0.2.0
```

`--repo NONERUN/zapman` пишите всегда: `gh` в этой копии может смотреть на репозиторий Flowseal.

Пока релиз — черновик, его можно удалить в UI и прогнать job снова. После Publish ассеты **immutable**: повтор job их не трогает (workflow видит, что релиз есть, и выходит).

Проверка обновлений в программе смотрит на **git-теги**, а не на черновики: тег на `origin` клиент видит сразу. Кнопка скачивания ведёт на опубликованный релиз («latest»).
