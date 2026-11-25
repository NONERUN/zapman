package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/NONERUN/zapman/internal/spec"
)

// The strategy with the games and the strategy that it must be without them.
const (
	photonName = "general (ALT11 Photon)"
	alt11Name  = "general (ALT11)"
)

// The catalog.json data of the stock catalog.
const (
	catalogID    = "stock"
	catalogTitle = "Zapman"
)

// strategyPath gives the slash path of a converted strategy in the catalog.
// All v0.1 strategies are ports of Flowseal .bat files.
func strategyPath(name string) string {
	return string(spec.OriginFlowseal) + "/" + name + ".json"
}

// gameDef tells which Photon profiles make a game module.
type gameDef struct {
	id, title string
	order     int
	updater   string
	profiles  []int // indexes in photonName, 0-based
}

// games are in build order.
var games = []gameDef{
	{"sky", "Sky: Children of the Light", 10, "sky-log", []int{0, 1}},
	{"sker", "Sker Ritual, Hazard Levels", 20, "", []int{4}},
	{"photon", "Photon (UDP 5055-5058, 27000-27002)", 30, "", []int{5}},
}

// use is a distinct target, action or bundle with its use count.
type use[T any] struct {
	key  string
	v    T
	uses int
	name string // empty: the item stays inline
}

// bundleUse is a distinct list of refs of one kind.
type bundleUse struct {
	kind  string // hostlist, hostlistExclude, ipset, ipsetExclude
	refs  []string
	name  string
	uses  int
	users map[string]bool // file names of the rules that use it
}

type rule struct {
	t *use[spec.Target]
	a *use[spec.Action]
}

type outFile struct {
	name  string // strategy name or game id
	game  *gameDef
	rules []rule
}

// result is the output of convert.
type result struct {
	bundles []*bundleUse
	targets []*use[spec.Target]
	actions []*use[spec.Action]
	out     []*outFile
	files   map[string][]byte // slash path in the strategies folder
	notes   []string          // Photon check and other notes for the report
}

type converter struct {
	res       *result
	bundleKey map[string]*bundleUse
	targetKey map[string]*use[spec.Target]
	actionKey map[string]*use[spec.Action]
	tags      map[string]string // upstream tag by strategy name
}

// convert makes the v2 files from the v0.1 strategies. The tags are the
// upstream tags of the strategies. Each strategy gets rev 1 and each game
// gets version 1.
func convert(old map[string]*spec.V01Spec, names []string, listsDir string, tags map[string]string) (*result, error) {
	c := &converter{
		tags:      tags,
		res:       &result{files: map[string][]byte{}},
		bundleKey: map[string]*bundleUse{},
		targetKey: map[string]*use[spec.Target]{},
		actionKey: map[string]*use[spec.Action]{},
	}
	photon, alt11 := old[photonName], old[alt11Name]
	if photon == nil || alt11 == nil {
		return nil, fmt.Errorf("%q and %q must both exist", photonName, alt11Name)
	}
	notes, err := checkPhoton(photon, alt11)
	if err != nil {
		return nil, fmt.Errorf("%s is not %s with the game profiles: %w (nothing is written)", photonName, alt11Name, err)
	}
	c.res.notes = append(c.res.notes, notes...)

	for _, n := range names {
		if n == photonName {
			continue
		}
		f := &outFile{name: n}
		for i := range old[n].Profiles {
			r, err := c.rule(&old[n].Profiles[i], n+".json")
			if err != nil {
				return nil, fmt.Errorf("%s.json: profiles[%d]: %w", n, i, err)
			}
			f.rules = append(f.rules, r)
		}
		c.res.out = append(c.res.out, f)
	}
	for gi := range games {
		g := &games[gi]
		f := &outFile{name: g.id, game: g}
		for _, i := range g.profiles {
			r, err := c.rule(&photon.Profiles[i], path.Join(spec.GamesDir, g.id+".json"))
			if err != nil {
				return nil, fmt.Errorf("%s.json: profiles[%d]: %w", photonName, i, err)
			}
			f.rules = append(f.rules, r)
		}
		c.res.out = append(c.res.out, f)
	}
	nameTargets(c.res.targets)
	nameActions(c.res.actions)
	if err := c.render(listsDir); err != nil {
		return nil, err
	}
	return c.res, nil
}

