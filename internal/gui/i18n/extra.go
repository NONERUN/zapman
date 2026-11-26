package i18n

// extraEN and extraRU are the strings that only the Go GUI uses. The
// PowerShell version has no game modules.
var (
	extraEN = map[string]string{
		"BtnYes":            "Yes",
		"BtnNo":             "No",
		"BtnGames":          "Games...",
		"TipGames":          "Game modules: the rules for the selected games go before the strategy rules.",
		"GamesTitle":        "Games",
		"GamesNone":         "The catalog has no game modules.",
		"GamesNeedInstall":  "Games saved. Install the service again to apply?",
		"GamesHintInstall":  "Games saved. Install the service again to apply.",
		"GamesHintRerun":    "Games saved. Stop, then run without install.",
		"StatusRefreshFail": "Status refresh failed.",
		"IpsetNoBackup":     "No IPSet backup found. Download the ipset list first.",
		"EngineForeignAsk":  "{0} is already running (not started by zapman). Stop it and continue?",
		"EngineForeignKeep": "Cancelled: {0} is still running.",
		"TrayTaskFail":      "Cannot change the start of the tray at logon: {0}",
		"NotWired":          "{0}: this action is not available in this build yet.",
	}
	extraRU = map[string]string{
		"BtnYes":            "Да",
		"BtnNo":             "Нет",
		"BtnGames":          "Игры...",
		"TipGames":          "Игровые модули: правила выбранных игр идут перед правилами стратегии.",
		"GamesTitle":        "Игры",
		"GamesNone":         "В каталоге нет игровых модулей.",
		"GamesNeedInstall":  "Игры сохранены. Поставить службу снова, чтобы применить?",
		"GamesHintInstall":  "Игры сохранены. Поставьте службу снова, чтобы применить.",
		"GamesHintRerun":    "Игры сохранены. Стоп, затем запуск без установки.",
		"StatusRefreshFail": "Не удалось обновить статус.",
		"IpsetNoBackup":     "Нет копии списка IPSet. Сначала скачайте список ipset.",
		"EngineForeignAsk":  "{0} уже работает (запущен не zapman). Остановить его и продолжить?",
		"EngineForeignKeep": "Отменено: {0} продолжает работать.",
		"TrayTaskFail":      "Не удалось изменить запуск трея при входе: {0}",
		"NotWired":          "{0}: это действие пока недоступно в этой сборке.",
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
