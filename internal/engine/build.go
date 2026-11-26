package engine

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/NONERUN/zapman/internal/config"
	"github.com/NONERUN/zapman/internal/spec"
)

// MaxCommandLine is the maximum length of a command line in UTF-16 units
// (CreateProcess: 32767 with the terminating null). The service ImagePath
// has the same limit, because the SCM gives it to CreateProcess.
const MaxCommandLine = 32766

// ErrNoStrategy is the error of Build when the config has no strategy.
var ErrNoStrategy = errors.New("no strategy is selected")

// Build is a strategy with its games, built for one engine.
type Build struct {
	Strategy *spec.Strategy
	Games    []*spec.Game
	Engine   spec.Engine
	Exe      string // full path of winws.exe or winws2.exe
	Bin      string // the work folder of the engine
	// Lines are the argv lines of spec.Build.
	Lines []string
}

// Build builds the selected strategy of cfg with the games that are on.
// It gives an error when the strategy or a game is not in the catalog, when
// the exe of the engine is missing, or when the command line is too long.
func (l Layout) Build(cat *spec.Catalog, cfg *config.Config) (*Build, error) {
	if cfg.Strategy == "" {
		return nil, ErrNoStrategy
	}
	st, err := cat.Strategy(cfg.Strategy)
	if err != nil {
		return nil, err
	}
	games, err := Games(cat, cfg)
	if err != nil {
		return nil, err
	}
	exe, err := l.Exe(cfg.Engine)
	if err != nil {
		return nil, err
	}
	p, err := l.Paths()
	if err != nil {
		return nil, err
	}
	lines, err := spec.Build(st, games, cfg.Engine, p, cfg.GameFilterPorts())
	if err != nil {
		return nil, err
	}
	b := &Build{Strategy: st, Games: games, Engine: cfg.Engine, Exe: exe, Bin: l.Bin, Lines: lines}
	if err := checkLength(b.CommandLine()); err != nil {
		return nil, err
	}
	return b, nil
}

// Games gives the games of cfg that are on, with the option values of
// cfg.GameOptions. A game that is not in the catalog is an error. An entry of
// cfg.GameOptions with a game or an option that is not in the catalog is
// also an error; an entry of a game that is off is not used.
func Games(cat *spec.Catalog, cfg *config.Config) ([]*spec.Game, error) {
	for _, id := range slices.Sorted(maps.Keys(cfg.GameOptions)) {
		g := cat.Game(id)
		if g == nil {
			return nil, &config.FieldError{Field: "gameOptions", Err: fmt.Errorf("game %q is not in the catalog", id)}
		}
		if _, err := g.With(cfg.GameOptions[id]); err != nil {
			return nil, &config.FieldError{Field: "gameOptions", Err: err}
		}
	}
	games := make([]*spec.Game, 0, len(cfg.Games))
	for _, id := range cfg.Games {
		g := cat.Game(id)
		if g == nil {
			return nil, &config.FieldError{Field: "games", Err: fmt.Errorf("game %q is not in the catalog", id)}
		}
		g, err := g.With(cfg.GameOptions[id])
		if err != nil {
			return nil, &config.FieldError{Field: "gameOptions", Err: err}
		}
		games = append(games, g)
	}
	return games, nil
}

// checkLength gives an error when the command line is longer than
// MaxCommandLine.
func checkLength(cmdLine string) error {
	if n := utf16Len(cmdLine); n > MaxCommandLine {
		return fmt.Errorf("the command line has %d characters; the limit of Windows is %d. Turn off some games or use a shorter zapman folder path", n, MaxCommandLine)
	}
	return nil
}

// Args gives the arguments as one string: the lines without the spaces at
// the ends, with one space between them. For winws2 it puts
// --chdir="<bin>" first when the lines do not have --chdir: the service
// starts in System32, and winws2 finds its Lua files from the work folder.
// This is the same text as the PowerShell version.
func (b *Build) Args() string {
	parts := make([]string, 0, len(b.Lines)+1)
	if b.Engine == spec.Winws2 && !hasChdir(b.Lines) {
		parts = append(parts, `--chdir="`+strings.TrimRight(b.Bin, `\`)+`"`)
	}
	for _, l := range b.Lines {
		if l = strings.TrimSpace(l); l != "" {
			parts = append(parts, l)
		}
	}
	return strings.Join(parts, " ")
}

func hasChdir(lines []string) bool {
	for _, l := range lines {
		for _, f := range strings.Fields(l) {
			if strings.HasPrefix(f, "--chdir=") {
				return true
			}
		}
	}
	return false
}

// CommandLine gives the full command line: "<exe>" <Args>. It is also the
// ImagePath of the service.
func (b *Build) CommandLine() string {
	return `"` + b.Exe + `" ` + b.Args()
}

// GameVersions gives "<id> <version>" for each game, for example "sky 3".
// A game with options also has the option values, for example
// "photon 3 ipset=off". The service record keeps these texts: a change of a
// game version or of an option makes the service stale.
func (b *Build) GameVersions() []string {
	out := make([]string, len(b.Games))
	for i, g := range b.Games {
		out[i] = g.ID + " " + strconv.Itoa(g.GameFile.Version)
		if t := g.OptionText(); t != "" {
			out[i] += " " + t
		}
	}
	return out
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}
