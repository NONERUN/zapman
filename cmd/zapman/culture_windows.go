package main

import "golang.org/x/sys/windows"

// uiCulture gives the first Windows UI language of the user, for example
// "ru-RU". On an error it gives "".
func uiCulture() string {
	langs, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	if err != nil || len(langs) == 0 {
		return ""
	}
	return langs[0]
}
