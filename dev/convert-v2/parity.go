package main

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/NONERUN/zapman/internal/spec"
)

// parityPaths are fixed folders for the argv compare.
var parityPaths = spec.Paths{Bin: `C:\zapman\bin`, Lists: `C:\zapman\lists`, User: `C:\zapman\user`}

// parityGF are the Game Filter cases of the golden test.
var parityGF = []struct {
	name string
	gf   spec.GameFilter
}{
	{"gf-off", spec.GameFilter{TCP: "12", UDP: "12"}},
	{"gf-all", spec.GameFilter{TCP: "1024-65535", UDP: "1024-65535"}},
}

// checkParity loads the v2 files of dir and compares the argv with the
// v0.1 argv. Each strategy but Photon must give the same bytes. Photon
// must be ALT11 with all games: only the place of the game profiles and
// the order of the wf ports and the blobs can be different.
func checkParity(dir, bin string, old map[string]*spec.V01Spec, names []string, res *result) error {
	cat, err := spec.LoadCatalog(dir, bin)
	if err != nil {
		return err
	}
	if got := len(cat.Games); got != len(games) {
		return fmt.Errorf("%d games, want %d", got, len(games))
	}
	same, wfSame := 0, 0
	for _, n := range names {
		if n == photonName {
			continue
		}
		st, err := cat.Strategy(n)
		if err != nil {
			return err
		}
		tcp, udp := spec.WFPorts(st, nil)
		if reflect.DeepEqual(tcp, []string(old[n].WF.TCP)) && reflect.DeepEqual(udp, []string(old[n].WF.UDP)) {
			wfSame++
		} else {
			res.notes = append(res.notes, fmt.Sprintf("wf %s: v0.1 %v %v, v2 %v %v", n, old[n].WF.TCP, old[n].WF.UDP, tcp, udp))
		}
		for _, eng := range []spec.Engine{spec.Winws, spec.Winws2} {
			for _, c := range parityGF {
				a, err := spec.BuildV01(old[n], eng, parityPaths, c.gf)
				if err != nil {
					return fmt.Errorf("%s v0.1 %s %s: %w", n, eng, c.name, err)
				}
				b, err := spec.Build(st, nil, eng, parityPaths, c.gf)
				if err != nil {
					return fmt.Errorf("%s v2 %s %s: %w", n, eng, c.name, err)
				}
				if d := lineDiff(a, b); d != "" {
					return fmt.Errorf("%s %s %s: argv differs: %s", n, eng, c.name, d)
				}
				same++
			}
		}
	}
	res.notes = append(res.notes, fmt.Sprintf("parity: %d argv (strategies x engines x Game Filter) are byte-identical; wf of %d of %d strategies has the same items in the same order",
		same, wfSame, len(names)-1))

	st, err := cat.Strategy(alt11Name)
	if err != nil {
		return err
	}
	for _, eng := range []spec.Engine{spec.Winws, spec.Winws2} {
		for _, c := range parityGF {
			a, err := spec.BuildV01(old[photonName], eng, parityPaths, c.gf)
			if err != nil {
				return err
			}
			b, err := spec.Build(st, cat.Games, eng, parityPaths, c.gf)
			if err != nil {
				return err
			}
			note, err := comparePhoton(a, b, len(old[photonName].Profiles))
			if err != nil {
				return fmt.Errorf("%s and %s + games, %s %s: %w", photonName, alt11Name, eng, c.name, err)
			}
			res.notes = append(res.notes, fmt.Sprintf("%s vs %s + %s, %s %s: %s", photonName, alt11Name, gameIDs(), eng, c.name, note))
		}
	}
	return nil
}

func gameIDs() string {
	var ids []string
	for _, g := range games {
		ids = append(ids, g.id)
	}
	return "[" + strings.Join(ids, ", ") + "]"
}

// comparePhoton compares the Photon argv a with the ALT11 + games argv b.
// The last nProfiles lines are the profiles.
func comparePhoton(a, b []string, nProfiles int) (string, error) {
	if len(a) != len(b) {
		return "", fmt.Errorf("%d lines, want %d", len(b), len(a))
	}
	h := len(a) - nProfiles
	var notes []string
	// wf line: the same port items.
	wa, wb := wfItems(a[0]), wfItems(b[0])
	if !reflect.DeepEqual(wa, wb) {
		return "", fmt.Errorf("wf ports: %v, want %v", wb, wa)
	}
	if a[0] == b[0] {
		notes = append(notes, "wf same")
	} else {
		notes = append(notes, fmt.Sprintf("wf same ports, other order (%q -> %q)", a[0], b[0]))
	}
	// Other head lines (winws2 Lua and blobs): the same words.
	for i := 1; i < h; i++ {
		if a[i] == b[i] {
			continue
		}
		if !reflect.DeepEqual(sortedWords(a[i]), sortedWords(b[i])) {
			return "", fmt.Errorf("line %d: %q, want %q", i+1, b[i], a[i])
		}
		notes = append(notes, fmt.Sprintf("line %d same words, other order", i+1))
	}
	// Profiles: the game profiles go first, the rest keeps its order.
	isGame := map[int]bool{}
	var order []int
	for _, g := range games {
		for _, i := range g.profiles {
			isGame[i] = true
			order = append(order, i)
		}
	}
	for i := range nProfiles {
		if !isGame[i] {
			order = append(order, i)
		}
	}
	moved := 0
	for j, i := range order {
		if b[h+j] != a[h+i] {
			return "", fmt.Errorf("profile line %d is not Photon profile %d:\n got: %s\nwant: %s", j+1, i+1, b[h+j], a[h+i])
		}
		if i != j {
			moved++
		}
	}
	notes = append(notes, fmt.Sprintf("profile lines equal; Photon profile order %v (1-based: %s), %d lines at another place", order, oneBased(order), moved))
	return strings.Join(notes, "; "), nil
}

func oneBased(order []int) string {
	parts := make([]string, len(order))
	for i, v := range order {
		parts[i] = fmt.Sprint(v + 1)
	}
	return strings.Join(parts, ",")
}

// wfItems gives the sorted port items of a wf line, by option.
func wfItems(line string) map[string][]string {
	out := map[string][]string{}
	for _, w := range strings.Fields(line) {
		k, v, ok := strings.Cut(w, "=")
		if !ok {
			continue
		}
		items := strings.Split(v, ",")
		sort.Strings(items)
		out[k] = items
	}
	return out
}

func sortedWords(s string) []string {
	f := strings.Fields(s)
	sort.Strings(f)
	return f
}

// lineDiff tells the first line that differs, or "".
func lineDiff(a, b []string) string {
	for i := 0; i < len(a) || i < len(b); i++ {
		var x, y string
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			return fmt.Sprintf("line %d\nv0.1: %s\n  v2: %s", i+1, x, y)
		}
	}
	return ""
}
