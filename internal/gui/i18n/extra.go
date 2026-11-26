package i18n

// extraEN and extraRU are the strings that only the Go GUI uses. The
// PowerShell version has no game modules.
var (
	extraEN = map[string]string{
		"BtnYes":              "Yes",
		"BtnNo":               "No",
		"BtnGames":            "Games...",
		"TipGames":            "Game modules: the rules for the selected games go before the strategy rules.",
		"GamesTitle":          "Games",
		"GamesNone":           "The catalog has no game modules.",
		"GamesNeedInstall":    "Games saved. Install the service again to apply?",
		"GamesHintInstall":    "Games saved. Install the service again to apply.",
		"GamesHintRerun":      "Games saved. Stop, then run without install.",
		"BtnGameLists":        "List",
		"TipGameLists":        "Open your list of the game in Notepad. Add IPs and subnets, one on each line. The stock list is in the lists folder.",
		"StatusRefreshFail":   "Status refresh failed.",
		"IpsetNoBackup":       "No IPSet backup found. Download the ipset list first.",
		"EngineForeignAsk":    "{0} is already running (not started by zapman). Stop it and continue?",
		"EngineForeignKeep":   "Cancelled: {0} is still running.",
		"TrayTaskFail":        "Cannot change the start of the tray at logon: {0}",
		"NotWired":            "{0}: this action is not available in this build yet.",
		"EngineStartFailText": "{0} did not start. Check the bin folder and antivirus exclusions.\n\n{1}",
		"ServiceInstallFail":  "The zapret service was not installed.\n\n{0}",
		"ServiceRestoreFail":  "The zapret service was not restored after the tests.\n\n{0}",
		"EngineBinBad":        "The engine files in bin are not the files of the pinned release (bin\\versions.json). Update the engine files.\n\n{0}",
	}
	extraRU = map[string]string{
		"BtnYes":              "Да",
		"BtnNo":               "Нет",
		"BtnGames":            "Игры...",
		"TipGames":            "Игровые модули: правила выбранных игр идут перед правилами стратегии.",
		"GamesTitle":          "Игры",
		"GamesNone":           "В каталоге нет игровых модулей.",
		"GamesNeedInstall":    "Игры сохранены. Поставить службу снова, чтобы применить?",
		"GamesHintInstall":    "Игры сохранены. Поставьте службу снова, чтобы применить.",
		"GamesHintRerun":      "Игры сохранены. Стоп, затем запуск без установки.",
		"BtnGameLists":        "Список",
		"TipGameLists":        "Открыть ваш список игры в Блокноте. Добавляйте IP и подсети, по одному в строке. Стандартный список — в папке lists.",
		"StatusRefreshFail":   "Не удалось обновить статус.",
		"IpsetNoBackup":       "Нет копии списка IPSet. Сначала скачайте список ipset.",
		"EngineForeignAsk":    "{0} уже работает (запущен не zapman). Остановить его и продолжить?",
		"EngineForeignKeep":   "Отменено: {0} продолжает работать.",
		"TrayTaskFail":        "Не удалось изменить запуск трея при входе: {0}",
		"NotWired":            "{0}: это действие пока недоступно в этой сборке.",
		"EngineStartFailText": "{0} не запустился. Проверьте папку bin и исключения антивируса.\n\n{1}",
		"ServiceInstallFail":  "Служба zapret не установлена.\n\n{0}",
		"ServiceRestoreFail":  "Служба zapret не восстановлена после тестов.\n\n{0}",
		"EngineBinBad":        "Файлы движка в bin не совпадают с файлами закреплённого релиза (bin\\versions.json). Обновите файлы движка.\n\n{0}",
	}
)

func init() {
	for k, v := range extraEN {
		en[k] = v
	}
	for k, v := range extraRU {
		ru[k] = v
	}
}
