// Package config reads and writes the user settings file user\config.json.
//
// Load gives the defaults when the file is missing. A field with a value
// that is not correct keeps its default, and Load returns a FieldError that
// names the field. Save keeps the fields that this version does not know.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/NONERUN/zapman/internal/spec"
)

// FileName is the name of the settings file in the user folder.
const FileName = "config.json"

// GameFilterMode tells which protocols the Game Filter ports use.
type GameFilterMode string

// The Game Filter modes. The values are the values of the PowerShell
// version.
const (
	GameFilterOff GameFilterMode = "disabled"
	GameFilterAll GameFilterMode = "all"
	GameFilterTCP GameFilterMode = "tcp"
	GameFilterUDP GameFilterMode = "udp"
)

// OffPort is the port of the $gf tokens for a protocol when the Game Filter
// is off for it. Flowseal uses 12: no game uses it.
const OffPort = "12"

// Config is the data of user\config.json.
type Config struct {
	// Language is the UI language: "ru", "en" or "" (the language of
	// Windows).
	Language string `json:"language"`
	// Engine is the engine for a run and for the service.
	Engine spec.Engine `json:"engine"`
	// Strategy is the name of the selected strategy. "" is no selection.
	Strategy string `json:"strategy"`
	// Games are the IDs of the game modules that are on (sorted).
	Games []string `json:"games"`
	// GameOptions are the option values that the user set: game ID to
	// option name to value. An option that is not in it has the default of
	// the catalog. The catalog checks the names (engine.Games).
	GameOptions map[string]map[string]bool `json:"gameOptions,omitempty"`
	// GameFilter is the Game Filter mode.
	GameFilter GameFilterMode `json:"gameFilter"`
	// GameFilterTCP and GameFilterUDP are the port ranges of the Game
	// Filter, for example "1024-65535" or "27015,50000-50100". See
	// ParsePorts.
	GameFilterTCP string `json:"gameFilterTcp"`
	GameFilterUDP string `json:"gameFilterUdp"`
	// AutoUpdateCheck is on when the app examines GitHub for a new
	// release.
	AutoUpdateCheck bool `json:"autoUpdateCheck"`
	// TrayWatch is on when the tray starts at logon.
	TrayWatch bool `json:"trayWatch"`
	// AutoUpdateBricks is on when user strategies use the new stock
	// bricks after an update. When it is off, they keep the locked
	// bricks.
	AutoUpdateBricks bool `json:"autoUpdateBricks"`

	// extra are the fields that this version does not know. Save writes
	// them again.
	extra map[string]json.RawMessage
}

// Default gives the settings for a missing file.
func Default() Config {
	return Config{
		Engine:           spec.Winws,
		GameFilter:       GameFilterOff,
		GameFilterTCP:    DefaultPorts,
		GameFilterUDP:    DefaultPorts,
		AutoUpdateCheck:  true,
		TrayWatch:        true,
		AutoUpdateBricks: true,
	}
}

// FieldError is an error in one field of the settings file.
type FieldError struct {
	Field string
	Err   error
}

func (e *FieldError) Error() string { return fmt.Sprintf("%s: %s: %v", FileName, e.Field, e.Err) }

func (e *FieldError) Unwrap() error { return e.Err }

// field is a known field: its JSON name, a pointer to its value and a check
// of the value.
type field struct {
	name  string
	ptr   func(c *Config) any
	check func(c *Config) error
}

var fields = []field{
	{"language", func(c *Config) any { return &c.Language }, checkLanguage},
	{"engine", func(c *Config) any { return &c.Engine }, checkEngine},
	{"strategy", func(c *Config) any { return &c.Strategy }, checkStrategy},
	{"games", func(c *Config) any { return &c.Games }, checkGames},
	{"gameOptions", func(c *Config) any { return &c.GameOptions }, checkGameOptions},
	{"gameFilter", func(c *Config) any { return &c.GameFilter }, checkGameFilter},
	{"gameFilterTcp", func(c *Config) any { return &c.GameFilterTCP }, func(c *Config) error { return checkPorts(c.GameFilterTCP) }},
	{"gameFilterUdp", func(c *Config) any { return &c.GameFilterUDP }, func(c *Config) error { return checkPorts(c.GameFilterUDP) }},
	{"autoUpdateCheck", func(c *Config) any { return &c.AutoUpdateCheck }, nil},
	{"trayWatch", func(c *Config) any { return &c.TrayWatch }, nil},
	{"autoUpdateBricks", func(c *Config) any { return &c.AutoUpdateBricks }, nil},
}

