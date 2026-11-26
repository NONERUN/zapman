package backend

import (
	"fmt"
	"maps"
	"slices"

	"github.com/NONERUN/zapman/internal/spec"
)

// LoadCatalogInfo reads the strategies and the games of the catalog folder
// dir. A strategy supports an engine when spec.Build gives the argv of the
// strategy for that engine. The function only reads the catalog.
func LoadCatalogInfo(dir string) ([]StrategyInfo, []GameInfo, error) {
	cat, err := spec.LoadCatalog(dir, "")
	if err != nil {
		return nil, nil, err
	}
	names, err := cat.StrategyNames()
	if err != nil {
		return nil, nil, err
	}
	var list []StrategyInfo
	for _, name := range names {
		st, err := cat.Strategy(name)
		if err != nil {
			return nil, nil, err
		}
		info := StrategyInfo{Name: name}
		for _, e := range []spec.Engine{spec.Winws, spec.Winws2} {
			if _, err := spec.Build(st, nil, e, spec.Paths{}, spec.GameFilter{TCP: "12", UDP: "12"}); err == nil {
				info.Engines = append(info.Engines, e)
			}
		}
		list = append(list, info)
	}
	return list, GameInfos(cat), nil
}

// GameInfos gives the game modules of the catalog, in build order.
func GameInfos(cat *spec.Catalog) []GameInfo {
	games := make([]GameInfo, 0, len(cat.Games))
	for _, g := range cat.Games {
		info := GameInfo{ID: g.ID, Title: g.Title, About: maps.Clone(g.About), UserLists: g.UserLists()}
		for _, id := range slices.Sorted(maps.Keys(g.Options)) {
			o := g.Options[id]
			info.Options = append(info.Options, GameOptionInfo{
				ID: id, Title: maps.Clone(o.Title), About: maps.Clone(o.About), Default: o.Default,
			})
		}
		games = append(games, info)
	}
	return games
}

// GameOverrides checks the option values opts against the games and gives
// only the values that are not the defaults. A game or an option that is
// not in games is an error. The result is nil when all values are the
// defaults.
func GameOverrides(games []GameInfo, opts map[string]map[string]bool) (map[string]map[string]bool, error) {
	var out map[string]map[string]bool
	for _, id := range slices.Sorted(maps.Keys(opts)) {
		i := slices.IndexFunc(games, func(g GameInfo) bool { return g.ID == id })
		if i < 0 {
			return nil, fmt.Errorf("unknown game %q", id)
		}
		for _, name := range slices.Sorted(maps.Keys(opts[id])) {
			j := slices.IndexFunc(games[i].Options, func(o GameOptionInfo) bool { return o.ID == name })
			if j < 0 {
				return nil, fmt.Errorf("game %q has no option %q", id, name)
			}
			if v := opts[id][name]; v != games[i].Options[j].Default {
				if out == nil {
					out = map[string]map[string]bool{}
				}
				if out[id] == nil {
					out[id] = map[string]bool{}
				}
				out[id][name] = v
			}
		}
	}
	return out, nil
}

// CloneGameOptions gives a deep copy of the option values.
func CloneGameOptions(opts map[string]map[string]bool) map[string]map[string]bool {
	if opts == nil {
		return nil
	}
	out := make(map[string]map[string]bool, len(opts))
	for id, v := range opts {
		out[id] = maps.Clone(v)
	}
	return out
}
