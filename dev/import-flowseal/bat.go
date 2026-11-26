package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// The Game Filter variables of the Flowseal .bat files. service.bat sets
// them.
const (
	gfTCPVar = "%GameFilterTCP%"
	gfUDPVar = "%GameFilterUDP%"
)

// dp0 is the folder of the .bat file.
const dp0 = "%~dp0"

// batPrefixes are the path prefixes of the .bat files after the variables
// are expanded.
var batPrefixes = []prefix{{dp0 + `bin\`, "bin/"}, {dp0 + `lists\`, "lists/"}}

// setRe finds a line set "NAME=value".
var setRe = regexp.MustCompile(`(?i)^\s*set\s+"([A-Za-z_][A-Za-z0-9_]*)=([^"]*)"\s*$`)

// varRe finds a %NAME% variable.
var varRe = regexp.MustCompile(`%[A-Za-z_][A-Za-z0-9_]*%`)

// parseBat gives the winws argv of a Flowseal .bat file. It joins the lines
// that end with ^, expands the variables of the set lines and finds the one
// line that starts winws.exe. %~dp0, %GameFilterTCP% and %GameFilterUDP%
// stay in the args. Another variable in the args is an error.
func parseBat(data []byte) (*argv, error) {
	text := strings.TrimPrefix(string(data), "\xef\xbb\xbf")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var lines []string
	cur := ""
	for _, l := range strings.Split(text, "\n") {
		t := strings.TrimRight(l, " \t")
		if strings.HasSuffix(t, "^") {
			cur += strings.TrimSuffix(t, "^") + " "
			continue
		}
		lines = append(lines, cur+l)
		cur = ""
	}
	if cur != "" {
		lines = append(lines, cur)
	}

	vars := map[string]string{}
	var words []string
	found := 0
	for _, l := range lines {
		if m := setRe.FindStringSubmatch(l); m != nil {
			v, err := expand(m[2], vars)
			if err != nil {
				return nil, fmt.Errorf("set %s: %w", m[1], err)
			}
			vars[strings.ToUpper(m[1])] = v
			continue
		}
		ws := splitWords(l)
		for i, w := range ws {
			exe, err := expand(batWord(w), vars)
			if err != nil {
				continue
			}
			low := strings.ToLower(exe)
			if strings.HasSuffix(low, "winws2.exe") {
				return nil, errors.New("the .bat file starts winws2.exe: the importer reads only winws")
			}
			if low != "winws.exe" && !strings.HasSuffix(low, `\winws.exe`) {
				continue
			}
			found++
			for _, a := range ws[i+1:] {
				v, err := expand(batWord(a), vars)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", a, err)
				}
				words = append(words, v)
			}
			break
		}
	}
	switch {
	case found == 0:
		return nil, errors.New("no line starts winws.exe")
	case found > 1:
		return nil, fmt.Errorf("%d lines start winws.exe; only one is supported", found)
	}
	return parseArgs(words, batPrefixes)
}

// batWord removes the double quotes of a word. Out of quotes, ^ escapes
// the next character: cmd removes the ^ (for example ^! gives !).
func batWord(w string) string {
	var b strings.Builder
	quoted, esc := false, false
	for _, r := range w {
		switch {
		case esc:
			b.WriteRune(r)
			esc = false
		case r == '"':
			quoted = !quoted
		case r == '^' && !quoted:
			esc = true
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// expand replaces the variables of vars in s (with no case). %~dp0 and the
// Game Filter variables stay. Another variable is an error.
func expand(s string, vars map[string]string) (string, error) {
	var bad string
	out := varRe.ReplaceAllStringFunc(s, func(v string) string {
		if strings.EqualFold(v, gfTCPVar) || strings.EqualFold(v, gfUDPVar) {
			return v
		}
		if x, ok := vars[strings.ToUpper(strings.Trim(v, "%"))]; ok {
			return x
		}
		if bad == "" {
			bad = v
		}
		return v
	})
	if bad != "" {
		return "", fmt.Errorf("unknown variable %s", bad)
	}
	// %~dp0 is not a %NAME% variable; other %~ forms are not supported.
	if strings.Contains(strings.ReplaceAll(out, dp0, ""), "%~") {
		return "", fmt.Errorf("unknown variable in %q", out)
	}
	return out, nil
}
