package backend

import (
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
	games := make([]GameInfo, 0, len(cat.Games))
	for _, g := range cat.Games {
		games = append(games, GameInfo{ID: g.ID, Title: g.Title})
	}
	return list, games, nil
}
