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