func knownField(name string) bool {
	for i := range fields {
		if fields[i].name == name {
			return true
		}
	}
	return false
}

// Load reads the settings file at path. A missing file gives Default() and
// no error. A file that is not a JSON object gives Default() and an error.
// A field with an incorrect value keeps its default; the error then has a
// FieldError for each such field (errors.Join). The names of the fields
// are case-sensitive.
func Load(path string) (Config, error) {
	c := Default()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return c, fmt.Errorf("%s is not a correct JSON object. The app uses the defaults: %w", FileName, err)
	}
	var errs []error
	for i := range fields {
		f := &fields[i]
		v, ok := raw[f.name]
		if !ok {
			continue
		}
		t := c
		if err := json.Unmarshal(v, f.ptr(&t)); err != nil {
			errs = append(errs, &FieldError{f.name, fmt.Errorf("incorrect type: %s", strings.TrimSpace(string(v)))})
			continue
		}
		t.normalize(f.name)
		if f.check != nil {
			if err := f.check(&t); err != nil {
				errs = append(errs, &FieldError{f.name, err})
				continue
			}
		}
		c = t
	}
	for k, v := range raw {
		if !knownField(k) {
			if c.extra == nil {
				c.extra = map[string]json.RawMessage{}
			}
			c.extra[k] = v
		}
	}
	return c, errors.Join(errs...)
}

// normalize puts the field name of c into its usual form.
func (c *Config) normalize(name string) {
	switch name {
	case "games":
		if c.Games != nil {
			c.Games = append([]string(nil), c.Games...)
			sort.Strings(c.Games)
		}
	case "gameFilterTcp":
		c.GameFilterTCP = stripSpaces(c.GameFilterTCP)
	case "gameFilterUdp":
		c.GameFilterUDP = stripSpaces(c.GameFilterUDP)
	}
}

// Validate examines each field. The error has a FieldError for each
// incorrect field.
func (c *Config) Validate() error {
	var errs []error
	for i := range fields {
		f := &fields[i]
		if f.check == nil {
			continue
		}
		if err := f.check(c); err != nil {
			errs = append(errs, &FieldError{f.name, err})
		}
	}
	return errors.Join(errs...)
}

// Save writes c to path. It writes a temporary file in the same folder and
// then replaces the file, so a reader sees the old file or the new file.
// It makes the folder when it is missing. The fields that Load did not
// know go after the known fields.
func Save(path string, c *Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	data, err := c.marshal()
	if err != nil {
		return err
	}
	return writeAtomic(path, data)
}

// marshal gives the JSON text of c with the extra fields.
func (c *Config) marshal() ([]byte, error) {
	cc := *c
	if cc.Games == nil {
		cc.Games = []string{}
	}
	known, err := json.Marshal(&cc)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.Write(known[:len(known)-1])
	keys := make([]string, 0, len(c.extra))
	for k := range c.extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		kb, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		b.WriteByte(',')
		b.Write(kb)
		b.WriteByte(':')
		b.Write(c.extra[k])
	}
	b.WriteByte('}')
	var out bytes.Buffer
	if err := json.Indent(&out, b.Bytes(), "", "  "); err != nil {
		return nil, err
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}

// writeAtomic writes data to a temporary file in the folder of path, and
// then renames it to path.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func checkLanguage(c *Config) error {
	switch c.Language {
	case "", "ru", "en":
		return nil
	}
	return fmt.Errorf("%q is not a language. Use ru, en or an empty value", c.Language)
}

func checkEngine(c *Config) error {
	switch c.Engine {
	case spec.Winws, spec.Winws2:
		return nil
	}
	return fmt.Errorf("%q is not an engine. Use winws or winws2", c.Engine)
}

func checkStrategy(c *Config) error {
	if strings.ContainsAny(c.Strategy, `/\:*?"<>|`) || strings.TrimSpace(c.Strategy) != c.Strategy {
		return fmt.Errorf("%q is not a strategy name", c.Strategy)
	}
	return nil
}

