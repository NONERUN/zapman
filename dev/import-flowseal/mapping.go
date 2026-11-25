package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/NONERUN/zapman/dev/internal/jsonfmt"
	"github.com/NONERUN/zapman/internal/spec"
)

// mapper makes v2 rules from the profiles of a winws argv. It is the
// inverse of the winws build of internal/spec.
type mapper struct {
	cat     *spec.Catalog
	include []bundle // bundles with hostlist and ipset refs
	exclude []bundle // bundles with hostlistExclude and ipsetExclude refs
}

// bundle is a list bundle with its refs in the normal form of the argv
// paths, per kind: [hostlist, ipset] or [hostlistExclude, ipsetExclude].
type bundle struct {
	name string
	refs [2][]string
}

func newMapper(cat *spec.Catalog) *mapper {
	m := &mapper{cat: cat}
	for _, name := range sortedKeys(cat.Lists) {
		b := cat.Lists[name]
		inc := [2][]string{normRefs(b.Hostlist), normRefs(b.Ipset)}
		exc := [2][]string{normRefs(b.HostlistExclude), normRefs(b.IpsetExclude)}
		switch {
		case len(exc[0])+len(exc[1]) == 0:
			m.include = append(m.include, bundle{name, inc})
		case len(inc[0])+len(inc[1]) == 0:
			m.exclude = append(m.exclude, bundle{name, exc})
		}
	}
	return m
}

// normRef gives the argv path of a list ref in the normal form. Flowseal
// keeps the user lists in lists/ too: lists: and user: give lists/.
func normRef(ref string) string {
	for _, p := range []prefix{{spec.PrefixLists, "lists/"}, {spec.PrefixUser, "lists/"}, {spec.PrefixBin, "bin/"}} {
		if strings.HasPrefix(ref, p.from) {
			return p.to + ref[len(p.from):]
		}
	}
	return ref
}

func normRefs(refs []string) []string {
	out := make([]string, len(refs))
	for i, r := range refs {
		out[i] = normRef(r)
	}
	return out
}

// fakeSlots maps the fake flags to the fake slots.
var fakeSlots = map[string]func(*spec.Fakes) *[]string{
	"--dpi-desync-fake-quic":        func(f *spec.Fakes) *[]string { return &f.QUIC },
	"--dpi-desync-fake-tls":         func(f *spec.Fakes) *[]string { return &f.TLS },
	"--dpi-desync-fake-http":        func(f *spec.Fakes) *[]string { return &f.HTTP },
	"--dpi-desync-fake-discord":     func(f *spec.Fakes) *[]string { return &f.Discord },
	"--dpi-desync-fake-stun":        func(f *spec.Fakes) *[]string { return &f.STUN },
	"--dpi-desync-fake-unknown":     func(f *spec.Fakes) *[]string { return &f.Unknown },
	"--dpi-desync-fake-unknown-udp": func(f *spec.Fakes) *[]string { return &f.UnknownUDP },
}

// listFlags are the flags with list paths. They can be in a profile more
// than one time.
var listFlags = map[string]int{
	"--hostlist": 0, "--ipset": 1, "--hostlist-exclude": 2, "--ipset-exclude": 3,
}

// rules gives the rules of the profiles.
func (m *mapper) rules(a *argv) ([]spec.Rule, error) {
	out := make([]spec.Rule, 0, len(a.Groups))
	for i, g := range a.Groups {
		r, err := m.rule(g)
		if err != nil {
			return nil, fmt.Errorf("profile %d: %w", i+1, err)
		}
		out = append(out, r)
	}
	return out, nil
}

