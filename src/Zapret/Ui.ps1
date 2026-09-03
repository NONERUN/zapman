# Zapret module: UI language and strings (RU / EN).

Set-StrictMode -Version Latest

$script:ZapretUiLang = 'en'

$script:ZapretUiEn = @{
    AppName              = 'Zapret'
    AppTitle             = 'Zapret Manager v{0}'
    LangRu               = 'RU'
    LangEn               = 'EN'
    GrpStatus            = 'Status'
    GrpSettings          = 'Settings'
    GrpTools             = 'Tools'
    BtnStart             = 'Start service'
    BtnStop              = 'Stop'
    BtnRemove            = 'Remove services'
    BtnStrategy          = 'Strategy...'
    BtnFakes             = 'Replace fakes'
    BtnIpset             = 'Download ipset'
    BtnHosts             = 'Compare hosts'
    BtnVersion           = 'Check version'
    BtnDiag              = 'Diagnostics'
    BtnRunSelected       = 'Run selected'
    BtnInstall           = 'Install service'
    BtnTests             = 'Run tests'
    BtnCopy              = 'Copy'
    BtnClose             = 'Close'
    BtnOk                = 'OK'
    BtnCancel            = 'Cancel'
    BtnReplace           = 'Replace'
    ChkAuto              = 'Auto-Update Check'
    LblGame              = 'Game Filter'
    LblIpset             = 'IPSet Filter'
    LblEngine            = 'Engine'
    EngineNoFlags        = 'This strategy has no {0} flags.'
    EngineNoWinws        = 'winws.exe is not found in the bin folder.'
    EngineNoWinws2       = 'winws2.exe or bin\lua is not found. Add the zapret2 files to bin.'
    EngineStartFail      = '{0} did not start. Check the bin folder and antivirus exclusions.'
    EngineInstallFail    = '{0} did not start. The service was not installed.'
    EngineServiceFail    = 'The service started, but {0} is not running.'
    EngineCmdFail        = 'Failed to read the {0} command line.'
    StratTitle           = 'Strategy'
    StatusDlgTitle       = 'Status and journal'
    JournalHeader        = 'Journal'
    StatusBypassOn       = 'Bypass: running ({0})'
    StatusBypassOff      = 'Bypass: not running'
    StatusServiceOn      = 'Service: installed ({0})'
    StatusServiceOff     = 'Service: not installed'
    StatusStrategy       = 'Strategy: {0}'
    StatusStrategyNone   = 'Strategy: -'
    StatusStrategyGone   = 'Strategy: {0} (no files in strategies/)'
    StatusStrategyManual = 'Strategy: - (manual run)'
    StatusStrategyRun    = 'Strategy: {0} (manual run)'
    StatusDivertOn       = 'WinDivert: running'
    StatusDivertPending  = 'WinDivert: STOP_PENDING'
    StatusDivertOther    = 'WinDivert: {0}'
    StatusDivertNone     = 'WinDivert: not installed'
    BannerNoService      = 'No service. Start a strategy from Strategy..., or install one for autostart.'
    BannerMismatch       = 'Service is installed, {0} is not running. Use Start service.'
    BannerOrphan         = 'Bypass is running from Strategy..., without autostart.'
    BannerNoStrategies   = 'No .ps1 files in strategies. Strategy... is unavailable. You can still start the installed service.'
    BannerStopPending    = 'WinDivert is STOP_PENDING. Remove services (reboot if it stays). Do not install over this.'
    TipStart             = 'Start the installed zapret service.'
    TipStop              = 'Stop winws / winws2 and the zapret service if it is running.'
    TipRemove            = 'Remove zapret and WinDivert services. No extra confirmation.'
    TipStrategy          = 'Pick a strategy: try it now, install as a service, or run tests.'
    TipFakes             = 'Replace ACTIVE_*.bin in bin (Discord UDP / Game UDP).'
    TipIpset             = 'Download ipset-all.txt from the repository.'
    TipHosts             = 'Compare the Windows hosts file with the repo copy. Does not overwrite hosts; opens Notepad if they differ.'
    TipVersion           = 'Compare this build version with GitHub. Does not download a new archive.'
    TipDiag              = 'Check conflicts, path, DNS, WinDivert and Discord cache.'
    TipAuto              = 'When enabled, the GUI checks the app version at start. The check does not block the window.'
    TipStatusClick       = 'Click for full status, WinDivert64.sys and the action journal.'
    ConfirmRunOver       = 'Stop the running service or bypass process, then start the selected strategy?'
    FilterNeedInstall    = 'Game Filter is saved. Install the service again to apply it?'
    FilterNeedRestart    = 'IPSet Filter is saved. Stop, then start the service to apply it?'
    FilterNeedRerun      = 'Filter is saved. Stop, then run a strategy from Strategy... to apply it.'
    AdminRequired        = 'Administrator rights are required.'
    BinMissing           = 'The bin folder is not found. Extract the full Zapret archive first.'
    NoStrategies         = 'No strategy files found in the strategies folder.'
    TestsNeedNoService   = 'The zapret service will be removed so tests can run.'
    TestsCancelled       = 'Tests cancelled.'
    TestsFinished        = 'Tests finished.'
    TestsTitle           = 'Tests'
    FakesTitle           = 'Replace fakes'
    FakesSlot            = 'Slot'
    FakesFile            = 'Fake file'
    FakesNone            = 'No .bin files were found in the bin folder.'
    TrayOpen             = 'Open'
    TrayExit             = 'Exit'
    TrayRunning          = 'Zapret: {0} running'
    TrayStopped          = 'Zapret: {0} not running'
    VersionNew           = 'New version available: {0}. Open the releases page?'
    VersionLatest        = 'Latest version installed: {0}'
    VersionFail          = 'Failed to fetch the latest version. This warning does not affect zapret.'
    VersionChecking      = 'Checking version...'
    AutoFail             = 'Version check failed. Zapret still works.'
    HostsTitle           = 'Compare hosts'
    HostsNeed            = 'Hosts file needs an update. Copy the notepad text into the hosts file as Administrator.'
    HostsOk              = 'Hosts file is up to date.'
    HostsFail            = 'Failed to download hosts file from the repository.'
    HostsCancel          = 'Hosts compare cancelled.'
    IpsetTitle           = 'Download ipset'
    IpsetOk              = 'IPSet list updated.'
    IpsetCancel          = 'IPSet download cancelled.'
    DiagTitle            = 'Diagnostics'
    DiagConflicts        = 'Remove these conflicting services?'
    DiagCache            = 'Clear Discord cache (Stable, PTB, Canary, Development)?'
    DiagRun              = 'Run'
    DiagDone             = 'Diagnostics finished.'
    RemoveDone           = 'Services removed.'
    StopDone             = 'Stopped.'
    StartDone            = 'Service started.'
    StartDoneName        = 'Started service ({0}).'
    RunDone              = 'Running {0} (not installed as a service).'
    InstallDone          = 'Installed service: {0}.'
    FakeDone             = 'Replaced {0} fake with {1}.'
    MenuStrategy         = 'Strategy (run / install / tests)'
    MenuStart            = 'Start service'
    MenuStop             = 'Stop'
    MenuRemove           = 'Remove services'
    MenuStatus           = 'Status'
    MenuGame             = 'Game Filter'
    MenuIpset            = 'IPSet Filter'
    MenuAuto             = 'Auto-Update Check'
    MenuFakes            = 'Replace fakes'
    MenuIpsetDl          = 'Download ipset'
    MenuHosts            = 'Compare hosts'
    MenuVersion          = 'Check version'
    MenuDiag             = 'Diagnostics'
    MenuExit             = 'Exit'
    PickStrategy         = 'Pick a strategy:'
    CancelItem           = '0. Cancel'
    InvalidChoice        = 'Invalid choice.'
    IpsetNow             = 'IPSet Filter is now: {0}'
    AutoOn               = 'Auto-Update Check enabled.'
    AutoOff              = 'Auto-Update Check disabled.'
    ConsoleUtf8Warn      = 'Console UTF-8 may not display all characters on this system.'
    CliTitle             = 'Zapret CLI'
    CliService           = 'Service manager'
    CliTests             = 'Strategy tests'
    CliEnv               = 'Check environment'
    CliUnknown           = 'Unknown command: {0}'
    CliUsage             = "cli.bat`r`ncli.bat service`r`ncli.bat tests [-TestType standard|dpi] [-Strategies all|name,...] [-NoPause]`r`ncli.bat env"
}