// checkPhoton tells if photon is alt11 with the game profiles, field by
// field, and if its wf has the same ports as alt11 and the games.
func checkPhoton(photon, alt11 *spec.V01Spec) ([]string, error) {
	isGame := map[int]bool{}
	var gamePorts struct{ tcp, udp []string }
	for _, g := range games {
		for _, i := range g.profiles {
			if i >= len(photon.Profiles) {
				return nil, fmt.Errorf("no profile %d", i)
			}
			isGame[i] = true
			pr := photon.Profiles[i]
			gamePorts.tcp = append(gamePorts.tcp, splitPorts(pr.TCP)...)
			gamePorts.udp = append(gamePorts.udp, splitPorts(pr.UDP)...)
		}
	}
	var rest []spec.V01Profile
	for i, pr := range photon.Profiles {
		if !isGame[i] {
			rest = append(rest, pr)
		}
	}
	if len(rest) != len(alt11.Profiles) {
		return nil, fmt.Errorf("%d profiles without the games, %s has %d", len(rest), alt11Name, len(alt11.Profiles))
	}
	pt := reflect.TypeOf(spec.V01Profile{})
	for i := range rest {
		a, b := reflect.ValueOf(rest[i]), reflect.ValueOf(alt11.Profiles[i])
		for f := 0; f < pt.NumField(); f++ {
			if !reflect.DeepEqual(a.Field(f).Interface(), b.Field(f).Interface()) {
				return nil, fmt.Errorf("profile %d of the rest, field %s: %v and %v", i, pt.Field(f).Name, a.Field(f).Interface(), b.Field(f).Interface())
			}
		}
	}
	for _, w := range []struct {
		name          string
		photon, alt11 spec.StringList
		game          []string
	}{
		{"tcp", photon.WF.TCP, alt11.WF.TCP, gamePorts.tcp},
		{"udp", photon.WF.UDP, alt11.WF.UDP, gamePorts.udp},
	} {
		want := setOf(append(append([]string(nil), w.alt11...), w.game...))
		if got := setOf(w.photon); !reflect.DeepEqual(got, want) {
			return nil, fmt.Errorf("wf %s: %v, want the ports of %s and the games %v", w.name, w.photon, alt11Name, want)
		}
	}
	return []string{fmt.Sprintf("%s: %d profiles; without the game profiles %v the %d profiles are equal to %s in all %d fields; wf has the ports of %s and the games",
		photonName, len(photon.Profiles), sortedInts(isGame), len(rest), alt11Name, pt.NumField(), alt11Name)}, nil
}

// rule splits a v0.1 profile into a target and an action.
func (c *converter) rule(pr *spec.V01Profile, file string) (rule, error) {
	t := spec.Target{
		TCP:            v2Ports(pr.TCP, "$gf.Tcp"),
		UDP:            v2Ports(pr.UDP, "$gf.Udp"),
		L3:             pr.L3,
		AnyProtocol:    pr.AnyProtocol,
		Domains:        list(pr.HostlistDomains),
		ExcludeDomains: list(pr.HostlistExcludeDomains),
	}
	if pr.L7 != "" {
		t.L7 = strings.Split(pr.L7, ",")
	}
	if pr.Payload != "" {
		if d := pr.DerivedPayload(); len(d) != 1 || d[0] != pr.Payload {
			t.Payload = pr.Payload
		}
	}
	for _, k := range []struct {
		kind    string
		refs    spec.StringList
		exclude bool
	}{
		{"hostlist", pr.Hostlist, false}, {"ipset", pr.Ipset, false},
		{"hostlistExclude", pr.HostlistExclude, true}, {"ipsetExclude", pr.IpsetExclude, true},
	} {
		if len(k.refs) == 0 {
			continue
		}
		b, err := c.bundle(k.kind, k.refs, file)
		if err != nil {
			return rule{}, err
		}
		if k.exclude {
			t.Exclude = append(t.Exclude, b.name)
		} else {
			t.Lists = append(t.Lists, b.name)
		}
	}

	a := spec.Action{
		Desync:            list(pr.Desync),
		Repeats:           pr.Repeats,
		TTL:               pr.TTL,
		SplitPos:          pr.SplitPos,
		Seqovl:            pr.Seqovl,
		SeqovlPattern:     pr.SeqovlPattern,
		Fooling:           list(pr.Fooling),
		BadseqIncrement:   pr.BadseqIncrement,
		IPID:              pr.IPID,
		HostfakesplitMod:  pr.HostfakesplitMod,
		FakedsplitPattern: pr.FakedsplitPattern,
		TLSMod:            pr.TLSMod,
		LuaPayload:        pr.LuaPayload,
	}
	switch {
	case pr.Cutoff == "none":
		a.Cutoff = &spec.Cutoff{None: true}
	case pr.Cutoff != "":
		n, err := strconv.Atoi(strings.TrimPrefix(pr.Cutoff, "n"))
		if err != nil || !strings.HasPrefix(pr.Cutoff, "n") {
			return rule{}, fmt.Errorf("cutoff %q", pr.Cutoff)
		}
		a.Cutoff = &spec.Cutoff{N: n}
	}
	var f spec.Fakes
	set := false
	for _, s := range []struct {
		name string
		src  spec.FakeSlot
		dst  *[]string
	}{
		{"quic", pr.Fake.QUIC, &f.QUIC}, {"tls", pr.Fake.TLS, &f.TLS}, {"http", pr.Fake.HTTP, &f.HTTP},
		{"discord", pr.Fake.Discord, &f.Discord}, {"stun", pr.Fake.STUN, &f.STUN},
		{"unknown", pr.Fake.Unknown, &f.Unknown}, {"unknownUdp", pr.Fake.UnknownUDP, &f.UnknownUDP},
	} {
		if !s.src.Set {
			continue
		}
		if len(s.src.Refs) == 0 {
			return rule{}, fmt.Errorf("fake slot %s is set with no refs; v2 has no form for it", s.name)
		}
		*s.dst = list(s.src.Refs)
		set = true
	}
	if set {
		a.Fake = &f
	}
	return rule{t: addUse(&c.res.targets, c.targetKey, t), a: addUse(&c.res.actions, c.actionKey, a)}, nil
}

