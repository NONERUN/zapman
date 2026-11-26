package spec

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Build gives the argv lines of the strategy with the games for the engine.
// The rules of the games go first, in the order of Catalog.Games (Order,
// then ID), because they are more specific than the strategy rules on the
// same ports. Then the strategy rules follow. The wf line is the union of
// the target ports (see WFPorts). An active: ref uses the user file when
// p.ActiveFiles has its name, else the default of the catalog of st.
func Build(st *Strategy, games []*Game, engine Engine, p Paths, gf GameFilter) ([]string, error) {
	if st == nil {
		return nil, errors.New("Strategy spec is empty.")
	}
	p.activeDefault = st.active
	gs := append([]*Game(nil), games...)
	sortGames(gs)
	var rules []resolvedRule
	for i, g := range gs {
		if g == nil {
			return nil, errors.New("Game is empty.")
		}
		if i > 0 && gs[i-1].ID == g.ID {
			return nil, fmt.Errorf("Game %q is in the list two times.", g.ID)
		}
		rules = append(rules, g.rules...)
	}
	rules = append(rules, st.rules...)
	s := &V01Spec{WF: &V01WF{}}
	s.WF.TCP, s.WF.UDP = wfPorts(rules)
	for _, r := range rules {
		s.Profiles = append(s.Profiles, r.profile)
		s.origins = append(s.origins, r.origin)
	}
	return BuildV01(s, engine, p, gf)
}

// WFPorts gives the wf ports of the strategy with the games, as v0.1 port
// items ($gf.Tcp and $gf.Udp for the Game Filter).
func WFPorts(st *Strategy, games []*Game) (tcp, udp []string) {
	var rules []resolvedRule
	for _, g := range games {
		rules = append(rules, g.rules...)
	}
	return wfPorts(append(rules, st.rules...))
}

// wfPorts gives the union of the target ports. Each item occurs one time.
// The items are sorted by the first port number; the Game Filter item is
// the last. Ranges that overlap stay as they are.
func wfPorts(rules []resolvedRule) (tcp, udp []string) {
	collect := func(get func(*resolvedRule) []string, gfToken string) []string {
		seen := map[string]bool{}
		var out []string
		for i := range rules {
			for _, it := range get(&rules[i]) {
				if seen[it] {
					continue
				}
				seen[it] = true
				out = append(out, it)
			}
		}
		sort.SliceStable(out, func(i, j int) bool { return portLess(out[i], out[j]) })
		for i, it := range out {
			if it == GameFilterPort {
				out[i] = gfToken
			}
		}
		return out
	}
	tcp = collect(func(r *resolvedRule) []string { return r.tcp }, "$gf.Tcp")
	udp = collect(func(r *resolvedRule) []string { return r.udp }, "$gf.Udp")
	return tcp, udp
}

// portLess orders port items by the first port, then by the text. The Game
// Filter item is after all other items.
func portLess(a, b string) bool {
	if (a == GameFilterPort) != (b == GameFilterPort) {
		return b == GameFilterPort
	}
	na, nb := firstPort(a), firstPort(b)
	if na != nb {
		return na < nb
	}
	return a < b
}

func firstPort(s string) int {
	lo, _, _ := strings.Cut(s, "-")
	n, err := strconv.Atoi(lo)
	if err != nil {
		return -1
	}
	return n
}

// DerivedPayload gives the winws2 payload groups that the build makes for
// the profile when its payload is empty. A v2 target keeps an explicit
// payload only when it is not the same as this.
func (pr V01Profile) DerivedPayload() []string {
	pr.Payload = ""
	return pr.payloadGroups()
}