$script:ZapretUiRu = @{
    AppName              = 'Zapret'
    AppTitle             = 'Zapret Manager v{0}'
    LangRu               = 'RU'
    LangEn               = 'EN'
    GrpStatus            = 'Status'
    GrpSettings          = 'Settings'
    GrpTools             = 'Tools'
    BtnStart             = 'Старт службы'
    BtnStop              = 'Стоп'
    BtnRemove            = 'Снять службы'
    BtnStrategy          = 'Стратегия...'
    BtnFakes             = 'Подменить fake'
    BtnIpset             = 'Скачать ipset'
    BtnHosts             = 'Сверить hosts'
    BtnVersion           = 'Проверить версию'
    BtnDiag              = 'Диагностика'
    BtnRunSelected       = 'Запустить выбранную'
    BtnInstall           = 'Установить службу'
    BtnTests             = 'Прогнать тесты'
    BtnCopy              = 'Копировать'
    BtnClose             = 'Закрыть'
    BtnOk                = 'OK'
    BtnCancel            = 'Отмена'
    BtnReplace           = 'Заменить'
    ChkAuto              = 'Auto-Update Check'
    LblGame              = 'Game Filter'
    LblIpset             = 'IPSet Filter'
    LblEngine            = 'Движок'
    EngineNoFlags        = 'У этой стратегии нет флагов {0}.'
    EngineNoWinws        = 'winws.exe не найден в папке bin.'
    EngineNoWinws2       = 'Нет winws2.exe или bin\lua. Положите файлы zapret2 в bin.'
    EngineStartFail      = '{0} не запустился. Проверьте папку bin и исключения антивируса.'
    EngineInstallFail    = '{0} не запустился. Служба не установлена.'
    EngineServiceFail    = 'Служба стартовала, но {0} не запущен.'
    EngineCmdFail        = 'Не удалось прочитать командную строку {0}.'
    StratTitle           = 'Стратегия'
    StatusDlgTitle       = 'Статус и журнал'
    JournalHeader        = 'Журнал'
    StatusBypassOn       = 'Обход: работает ({0})'
    StatusBypassOff      = 'Обход: не запущен'
    StatusServiceOn      = 'Служба: установлена ({0})'
    StatusServiceOff     = 'Служба: не установлена'
    StatusStrategy       = 'Стратегия: {0}'
    StatusStrategyNone   = 'Стратегия: -'
    StatusStrategyGone   = 'Стратегия: {0} (нет файлов в strategies/)'
    StatusStrategyManual = 'Стратегия: - (ручной запуск)'
    StatusStrategyRun    = 'Стратегия: {0} (ручной запуск)'
    StatusDivertOn       = 'WinDivert: running'
    StatusDivertPending  = 'WinDivert: STOP_PENDING'
    StatusDivertOther    = 'WinDivert: {0}'
    StatusDivertNone     = 'WinDivert: not installed'
    BannerNoService      = 'Службы нет. Запуск/установка — через «Стратегия...».'
    BannerMismatch       = 'Служба стоит, процесс {0} не запущен. Нажмите «Старт службы».'
    BannerOrphan         = 'Обход запущен из «Стратегия...», без автозапуска.'
    BannerNoStrategies   = 'В папке strategies нет .ps1. «Стратегия...» недоступна. Старт службы ещё можно.'
    BannerStopPending    = 'WinDivert завис (STOP_PENDING). Снимите службы, перезагрузка если не отпускает. Не ставьте службу поверх.'
    TipStart             = 'Запустить установленную службу zapret.'
    TipStop              = 'Остановить winws / winws2 и службу zapret, если она запущена.'
    TipRemove            = 'Снять службы zapret и WinDivert. Без дополнительного вопроса.'
    TipStrategy          = 'Выбрать стратегию: попробовать сейчас, поставить службу или прогнать тесты.'
    TipFakes             = 'Подменить ACTIVE_*.bin в bin (Discord UDP / Game UDP).'
    TipIpset             = 'Скачать ipset-all.txt из репозитория.'
    TipHosts             = 'Сравнить файл hosts Windows с копией из репозитория. Сам hosts не перезаписывает; при расхождении откроет блокнот.'
    TipVersion           = 'Сравнить версию этой сборки с GitHub. Архив сами не скачивает.'
    TipDiag              = 'Конфликты, путь, DNS, WinDivert и кэш Discord.'
    TipAuto              = 'Если включено, GUI сверяет версию при старте. Проверка не блокирует окно.'
    TipStatusClick       = 'Клик — полный статус, WinDivert64.sys и журнал действий.'
    ConfirmRunOver       = 'Остановить службу или процесс обхода и запустить выбранную стратегию?'
    FilterNeedInstall    = 'Game Filter сохранён. Поставить службу снова, чтобы применить?'
    FilterNeedRestart    = 'IPSet Filter сохранён. Остановить и снова запустить службу, чтобы применить?'
    FilterNeedRerun      = 'Фильтр сохранён. Стоп, затем запуск из «Стратегия...», чтобы применить.'
    AdminRequired        = 'Нужны права администратора.'
    BinMissing           = 'Папка bin не найдена. Сначала распакуйте полный архив Zapret.'
    NoStrategies         = 'В папке strategies нет файлов стратегий.'
    TestsNeedNoService   = 'Служба zapret будет снята, чтобы можно было прогнать тесты.'
    TestsCancelled       = 'Тесты отменены.'
    TestsFinished        = 'Тесты закончены.'
    TestsTitle           = 'Тесты'
    FakesTitle           = 'Подменить fake'
    FakesSlot            = 'Слот'
    FakesFile            = 'Файл fake'
    FakesNone            = 'В папке bin нет файлов .bin.'
    TrayOpen             = 'Открыть'
    TrayExit             = 'Выход'
    TrayRunning          = 'Zapret: {0} running'
    TrayStopped          = 'Zapret: {0} not running'
    VersionNew           = 'Доступна новая версия: {0}. Открыть страницу релизов?'
    VersionLatest        = 'Установлена последняя версия: {0}'
    VersionFail          = 'Не удалось получить последнюю версию. На работу zapret это не влияет.'
    VersionChecking      = 'Проверка версии...'
    AutoFail             = 'Проверка версии не удалась. Zapret продолжает работать.'
    HostsTitle           = 'Сверить hosts'
    HostsNeed            = 'Файл hosts нужно обновить. Скопируйте текст из блокнота в hosts от имени администратора.'
    HostsOk              = 'Файл hosts актуален.'
    HostsFail            = 'Не удалось скачать hosts из репозитория.'
    HostsCancel          = 'Сверка hosts отменена.'
    IpsetTitle           = 'Скачать ipset'
    IpsetOk              = 'Список IPSet обновлён.'
    IpsetCancel          = 'Загрузка IPSet отменена.'
    DiagTitle            = 'Диагностика'
    DiagConflicts        = 'Снять эти конфликтующие службы?'
    DiagCache            = 'Очистить кэш Discord (Stable, PTB, Canary, Development)?'
    DiagRun              = 'Запуск'
    DiagDone             = 'Диагностика закончена.'
    RemoveDone           = 'Службы сняты.'
    StopDone             = 'Остановлено.'
    StartDone            = 'Служба запущена.'
    StartDoneName        = 'Запущена служба ({0}).'
    RunDone              = 'Запущено {0} (не как служба).'
    InstallDone          = 'Установлена служба: {0}.'
    FakeDone             = 'Слот {0} заменён на {1}.'
    MenuStrategy         = 'Стратегия (запуск / установка / тесты)'
    MenuStart            = 'Старт службы'
    MenuStop             = 'Стоп'
    MenuRemove           = 'Снять службы'
    MenuStatus           = 'Статус'
    MenuGame             = 'Game Filter'
    MenuIpset            = 'IPSet Filter'
    MenuAuto             = 'Auto-Update Check'
    MenuFakes            = 'Подменить fake'
    MenuIpsetDl          = 'Скачать ipset'
    MenuHosts            = 'Сверить hosts'
    MenuVersion          = 'Проверить версию'
    MenuDiag             = 'Диагностика'
    MenuExit             = 'Выход'
    PickStrategy         = 'Выберите стратегию:'
    CancelItem           = '0. Отмена'
    InvalidChoice        = 'Неверный выбор.'
    IpsetNow             = 'IPSet Filter сейчас: {0}'
    AutoOn               = 'Auto-Update Check включён.'
    AutoOff              = 'Auto-Update Check выключен.'
    ConsoleUtf8Warn      = 'UTF-8 в консоли на этой системе может отображаться не полностью.'
    CliTitle             = 'Zapret CLI'
    CliService           = 'Менеджер службы'
    CliTests             = 'Тесты стратегий'
    CliEnv               = 'Проверить среду'
    CliUnknown           = 'Неизвестная команда: {0}'
    CliUsage             = "cli.bat`r`ncli.bat service`r`ncli.bat tests [-TestType standard|dpi] [-Strategies all|name,...] [-NoPause]`r`ncli.bat env"
}