// bundle gives the bundle of the refs and counts the use.
func (c *converter) bundle(kind string, refs spec.StringList, file string) (*bundleUse, error) {
	key := kind + "\x00" + strings.Join(refs, "\x00")
	b := c.bundleKey[key]
	if b == nil {
		b = &bundleUse{kind: kind, refs: list(refs), name: bundleName(kind, refs), users: map[string]bool{}}
		for _, o := range c.res.bundles {
			if o.name == b.name {
				return nil, fmt.Errorf("two bundles get the name %q: %v and %v", b.name, o.refs, b.refs)
			}
		}
		c.bundleKey[key] = b
		c.res.bundles = append(c.res.bundles, b)
	}
	b.uses++
	b.users[file] = true
	return b, nil
}

// bundleName names a bundle by its first ref: list-general.txt -> general,
// ipset-all.txt -> ipset-all, list-exclude.txt -> exclude-hosts,
// ipset-exclude.txt -> exclude-ips.
func bundleName(kind string, refs spec.StringList) string {
	_, p, _ := strings.Cut(refs[0], ":")
	stem := strings.TrimSuffix(path.Base(p), path.Ext(p))
	stem = strings.TrimSuffix(stem, "-user")
	switch {
	case stem == "list-exclude":
		return "exclude-hosts"
	case stem == "ipset-exclude":
		return "exclude-ips"
	case kind == "hostlist":
		return sanitize(strings.TrimPrefix(stem, "list-"))
	}
	return sanitize(stem)
}

func addUse[T any](all *[]*use[T], byKey map[string]*use[T], v T) *use[T] {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	u := byKey[string(b)]
	if u == nil {
		u = &use[T]{key: string(b), v: v}
		byKey[u.key] = u
		*all = append(*all, u)
	}
	u.uses++
	return u
}

// nameTargets names the targets that are used minTargetUses or more times.
func nameTargets(ts []*use[spec.Target]) {
	var named []*use[spec.Target]
	for _, t := range ts {
		if t.uses >= minTargetUses {
			t.name = targetName(&t.v)
			named = append(named, t)
		}
	}
	unique(named, func(u *use[spec.Target]) *string { return &u.name })
}