// rule gives the rule of one profile. A flag that the build does not make
// or a value that the v2 format cannot hold is an error.
func (m *mapper) rule(g []arg) (spec.Rule, error) {
	var t spec.Target
	var a spec.Action
	var f spec.Fakes
	var lists [4][]string
	seen := map[string]bool{}
	for _, x := range g {
		_, multi := listFlags[x.Flag]
		if fakeSlots[x.Flag] != nil {
			multi = true
		}
		if !multi {
			if seen[x.Flag] {
				return spec.Rule{}, fmt.Errorf("%s is two times", x.Flag)
			}
			seen[x.Flag] = true
		}
		if !x.HasValue {
			return spec.Rule{}, fmt.Errorf("%s has no value", x.Flag)
		}
		if err := m.apply(x, &t, &a, &f, &lists); err != nil {
			return spec.Rule{}, fmt.Errorf("%s: %w", x, err)
		}
	}
	if mustJSON(f) != mustJSON(spec.Fakes{}) {
		a.Fake = &f
	}
	var ok bool
	if t.Lists, ok = decompose([2][]string{lists[0], lists[1]}, m.include); !ok {
		return spec.Rule{}, fmt.Errorf("no bundles of %s give --hostlist %s and --ipset %s", spec.ListsFile, listText(lists[0]), listText(lists[1]))
	}
	if t.Exclude, ok = decompose([2][]string{lists[2], lists[3]}, m.exclude); !ok {
		return spec.Rule{}, fmt.Errorf("no bundles of %s give --hostlist-exclude %s and --ipset-exclude %s", spec.ListsFile, listText(lists[2]), listText(lists[3]))
	}
	return spec.Rule{Target: m.targetRef(t), Action: m.actionRef(a)}, nil
}

// apply puts one arg into the target or the action.
func (m *mapper) apply(x arg, t *spec.Target, a *spec.Action, f *spec.Fakes, lists *[4][]string) error {
	v := x.Value
	var err error
	switch x.Flag {
	case "--filter-l3":
		t.L3 = v
	case "--filter-tcp":
		t.TCP, err = ports(v, gfTCPVar)
	case "--filter-udp":
		t.UDP, err = ports(v, gfUDPVar)
	case "--filter-l7":
		t.L7 = strings.Split(v, ",")
	case "--hostlist", "--ipset", "--hostlist-exclude", "--ipset-exclude":
		lists[listFlags[x.Flag]] = append(lists[listFlags[x.Flag]], v)
	case "--hostlist-domains":
		t.Domains = strings.Split(v, ",")
	case "--hostlist-exclude-domains":
		t.ExcludeDomains = strings.Split(v, ",")
	case "--dpi-desync-any-protocol":
		if v != "1" {
			return fmt.Errorf("only 1 is supported")
		}
		t.AnyProtocol = true
	case "--ip-id":
		a.IPID = v
	case "--dpi-desync":
		a.Desync = strings.Split(v, ",")
	case "--dpi-desync-repeats":
		a.Repeats, err = number(v)
	case "--dpi-desync-ttl":
		a.TTL, err = number(v)
	case "--dpi-desync-cutoff":
		n, ok := strings.CutPrefix(v, "n")
		c, e := number(n)
		if !ok || e != nil {
			return fmt.Errorf("only nN is supported")
		}
		a.Cutoff = &spec.Cutoff{N: *c}
	case "--dpi-desync-split-seqovl":
		a.Seqovl, err = number(v)
	case "--dpi-desync-split-pos":
		a.SplitPos = v
	case "--dpi-desync-fooling":
		a.Fooling = strings.Split(v, ",")
	case "--dpi-desync-badseq-increment":
		a.BadseqIncrement, err = number(v)
	case "--dpi-desync-split-seqovl-pattern":
		a.SeqovlPattern, err = m.fileRef(v)
	case "--dpi-desync-hostfakesplit-mod":
		a.HostfakesplitMod = v
	case "--dpi-desync-fakedsplit-pattern":
		a.FakedsplitPattern, err = m.fakeRef(v)
	case "--dpi-desync-fake-tls-mod":
		a.TLSMod = v
	default:
		get := fakeSlots[x.Flag]
		if get == nil {
			return fmt.Errorf("unknown flag %s", x.Flag)
		}
		var ref string
		if ref, err = m.fakeRef(v); err == nil {
			s := get(f)
			*s = append(*s, ref)
		}
	}
	return err
}

// ports gives the port items of a filter value. gfVar is the Game Filter
// variable of the protocol; it gives $gameFilter.
func ports(v, gfVar string) ([]string, error) {
	items := strings.Split(v, ",")
	for i, it := range items {
		switch {
		case it == gfVar:
			items[i] = spec.GameFilterPort
		case strings.Contains(it, "%"):
			return nil, fmt.Errorf("%s is not the Game Filter variable of this protocol", it)
		}
	}
	return items, nil
}

func number(v string) (*int, error) {
	n, err := strconv.Atoi(v)
	if err != nil {
		return nil, fmt.Errorf("%q is not a number", v)
	}
	return &n, nil
}