func checkGames(c *Config) error {
	for i, id := range c.Games {
		if !isGameID(id) {
			return fmt.Errorf("%q is not a game ID. Use a-z, 0-9 and -", id)
		}
		if i > 0 && c.Games[i-1] == id {
			return fmt.Errorf("game %q is in the list two times", id)
		}
	}
	return nil
}

func checkGameOptions(c *Config) error {
	for _, id := range slices.Sorted(maps.Keys(c.GameOptions)) {
		if !isGameID(id) {
			return fmt.Errorf("%q is not a game ID. Use a-z, 0-9 and -", id)
		}
		for _, name := range slices.Sorted(maps.Keys(c.GameOptions[id])) {
			if !isGameID(name) {
				return fmt.Errorf("%s: %q is not an option name. Use a-z, 0-9 and -", id, name)
			}
		}
	}
	return nil
}

func isGameID(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}

func checkGameFilter(c *Config) error {
	switch c.GameFilter {
	case GameFilterOff, GameFilterAll, GameFilterTCP, GameFilterUDP:
		return nil
	}
	return fmt.Errorf("%q is not a Game Filter mode. Use disabled, all, tcp or udp", c.GameFilter)
}

// HasGame tells if the game id is on.
func (c *Config) HasGame(id string) bool {
	i := sort.SearchStrings(c.Games, id)
	return i < len(c.Games) && c.Games[i] == id
}

// SetGame sets the game id on or off. The list stays sorted.
func (c *Config) SetGame(id string, on bool) {
	i := sort.SearchStrings(c.Games, id)
	has := i < len(c.Games) && c.Games[i] == id
	switch {
	case on && !has:
		c.Games = append(c.Games, "")
		copy(c.Games[i+1:], c.Games[i:])
		c.Games[i] = id
	case !on && has:
		c.Games = append(c.Games[:i], c.Games[i+1:]...)
	}
}

// GameFilterPorts gives the ports of the $gf tokens for spec.Build. A
// protocol that the mode does not use gets OffPort. A range that is not
// correct (Validate gives an error for it) gets DefaultPorts.
func (c *Config) GameFilterPorts() spec.GameFilter {
	tcp, err := ParsePorts(c.GameFilterTCP)
	if err != nil {
		tcp = DefaultPorts
	}
	udp, err := ParsePorts(c.GameFilterUDP)
	if err != nil {
		udp = DefaultPorts
	}
	gf := spec.GameFilter{TCP: OffPort, UDP: OffPort}
	switch c.GameFilter {
	case GameFilterAll:
		gf.TCP, gf.UDP = tcp, udp
	case GameFilterTCP:
		gf.TCP = tcp
	case GameFilterUDP:
		gf.UDP = udp
	}
	return gf
}

// DefaultPorts is the default port range of the Game Filter.
const DefaultPorts = "1024-65535"

// ParsePorts examines a Game Filter port list and gives it without spaces.
// The rules are the rules of Flowseal service.bat: items that commas
// separate, each item is a port N or a range N-M, 1 <= N <= M <= 65535.
func ParsePorts(s string) (string, error) {
	s = stripSpaces(s)
	if s == "" {
		return "", errors.New("the port list is empty")
	}
	for _, item := range strings.Split(s, ",") {
		lo, hi, isRange := strings.Cut(item, "-")
		a, err := parsePort(lo)
		if err != nil {
			return "", fmt.Errorf("%q: %w", item, err)
		}
		b := a
		if isRange {
			if b, err = parsePort(hi); err != nil {
				return "", fmt.Errorf("%q: %w", item, err)
			}
		}
		if a > b {
			return "", fmt.Errorf("%q: the start of the range is more than its end", item)
		}
	}
	return s, nil
}

func parsePort(s string) (int, error) {
	if s == "" || len(s) > 5 {
		return 0, errors.New("use a port from 1 to 65535")
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, errors.New("use a port from 1 to 65535")
		}
		n = n*10 + int(r-'0')
	}
	if n < 1 || n > 65535 {
		return 0, errors.New("use a port from 1 to 65535")
	}
	return n, nil
}

func checkPorts(s string) error {
	if _, err := ParsePorts(s); err != nil {
		return err
	}
	if stripSpaces(s) != s {
		return errors.New("the port list has spaces")
	}
	return nil
}

func stripSpaces(s string) string {
	return strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, s)
}