// targetName gives a short name from the ports, the l7 and the lists.
func targetName(t *spec.Target) string {
	if len(t.UDP) == 0 && len(t.TCP) == 1 && t.TCP[0] == spec.GameFilterPort {
		return "gf-tcp"
	}
	if len(t.TCP) == 0 && len(t.UDP) == 1 && t.UDP[0] == spec.GameFilterPort {
		return "gf-udp"
	}
	tag := ""
	for _, l := range t.Lists {
		switch l {
		case "general":
			tag = "general"
		case "ipset-all":
			tag = "ipset"
		}
	}
	if tag == "" && len(t.Lists) > 0 {
		tag = t.Lists[0]
	}
	has := func(n string) bool {
		for _, x := range t.L7 {
			if x == n {
				return true
			}
		}
		return false
	}
	switch {
	case has("quic"):
		return joinName("quic", tag)
	case has("discord") || has("stun"):
		return "discord-voice"
	case len(t.Domains) > 0:
		return sanitize(t.Domains[0])
	case tag == "google":
		return "google"
	case has("tls") && has("http"):
		return joinName("tls", tag)
	case has("tls"):
		return joinName("tls-only", tag)
	}
	if len(t.TCP) > 0 {
		return sanitize("tcp-" + strings.Join(t.TCP, "-"))
	}
	return sanitize("udp-" + strings.Join(t.UDP, "-"))
}

// Use counts that make a named item. A target or an action with fewer uses
// stays inline in the rule.
const (
	minTargetUses = 2
	minActionUses = 3
)

// nameActions names the actions that are used minActionUses or more times. The name
// has the attacks, the seqovl, the pattern and fake domains, and the
// repeats of a fake-only action. More details are added only where two
// actions get the same name.
func nameActions(as []*use[spec.Action]) {
	var named []*use[spec.Action]
	for _, a := range as {
		if a.uses >= minActionUses {
			a.name = actionBase(&a.v)
			named = append(named, a)
		}
	}
	for lvl := 0; lvl < actionDetailLevels; lvl++ {
		groups := map[string][]*use[spec.Action]{}
		for _, a := range named {
			groups[a.name] = append(groups[a.name], a)
		}
		for _, g := range groups {
			if len(g) < 2 {
				continue
			}
			tok := make([]string, len(g))
			differ := false
			for i, a := range g {
				tok[i] = actionDetails(&a.v)[lvl]
				if tok[i] != tok[0] {
					differ = true
				}
			}
			if !differ {
				continue
			}
			for i, a := range g {
				if tok[i] != "" {
					a.name += "-" + tok[i]
				}
			}
		}
	}
	for _, a := range named {
		a.name = dedupeWords(a.name)
	}
	unique(named, func(u *use[spec.Action]) *string { return &u.name })
}

// dedupeWords removes a word of a name that is also before it:
// fake-multisplit-664-sferum-tls-stun2-sferum -> ...-sferum-tls-stun2.
func dedupeWords(name string) string {
	seen := map[string]bool{}
	var out []string
	for _, w := range strings.Split(name, "-") {
		if !seen[w] {
			seen[w] = true
			out = append(out, w)
		}
	}
	return strings.Join(out, "-")
}

func fakeOnly(a *spec.Action) bool { return len(a.Desync) == 1 && a.Desync[0] == "fake" }