function Test-ZapretUiLanguageCode {
    param([string]$Code)
    if ($Code -eq 'ru' -or $Code -eq 'en') {
        return $true
    }
    return $false
}

function Get-ZapretUiLanguageFromOs {
    $ui = [string]$PSUICulture
    if ([string]::IsNullOrWhiteSpace($ui)) {
        $ui = [System.Globalization.CultureInfo]::CurrentUICulture.Name
    }
    if ($ui -like 'ru*') {
        return 'ru'
    }
    return 'en'
}

function Initialize-ZapretUiLanguage {
    $fromCfg = ''
    try {
        $fromCfg = ([string](Get-ZapretConfig).language).Trim().ToLowerInvariant()
    } catch {
        $fromCfg = ''
    }
    if (Test-ZapretUiLanguageCode $fromCfg) {
        $script:ZapretUiLang = $fromCfg
        return $script:ZapretUiLang
    }
    $script:ZapretUiLang = Get-ZapretUiLanguageFromOs
    return $script:ZapretUiLang
}

function Get-ZapretUiLanguage {
    return $script:ZapretUiLang
}

function Set-ZapretUiLanguage {
    param([string]$Language)
    $code = ([string]$Language).Trim().ToLowerInvariant()
    if (-not (Test-ZapretUiLanguageCode $code)) {
        throw 'UI language must be ru or en.'
    }
    $script:ZapretUiLang = $code
    [void](Update-ZapretConfig -Language $code)
}

function Get-ZapretUiString {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Key,
        [object[]]$FormatArgs = @()
    )
    $table = $script:ZapretUiEn
    if ($script:ZapretUiLang -eq 'ru') {
        $table = $script:ZapretUiRu
    }
    $text = ''
    if ($table.ContainsKey($Key)) {
        $text = [string]$table[$Key]
    } elseif ($script:ZapretUiEn.ContainsKey($Key)) {
        $text = [string]$script:ZapretUiEn[$Key]
    } else {
        $text = $Key
    }
    $argsList = @($FormatArgs)
    if ($argsList.Count -gt 0) {
        return ($text -f $argsList)
    }
    return $text
}

function Enable-ZapretConsoleUtf8 {
    try {
        [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
        [Console]::InputEncoding = [System.Text.Encoding]::UTF8
        $OutputEncoding = [System.Text.Encoding]::UTF8
    } catch {
        return
    }
}
