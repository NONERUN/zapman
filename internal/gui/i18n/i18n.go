// Package i18n holds the UI strings of the GUI in Russian and English.
//
// The tables in strings.go are a copy of the tables of the PowerShell
// version. extra.go holds the strings that only the Go GUI uses.
package i18n

import (
	"fmt"
	"strconv"
	"strings"
)

// The UI languages.
const (
	RU = "ru"
	EN = "en"
)

// Valid tells if code is a UI language.
func Valid(code string) bool { return code == RU || code == EN }

// FromLocale gives the UI language for a Windows UI culture, for example
// "ru-RU". A culture that starts with "ru" gives RU; other cultures give EN.
func FromLocale(culture string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(culture)), "ru") {
		return RU
	}
	return EN
}

// Pick gives the UI language: the language of the config when it is valid,
// else the language of the Windows UI culture.
func Pick(config, culture string) string {
	if Valid(config) {
		return config
	}
	return FromLocale(culture)
}

// T gives the string of the key in the language and puts the args into the
// {0}, {1} ... places. A key that is not in the language table comes from
// the English table. A key that is not in any table gives the key.
func T(lang, key string, args ...any) string {
	s, ok := table(lang)[key]
	if !ok {
		if s, ok = en[key]; !ok {
			s = key
		}
	}
	if len(args) == 0 {
		return s
	}
	return format(s, args)
}

// Table gives a copy of the table of the language, for the frontend.
func Table(lang string) map[string]string {
	out := make(map[string]string, len(en))
	for k, v := range en {
		out[k] = v
	}
	for k, v := range table(lang) {
		out[k] = v
	}
	return out
}

func table(lang string) map[string]string {
	if lang == RU {
		return ru
	}
	return en
}

// format replaces {n} with args[n], as .NET String.Format does for simple
// places. A place with no arg stays as it is.
func format(s string, args []any) string {
	var b strings.Builder
	for {
		i := strings.IndexByte(s, '{')
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		j := strings.IndexByte(s[i:], '}')
		if j < 0 {
			b.WriteString(s)
			return b.String()
		}
		n, err := strconv.Atoi(s[i+1 : i+j])
		b.WriteString(s[:i])
		if err != nil || n < 0 || n >= len(args) {
			b.WriteString(s[i : i+j+1])
		} else {
			fmt.Fprint(&b, args[n])
		}
		s = s[i+j+1:]
	}
}