func actionBase(a *spec.Action) string {
	toks := append([]string(nil), a.Desync...)
	if a.Seqovl != nil {
		toks = append(toks, strconv.Itoa(*a.Seqovl))
	}
	if a.SeqovlPattern != "" {
		toks = append(toks, refDomain(a.SeqovlPattern))
	}
	if a.Fake != nil {
		for _, s := range []struct {
			pre  string
			refs []string
		}{
			{"quic-", a.Fake.QUIC}, {"", a.Fake.TLS}, {"", a.Fake.Discord}, {"", a.Fake.STUN},
			{"", a.Fake.UnknownUDP}, {"", a.Fake.Unknown}, {"", a.Fake.HTTP},
		} {
			if len(s.refs) > 0 {
				toks = append(toks, s.pre+refDomain(s.refs[len(s.refs)-1]))
				break
			}
		}
	}
	if fakeOnly(a) && a.Repeats != nil {
		toks = append(toks, strconv.Itoa(*a.Repeats))
	}
	seen := map[string]bool{}
	var out []string
	for _, t := range toks {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return strings.Join(out, "-")
}

const actionDetailLevels = 18

// actionDetails gives the name parts that can separate two actions, in the
// order of use.
func actionDetails(a *spec.Action) []string {
	d := make([]string, actionDetailLevels)
	if !fakeOnly(a) && a.Repeats != nil {
		d[0] = "r" + strconv.Itoa(*a.Repeats)
	}
	if a.Cutoff != nil {
		if a.Cutoff.None {
			d[1] = "nocutoff"
		} else {
			d[1] = a.Cutoff.String()
		}
	}
	if a.TTL != nil {
		d[2] = "ttl" + strconv.Itoa(*a.TTL)
	}
	if len(a.Fooling) > 0 {
		d[3] = strings.ReplaceAll(strings.Join(a.Fooling, "-"), "md5sig", "md5")
	}
	if a.BadseqIncrement != nil {
		d[4] = "inc" + strconv.Itoa(*a.BadseqIncrement)
	}
	if a.SplitPos != "" {
		d[5] = sanitize("pos-" + a.SplitPos)
	}
	if a.TLSMod != "" {
		d[6] = sanitize(strings.ReplaceAll(a.TLSMod, "none", "nomod"))
	}
	if a.IPID != "" {
		d[7] = sanitize("ipid-" + a.IPID)
	}
	if a.HostfakesplitMod != "" {
		d[8] = sanitize(a.HostfakesplitMod)
	}
	if a.FakedsplitPattern != "" {
		d[9] = "fsp-" + refDomain(a.FakedsplitPattern)
	}
	if a.LuaPayload != "" {
		d[10] = sanitize("lua-" + a.LuaPayload)
	}
	if f := a.Fake; f != nil {
		for i, s := range []struct {
			name string
			refs []string
		}{
			{"quic", f.QUIC}, {"tls", f.TLS}, {"http", f.HTTP}, {"discord", f.Discord},
			{"stun", f.STUN}, {"unknown", f.Unknown}, {"udp", f.UnknownUDP},
		} {
			if len(s.refs) == 0 {
				continue
			}
			p := s.name
			for _, r := range s.refs {
				p += "-" + refDomain(r)
			}
			d[11+i] = p
		}
	}
	return d
}

// refDomain gives a short name of a fake or pattern ref:
// bin:tls_clienthello_www_google_com.bin -> google, ! -> stock.
func refDomain(ref string) string {
	if ref == "!" {
		return "stock"
	}
	if strings.HasPrefix(ref, "0x") {
		return "hex"
	}
	_, p, _ := strings.Cut(ref, ":")
	s := strings.TrimSuffix(path.Base(p), path.Ext(p))
	switch s {
	case "ACTIVE_GAME_UDP":
		return "game"
	case "ACTIVE_DISCORD_UDP":
		return "discord"
	}
	s = strings.ToLower(s)
	s = strings.TrimPrefix(s, "tls_clienthello_")
	s = strings.TrimPrefix(s, "quic_initial_")
	s = strings.TrimPrefix(s, "www_")
	s, _, _ = strings.Cut(s, "_")
	return sanitize(s)
}

// unique adds -2, -3 and so on to names that are the same.
func unique[T any](items []T, name func(T) *string) {
	seen := map[string]int{}
	for _, it := range items {
		seen[*name(it)]++
	}
	count := map[string]int{}
	for _, it := range items {
		n := name(it)
		if seen[*n] < 2 {
			continue
		}
		count[*n]++
		if count[*n] > 1 {
			*n += "-" + strconv.Itoa(count[*n])
		}
	}
}

// sanitize gives a name with a-z, 0-9 and single - only.
func sanitize(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

func joinName(parts ...string) string {
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "-")
}

// render makes the JSON files.
func (c *converter) render(listsDir string) error {
	res := c.res
	res.files[spec.CatalogFile] = pretty(mustJSON(spec.CatalogInfo{Schema: "./schema/catalog.json", ID: catalogID, Title: catalogTitle, Spec: spec.Version}))
	// A bundle that only one game uses goes into that game file.
	local := map[string][]*bundleUse{}
	var global []*bundleUse
	for _, b := range res.bundles {
		if len(b.users) == 1 {
			if u := firstKey(b.users); strings.HasPrefix(u, spec.GamesDir+"/") {
				local[u] = append(local[u], b)
				continue
			}
		}
		global = append(global, b)
	}
	var obj []string
	for _, b := range global {
		obj = append(obj, mustJSON(b.name)+":"+mustJSON(bundleOf(b)))
	}
	res.files[spec.ListsFile] = pretty(namedJSON("./schema/lists.json", obj))
	obj = nil
	for _, t := range res.targets {
		if t.name != "" {
			obj = append(obj, mustJSON(t.name)+":"+t.key)
		}
	}
	res.files[spec.TargetsFile] = pretty(namedJSON("./schema/targets.json", obj))
	obj = nil
	for _, a := range res.actions {
		if a.name != "" {
			obj = append(obj, mustJSON(a.name)+":"+a.key)
		}
	}
	res.files[spec.ActionsFile] = pretty(namedJSON("./schema/actions.json", obj))

	for _, f := range res.out {
		rules := make([]spec.Rule, len(f.rules))
		for i, r := range f.rules {
			if r.t.name != "" {
				rules[i].Target.Name = r.t.name
			} else {
				v := r.t.v
				rules[i].Target.Inline = &v
			}
			if r.a.name != "" {
				rules[i].Action.Name = r.a.name
			} else {
				v := r.a.v
				rules[i].Action.Inline = &v
			}
		}
		if f.game == nil {
			tag := c.tags[f.name]
			if tag == "" {
				return fmt.Errorf("%s: no upstream tag", f.name)
			}
			sf := spec.StrategyFile{Schema: "../schema/strategy.json", Spec: spec.Version, Upstream: tag, Rev: 1, Rules: rules}
			res.files[strategyPath(f.name)] = pretty(mustJSON(sf))
			continue
		}
		file := path.Join(spec.GamesDir, f.name+".json")
		gf := spec.GameFile{Schema: "../schema/game.json", ID: f.game.id, Title: f.game.title, Version: 1, Order: f.game.order, Updater: f.game.updater, Rules: rules}
		for _, b := range local[file] {
			if gf.Lists == nil {
				gf.Lists = map[string]spec.GameBundle{}
			}
			gb := spec.GameBundle{Bundle: bundleOf(b)}
			if b.kind == "ipset" || b.kind == "hostlist" {
				_, p, _ := strings.Cut(b.refs[0], ":")
				seed := strings.TrimSuffix(p, path.Ext(p)) + ".default" + path.Ext(p)
				if _, err := os.Stat(filepath.Join(listsDir, seed)); err == nil {
					gb.Seed = "lists:" + seed
				}
			}
			gf.Lists[b.name] = gb
		}
		res.files[file] = pretty(mustJSON(gf))
	}
	return nil
}

func firstKey(m map[string]bool) string {
	for k := range m {
		return k
	}
	return ""
}

func bundleOf(b *bundleUse) spec.Bundle {
	var out spec.Bundle
	switch b.kind {
	case "hostlist":
		out.Hostlist = b.refs
	case "hostlistExclude":
		out.HostlistExclude = b.refs
	case "ipset":
		out.Ipset = b.refs
	case "ipsetExclude":
		out.IpsetExclude = b.refs
	}
	return out
}

func namedJSON(schema string, items []string) string {
	return "{" + strings.Join(append([]string{`"$schema":` + mustJSON(schema)}, items...), ",") + "}"
}

func mustJSON(v any) string {
	b, err := marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// report writes the names and the use counts.
func (r *result) report(w io.Writer) {
	for _, n := range r.notes {
		fmt.Fprintln(w, n)
	}
	fmt.Fprintln(w, "bundles (name: uses, refs):")
	for _, b := range r.bundles {
		fmt.Fprintf(w, "  %s: %d, %s %v\n", b.name, b.uses, b.kind, b.refs)
	}
	inline := 0
	fmt.Fprintln(w, "targets (name: uses):")
	for _, t := range r.targets {
		if t.name == "" {
			inline++
			continue
		}
		fmt.Fprintf(w, "  %s: %d\n", t.name, t.uses)
	}
	fmt.Fprintf(w, "  inline: %d\n", inline)
	inline = 0
	fmt.Fprintln(w, "actions (name: uses):")
	for _, a := range r.actions {
		if a.name == "" {
			inline++
			continue
		}
		fmt.Fprintf(w, "  %s: %d\n", a.name, a.uses)
	}
	fmt.Fprintf(w, "  inline: %d\n", inline)
}

func v2Ports(s, gfToken string) []string {
	out := splitPorts(s)
	for i, p := range out {
		if p == gfToken {
			out[i] = spec.GameFilterPort
		}
	}
	return out
}

func splitPorts(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return strings.Split(s, ",")
}

func list(l spec.StringList) []string {
	if len(l) == 0 {
		return nil
	}
	return append([]string(nil), l...)
}

func setOf(items []string) map[string]bool {
	m := map[string]bool{}
	for _, it := range items {
		m[it] = true
	}
	return m
}

func sortedInts(m map[int]bool) []int {
	var out []int
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}