// fakeRef gives the ref of a fake value: "!" and 0x<hex> stay, a file
// gives a ref (see fileRef).
func (m *mapper) fakeRef(v string) (string, error) {
	if v == "!" || strings.HasPrefix(v, "0x") {
		return v, nil
	}
	return m.fileRef(v)
}

// fileRef gives the ref of a bin/ file: bin/ACTIVE_X.bin gives active:X,
// another file gives cat:fakes/<file>. The active name must have a default
// in catalog.json, and the file must be in the fakes folder of the catalog.
func (m *mapper) fileRef(v string) (string, error) {
	name, ok := strings.CutPrefix(v, "bin/")
	if !ok || name == "" || strings.Contains(name, "/") {
		return "", fmt.Errorf("the file is not in bin\\")
	}
	if n, ok := strings.CutPrefix(name, "ACTIVE_"); ok && strings.HasSuffix(n, ".bin") {
		n = strings.TrimSuffix(n, ".bin")
		if _, ok := m.cat.Active[n]; !ok {
			return "", fmt.Errorf("active fake %s has no default in %s", n, spec.CatalogFile)
		}
		return spec.PrefixActive + n, nil
	}
	if st, err := os.Stat(filepath.Join(m.cat.Dir, spec.FakesDir, name)); err != nil || st.IsDir() {
		return "", fmt.Errorf("%s is not in %s", name, filepath.Join(m.cat.Dir, spec.FakesDir))
	}
	return spec.PrefixCat + spec.FakesDir + "/" + name, nil
}

// decompose gives the bundles that give the refs of both kinds in the same
// order. It tries the bundles in name order and gives the first result. No
// refs give no bundles.
func decompose(want [2][]string, bundles []bundle) ([]string, bool) {
	if len(want[0]) == 0 && len(want[1]) == 0 {
		return nil, true
	}
	for _, b := range bundles {
		if !hasPrefix(want[0], b.refs[0]) || !hasPrefix(want[1], b.refs[1]) {
			continue
		}
		rest, ok := decompose([2][]string{want[0][len(b.refs[0]):], want[1][len(b.refs[1]):]}, bundles)
		if ok {
			return append([]string{b.name}, rest...), true
		}
	}
	return nil, false
}

// hasPrefix tells if p is a prefix of s, with no case (Windows paths).
func hasPrefix(s, p []string) bool {
	if len(p) > len(s) {
		return false
	}
	for i := range p {
		if !strings.EqualFold(s[i], p[i]) {
			return false
		}
	}
	return true
}

// targetRef gives the name of the target of targets.json with the same
// filter, else the inline target.
func (m *mapper) targetRef(t spec.Target) spec.Ref[spec.Target] {
	want := m.targetKey(t)
	for _, name := range sortedKeys(m.cat.Targets) {
		if m.targetKey(m.cat.Targets[name]) == want {
			return spec.Ref[spec.Target]{Name: name}
		}
	}
	return spec.Ref[spec.Target]{Inline: &t}
}

// targetKey gives a text that is the same for two targets with the same
// filter: the bundles are expanded to their refs per kind.
func (m *mapper) targetKey(t spec.Target) string {
	var refs [4][]string
	for _, n := range append(append([]string(nil), t.Lists...), t.Exclude...) {
		b := m.cat.Lists[n]
		refs[0] = append(refs[0], b.Hostlist...)
		refs[1] = append(refs[1], b.Ipset...)
		refs[2] = append(refs[2], b.HostlistExclude...)
		refs[3] = append(refs[3], b.IpsetExclude...)
	}
	t.Lists, t.Exclude = nil, nil
	return mustJSON(struct {
		T spec.Target
		R [4][]string
	}{t, refs})
}

// actionRef gives the name of the action of actions.json that is the same,
// else the inline action.
func (m *mapper) actionRef(a spec.Action) spec.Ref[spec.Action] {
	want := mustJSON(a)
	for _, name := range sortedKeys(m.cat.Actions) {
		if mustJSON(m.cat.Actions[name]) == want {
			return spec.Ref[spec.Action]{Name: name}
		}
	}
	return spec.Ref[spec.Action]{Inline: &a}
}

func mustJSON(v any) string {
	b, err := jsonfmt.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func sortedKeys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
