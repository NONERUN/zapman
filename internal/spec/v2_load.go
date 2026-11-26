package spec

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// The files of a catalog folder.
const (
	CatalogFile = "catalog.json"
	ListsFile   = "lists.json"
	TargetsFile = "targets.json"
	ActionsFile = "actions.json"
	GamesDir    = "games"
)

// Origin tells where a strategy comes from. It is also the name of the
// folder of the strategy in the catalog.
type Origin string

// The origins of strategies.
const (
	// OriginFlowseal is a port of a .bat file of Flowseal/zapret-discord-youtube.
	OriginFlowseal Origin = "flowseal"
	// OriginZapman is a strategy of zapman.
	OriginZapman Origin = "zapman"
)

// Origins are the strategy folders of a catalog, in search order.
var Origins = []Origin{OriginFlowseal, OriginZapman}

// Title gives the origin name for people: Flowseal or Zapman.
func (o Origin) Title() string {
	switch o {
	case OriginFlowseal:
		return "Flowseal"
	case OriginZapman:
		return "Zapman"
	}
	return string(o)
}

// gameUpdaters are the known values of GameFile.Updater.
var gameUpdaters = map[string]bool{"sky-log": true}

// Catalog is a loaded catalog folder: the catalog.json data, the named
// bundles, targets and actions, and the game modules. Strategies refer to
// these names.
type Catalog struct {
	CatalogInfo
	Dir     string
	Bin     string
	Lists   map[string]Bundle
	Targets map[string]Target
	Actions map[string]Action
	// Games are in build order: by Order, then by ID.
	Games []*Game
}

// Strategy is a loaded strategy file. Its rules refer to the catalog.
type Strategy struct {
	StrategyFile
	Name   string
	origin Origin
	file   string
	rules  []resolvedRule
	// active are the defaults of the active fakes (CatalogInfo.Active).
	active map[string]string
}

// Origin gives the origin of the strategy: the name of its folder.
func (s *Strategy) Origin() Origin { return s.origin }

// Game is a loaded game module.
type Game struct {
	GameFile
	file  string
	rules []resolvedRule
	// values are the option values that rules use.
	values map[string]bool
	// variants are the rules for each value set, by optionText.
	variants map[string][]resolvedRule
}

// resolvedRule is a rule as a v0.1 profile, with its ports for the wf line
// and the text that errors use to name the rule.
type resolvedRule struct {
	profile  V01Profile
	tcp, udp []string
	origin   string
}

// LoadCatalog reads catalog.json, lists.json, targets.json, actions.json and
// games/*.json of the catalog folder dir. Each cat: ref must be a file in
// dir. When bin is not empty, each bin: ref must be a file in bin.
func LoadCatalog(dir, bin string) (*Catalog, error) {
	c := &Catalog{Dir: dir, Bin: bin}
	if err := c.loadInfo(); err != nil {
		return nil, err
	}
	if err := checkRootFiles(dir); err != nil {
		return nil, err
	}
	var err error
	if c.Lists, err = loadNamed[Bundle](filepath.Join(dir, ListsFile)); err != nil {
		return nil, err
	}
	if c.Targets, err = loadNamed[Target](filepath.Join(dir, TargetsFile)); err != nil {
		return nil, err
	}
	if c.Actions, err = loadNamed[Action](filepath.Join(dir, ActionsFile)); err != nil {
		return nil, err
	}
	for _, name := range sortedKeys(c.Lists) {
		b := c.Lists[name]
		if err := c.checkBundle(&b); err != nil {
			return nil, fmt.Errorf("%s: %q: %w", ListsFile, name, err)
		}
	}
	for _, name := range sortedKeys(c.Targets) {
		t := c.Targets[name]
		if _, err := c.targetProfile(&t, nil); err != nil {
			return nil, fmt.Errorf("%s: %q: %w", TargetsFile, name, err)
		}
	}
	for _, name := range sortedKeys(c.Actions) {
		a := c.Actions[name]
		if err := c.checkAction(&a); err != nil {
			return nil, fmt.Errorf("%s: %q: %w", ActionsFile, name, err)
		}
	}
	files, err := filepath.Glob(filepath.Join(dir, GamesDir, "*.json"))
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		g, err := c.loadGame(f)
		if err != nil {
			return nil, err
		}
		c.Games = append(c.Games, g)
	}
	sortGames(c.Games)
	return c, nil
}

// Game gives the game with the id, or nil.
func (c *Catalog) Game(id string) *Game {
	for _, g := range c.Games {
		if g.ID == id {
			return g
		}
	}
	return nil
}

// loadInfo reads catalog.json.
func (c *Catalog) loadInfo() error {
	data, err := readJSON(filepath.Join(c.Dir, CatalogFile))
	if err != nil {
		return err
	}
	if err := decodeStrict(data, &c.CatalogInfo); err != nil {
		return fmt.Errorf("%s: %w", CatalogFile, err)
	}
	if c.Spec != Version {
		return fmt.Errorf("%s: spec must be %d, not %d", CatalogFile, Version, c.Spec)
	}
	if !isName(c.ID) {
		return fmt.Errorf("%s: id %q is not valid. Use a-z, 0-9 and -.", CatalogFile, c.ID)
	}
	if isBlank(c.Title) {
		return fmt.Errorf("%s: title is empty", CatalogFile)
	}
	for _, name := range sortedKeys(c.Active) {
		ref := c.Active[name]
		if !isActiveName(name) {
			return fmt.Errorf("%s: active: name %q is not valid. Use A-Z, 0-9 and _.", CatalogFile, name)
		}
		if !strings.HasPrefix(ref, PrefixCat) || !isFakeValue(ref) {
			return fmt.Errorf("%s: active: %q must be a cat: file: %s", CatalogFile, name, ref)
		}
		if err := c.checkCatFile(ref); err != nil {
			return fmt.Errorf("%s: active: %q: %w", CatalogFile, name, err)
		}
	}
	for _, rel := range sortedKeys(c.Files) {
		if !isRelPath(rel) || rel == CatalogFile || strings.HasPrefix(rel, SchemaDir+"/") {
			return fmt.Errorf("%s: files: path %q is not valid", CatalogFile, rel)
		}
		if h := c.Files[rel]; len(h) != 64 || !isHex(h) || strings.ToLower(h) != h {
			return fmt.Errorf("%s: files: %q: %q is not a SHA256 in lowercase hex", CatalogFile, rel, h)
		}
	}
	return nil
}

// isRelPath tells if p is a relative path with / and no empty, . or ..
// part.
func isRelPath(p string) bool {
	if p == "" || strings.Contains(p, `\`) {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

// checkCatFile tells if the cat: ref is a file in the catalog folder.
func (c *Catalog) checkCatFile(ref string) error {
	name := strings.TrimPrefix(ref, PrefixCat)
	if !isRelPath(name) {
		return fmt.Errorf("cat: path must be a relative path with / and no ..: %s", ref)
	}
	if st, err := os.Stat(filepath.Join(c.Dir, filepath.FromSlash(name))); err != nil || st.IsDir() {
		return fmt.Errorf("catalog file not found: %s", ref)
	}
	return nil
}

// checkRootFiles tells if the root of the catalog has a JSON file that is
// not a catalog file. A strategy file in the root is an error: it must be
// in a folder of its origin.
func checkRootFiles(dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return err
	}
	for _, f := range files {
		switch filepath.Base(f) {
		case CatalogFile, ListsFile, TargetsFile, ActionsFile:
			continue
		}
		return fmt.Errorf("%s: unknown file in the catalog root. Put a strategy in %s/ or %s/.", filepath.Base(f), OriginFlowseal, OriginZapman)
	}
	return nil
}

// strategyFiles gives the strategy files of the catalog by name. A name in
// two origin folders is an error.
func (c *Catalog) strategyFiles() (map[string]Origin, error) {
	out := map[string]Origin{}
	for _, o := range Origins {
		files, err := filepath.Glob(filepath.Join(c.Dir, string(o), "*.json"))
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			name := strings.TrimSuffix(filepath.Base(f), ".json")
			if prev, ok := out[name]; ok {
				return nil, fmt.Errorf("strategy %q is in %s/ and in %s/", name, prev, o)
			}
			out[name] = o
		}
	}
	return out, nil
}

// StrategyNames gives the names of the strategy files in all origin
// folders, sorted.
func (c *Catalog) StrategyNames() ([]string, error) {
	files, err := c.strategyFiles()
	if err != nil {
		return nil, err
	}
	return sortedKeys(files), nil
}

// Strategy reads the strategy file <origin>/<name>.json of the catalog.
func (c *Catalog) Strategy(name string) (*Strategy, error) {
	if isBlank(name) {
		return nil, errors.New("Strategy name is empty.")
	}
	files, err := c.strategyFiles()
	if err != nil {
		return nil, err
	}
	o, ok := files[name]
	if !ok {
		return nil, fmt.Errorf("strategy %q not found in %s", name, c.Dir)
	}
	data, err := readJSON(filepath.Join(c.Dir, string(o), name+".json"))
	if err != nil {
		return nil, err
	}
	return c.ParseStrategy(name, o, data)
}

// ParseStrategy reads the JSON data of the strategy name in the origin
// folder, as Strategy does with the file <origin>/<name>.json. The file
// need not be on the disk. The dev tools use it to check a strategy before
// they write it.
func (c *Catalog) ParseStrategy(name string, o Origin, data []byte) (*Strategy, error) {
	file := string(o) + "/" + name + ".json"
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	s := &Strategy{Name: name, origin: o, file: file, active: c.Active}
	if err := decodeStrict(data, &s.StrategyFile); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	if s.Spec != Version {
		return nil, fmt.Errorf("%s: spec must be %d, not %d", file, Version, s.Spec)
	}
	if err := s.checkVersion(); err != nil {
		return nil, err
	}
	if len(s.Rules) == 0 {
		return nil, fmt.Errorf("%s: Strategy has no rules.", file)
	}
	var err error
	if s.rules, err = c.resolveRules(file, s.Rules, nil); err != nil {
		return nil, err
	}
	return s, nil
}

func (c *Catalog) loadGame(path string) (*Game, error) {
	file := filepath.ToSlash(filepath.Join(GamesDir, filepath.Base(path)))
	data, err := readJSON(path)
	if err != nil {
		return nil, err
	}
	g := &Game{file: file}
	if err := decodeStrict(data, &g.GameFile); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	if want := strings.TrimSuffix(filepath.Base(path), ".json"); g.ID != want {
		return nil, fmt.Errorf("%s: id must be %q", file, want)
	}
	if !isName(g.ID) {
		return nil, fmt.Errorf("%s: id %q is not valid. Use a-z, 0-9 and -.", file, g.ID)
	}
	if isBlank(g.Title) {
		return nil, fmt.Errorf("%s: title is empty", file)
	}
	for _, lang := range sortedKeys(g.About) {
		if !AboutLangs[lang] {
			return nil, fmt.Errorf("%s: about: unknown language %q. Use en or ru", file, lang)
		}
		if isBlank(g.About[lang]) {
			return nil, fmt.Errorf("%s: about: the %s text is empty", file, lang)
		}
	}
	if g.GameFile.Version < 1 {
		return nil, fmt.Errorf("%s: version must be 1 or more", file)
	}
	if g.Updater != "" && !gameUpdaters[g.Updater] {
		return nil, fmt.Errorf("%s: unknown updater %q", file, g.Updater)
	}
	if len(g.Rules) == 0 {
		return nil, fmt.Errorf("%s: Game has no rules.", file)
	}
	if err := checkGameOptions(file, g.Options); err != nil {
		return nil, err
	}
	used := map[string]bool{}
	for _, name := range sortedKeys(g.Lists) {
		b := g.Lists[name]
		if !isName(name) {
			return nil, fmt.Errorf("%s: lists: name %q is not valid. Use a-z, 0-9 and -.", file, name)
		}
		if _, ok := c.Lists[name]; ok {
			return nil, fmt.Errorf("%s: lists: %q is also in %s", file, name, ListsFile)
		}
		if err := c.checkBundle(&b.Bundle); err != nil {
			return nil, fmt.Errorf("%s: lists: %q: %w", file, name, err)
		}
		if b.When != "" {
			if _, ok := g.Options[b.When]; !ok {
				return nil, fmt.Errorf("%s: lists: %q: when: unknown option %q", file, name, b.When)
			}
			used[b.When] = true
		}
	}
	for _, name := range sortedKeys(g.Options) {
		if !used[name] {
			return nil, fmt.Errorf("%s: options: %q is not used. Use it in when of a bundle.", file, name)
		}
	}
	if err := c.resolveGame(g); err != nil {
		return nil, err
	}
	return g, nil
}

// checkGameOptions checks the options of a game file.
func checkGameOptions(file string, opts map[string]GameOption) error {
	if len(opts) > MaxGameOptions {
		return fmt.Errorf("%s: options: a game can have %d options or fewer", file, MaxGameOptions)
	}
	for _, name := range sortedKeys(opts) {
		o := opts[name]
		if !isName(name) {
			return fmt.Errorf("%s: options: name %q is not valid. Use a-z, 0-9 and -.", file, name)
		}
		if len(o.Title) == 0 {
			return fmt.Errorf("%s: options: %q: title is empty", file, name)
		}
		for _, field := range []struct {
			name  string
			texts map[string]string
		}{{"title", o.Title}, {"about", o.About}} {
			for _, lang := range sortedKeys(field.texts) {
				if !AboutLangs[lang] {
					return fmt.Errorf("%s: options: %q: %s: unknown language %q. Use en or ru", file, name, field.name, lang)
				}
				if isBlank(field.texts[lang]) {
					return fmt.Errorf("%s: options: %q: %s: the %s text is empty", file, name, field.name, lang)
				}
			}
		}
	}
	return nil
}

// resolveGame resolves the rules of the game for each value set of its
// options. Each option must change the rules. g.rules are the rules with
// the default values.
func (c *Catalog) resolveGame(g *Game) error {
	names := sortedKeys(g.Options)
	g.variants = map[string][]resolvedRule{}
	for mask := range 1 << len(names) {
		vals := map[string]bool{}
		for i, n := range names {
			vals[n] = mask&(1<<i) != 0
		}
		rules, err := c.resolveRules(g.file, g.Rules, g.bundles(vals))
		if err != nil {
			if len(names) > 0 {
				return fmt.Errorf("%w (options %s)", err, optionText(names, vals))
			}
			return err
		}
		g.variants[optionText(names, vals)] = rules
	}
	for i, n := range names {
		for mask := range 1 << len(names) {
			if mask&(1<<i) != 0 {
				continue
			}
			off, on := map[string]bool{}, map[string]bool{}
			for j, m := range names {
				off[m] = mask&(1<<j) != 0
				on[m] = off[m]
			}
			on[n] = true
			if reflect.DeepEqual(g.variants[optionText(names, off)], g.variants[optionText(names, on)]) {
				return fmt.Errorf("%s: options: %q does not change the rules (options %s)", g.file, n, optionText(names, off))
			}
		}
	}
	g.values = map[string]bool{}
	for _, n := range names {
		g.values[n] = g.Options[n].Default
	}
	g.rules = g.variants[optionText(names, g.values)]
	return nil
}

// bundles gives the bundles of the game for the option values. A bundle
// with an option that is off is empty.
func (g *Game) bundles(vals map[string]bool) map[string]GameBundle {
	out := make(map[string]GameBundle, len(g.Lists))
	for name, b := range g.Lists {
		if b.When != "" && !vals[b.When] {
			b = GameBundle{When: b.When}
		}
		out[name] = b
	}
	return out
}

// optionText gives the option values as text: name=on or name=off, in the
// order of names, with spaces between them.
func optionText(names []string, vals map[string]bool) string {
	parts := make([]string, 0, len(names))
	for _, n := range names {
		v := "off"
		if vals[n] {
			v = "on"
		}
		parts = append(parts, n+"="+v)
	}
	return strings.Join(parts, " ")
}

// Values gives a copy of the option values of the game.
func (g *Game) Values() map[string]bool {
	out := make(map[string]bool, len(g.values))
	for n, v := range g.values {
		out[n] = v
	}
	return out
}

// OptionText gives the option values of the game as text, for example
// "ipset=off". It is empty when the game has no options.
func (g *Game) OptionText() string {
	return optionText(sortedKeys(g.Options), g.values)
}

// With gives a copy of the game with the option values set. An option that
// is not in vals keeps its value. An unknown option is an error.
func (g *Game) With(vals map[string]bool) (*Game, error) {
	out := *g
	out.values = g.Values()
	for _, n := range sortedKeys(vals) {
		if _, ok := g.Options[n]; !ok {
			return nil, fmt.Errorf("game %q has no option %q", g.ID, n)
		}
		out.values[n] = vals[n]
	}
	out.rules = g.variants[optionText(sortedKeys(g.Options), out.values)]
	return &out, nil
}

// resolveRules gives the rules as checked v0.1 profiles.
func (c *Catalog) resolveRules(file string, rules []Rule, local map[string]GameBundle) ([]resolvedRule, error) {
	out := make([]resolvedRule, 0, len(rules))
	for i := range rules {
		origin := fmt.Sprintf("%s: rules[%d]", file, i)
		r, err := c.resolveRule(&rules[i], local)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", origin, err)
		}
		r.origin = origin
		out = append(out, r)
	}
	return out, nil
}

func (c *Catalog) resolveRule(r *Rule, local map[string]GameBundle) (resolvedRule, error) {
	t := r.Target.Inline
	if t == nil {
		nt, ok := c.Targets[r.Target.Name]
		if !ok {
			return resolvedRule{}, fmt.Errorf("unknown target %q", r.Target.Name)
		}
		t = &nt
	}
	a := r.Action.Inline
	if a == nil {
		na, ok := c.Actions[r.Action.Name]
		if !ok {
			return resolvedRule{}, fmt.Errorf("unknown action %q", r.Action.Name)
		}
		a = &na
	}
	p, err := c.targetProfile(t, local)
	if err != nil {
		return resolvedRule{}, err
	}
	if err := checkFakes(a.Fake); err != nil {
		return resolvedRule{}, err
	}
	applyAction(&p, a)
	if err := p.validate(); err != nil {
		return resolvedRule{}, err
	}
	if err := c.checkBin(&p); err != nil {
		return resolvedRule{}, err
	}
	return resolvedRule{profile: p, tcp: t.TCP, udp: t.UDP}, nil
}

// targetProfile gives the filter part of a profile and checks the target.
func (c *Catalog) targetProfile(t *Target, local map[string]GameBundle) (V01Profile, error) {
	p := V01Profile{
		L3:                     t.L3,
		L7:                     strings.Join(t.L7, ","),
		Payload:                t.Payload,
		AnyProtocol:            t.AnyProtocol,
		HostlistDomains:        t.Domains,
		HostlistExcludeDomains: t.ExcludeDomains,
	}
	var err error
	if p.TCP, err = portList(t.TCP, "$gf.Tcp"); err != nil {
		return p, fmt.Errorf("tcp: %w", err)
	}
	if p.UDP, err = portList(t.UDP, "$gf.Udp"); err != nil {
		return p, fmt.Errorf("udp: %w", err)
	}
	for _, n := range t.L7 {
		if !v01L7[n] {
			return p, fmt.Errorf("Unknown l7 '%s'.", n)
		}
	}
	if t.Payload != "" && !matchLuaPayload(t.Payload) {
		return p, errors.New("payload must contain comma-separated payload names.")
	}
	for _, use := range []struct {
		names   []string
		exclude bool
	}{{t.Lists, false}, {t.Exclude, true}} {
		for _, name := range use.names {
			b, ok := c.Lists[name]
			if lb, lok := local[name]; lok {
				b, ok = lb.Bundle, true
			}
			if !ok {
				return p, fmt.Errorf("unknown list bundle %q", name)
			}
			if use.exclude && (len(b.Hostlist) > 0 || len(b.Ipset) > 0) {
				return p, fmt.Errorf("bundle %q in exclude has hostlist or ipset refs", name)
			}
			if !use.exclude && (len(b.HostlistExclude) > 0 || len(b.IpsetExclude) > 0) {
				return p, fmt.Errorf("bundle %q in lists has exclude refs", name)
			}
			p.Hostlist = append(p.Hostlist, b.Hostlist...)
			p.HostlistExclude = append(p.HostlistExclude, b.HostlistExclude...)
			p.Ipset = append(p.Ipset, b.Ipset...)
			p.IpsetExclude = append(p.IpsetExclude, b.IpsetExclude...)
		}
	}
	// Check the filter fields with the v0.1 rules. The attack is only a
	// placeholder.
	check := p
	check.Desync = StringList{"multisplit"}
	if err := check.validate(); err != nil {
		return p, err
	}
	return p, c.checkBin(&p)
}

// applyAction puts the attack fields of the action into the profile.
func applyAction(p *V01Profile, a *Action) {
	p.Desync = append(StringList(nil), a.Desync...)
	p.Repeats = a.Repeats
	p.TTL = a.TTL
	if a.Cutoff != nil {
		p.Cutoff = a.Cutoff.String()
	}
	p.SplitPos = a.SplitPos
	p.Seqovl = a.Seqovl
	p.SeqovlPattern = a.SeqovlPattern
	p.Fooling = a.Fooling
	p.BadseqIncrement = a.BadseqIncrement
	p.IPID = a.IPID
	p.HostfakesplitMod = a.HostfakesplitMod
	p.FakedsplitPattern = a.FakedsplitPattern
	p.TLSMod = a.TLSMod
	p.LuaPayload = a.LuaPayload
	if f := a.Fake; f != nil {
		for _, s := range []struct {
			dst *FakeSlot
			src []string
		}{
			{&p.Fake.QUIC, f.QUIC}, {&p.Fake.TLS, f.TLS}, {&p.Fake.HTTP, f.HTTP},
			{&p.Fake.Discord, f.Discord}, {&p.Fake.STUN, f.STUN},
			{&p.Fake.Unknown, f.Unknown}, {&p.Fake.UnknownUDP, f.UnknownUDP},
		} {
			if s.src != nil {
				*s.dst = FakeSlot{Set: true, Refs: s.src}
			}
		}
	}
}

// checkAction checks an action with the v0.1 rules.
func (c *Catalog) checkAction(a *Action) error {
	if err := checkFakes(a.Fake); err != nil {
		return err
	}
	var p V01Profile
	applyAction(&p, a)
	if err := p.validate(); err != nil {
		return err
	}
	return c.checkBin(&p)
}

// checkFakes tells if a slot is in the JSON with no refs.
func checkFakes(f *Fakes) error {
	if f == nil {
		return nil
	}
	for _, s := range []struct {
		name string
		refs []string
	}{
		{"quic", f.QUIC}, {"tls", f.TLS}, {"http", f.HTTP}, {"discord", f.Discord},
		{"stun", f.STUN}, {"unknown", f.Unknown}, {"unknownUdp", f.UnknownUDP},
	} {
		if s.refs != nil && len(s.refs) == 0 {
			return fmt.Errorf("Fake slot '%s' is empty.", s.name)
		}
	}
	return nil
}

func (c *Catalog) checkBundle(b *Bundle) error {
	n := 0
	for _, refs := range [][]string{b.Hostlist, b.HostlistExclude, b.Ipset, b.IpsetExclude} {
		for _, ref := range refs {
			if !isFileRef(ref) {
				return fmt.Errorf("Path prefix is not %s : %s", refPrefixText, ref)
			}
			n++
		}
	}
	if n == 0 {
		return errors.New("bundle has no refs")
	}
	return c.checkBin(&V01Profile{Hostlist: b.Hostlist, HostlistExclude: b.HostlistExclude, Ipset: b.Ipset, IpsetExclude: b.IpsetExclude})
}

// checkBin checks the file refs of the profile. A cat: ref must be a file
// in the catalog folder. An active: ref must have a default in
// catalog.json. A bin: ref must be a file in c.Bin; this check is only done
// when c.Bin is not empty.
func (c *Catalog) checkBin(p *V01Profile) error {
	for _, r := range p.fileRefs() {
		pre, name, ok := splitRef(r)
		if !ok {
			continue
		}
		switch {
		case pre == PrefixCat:
			if err := c.checkCatFile(r); err != nil {
				return err
			}
		case pre == PrefixActive:
			if _, ok := c.Active[name]; !ok {
				return fmt.Errorf("active fake %s has no default in %s", r, CatalogFile)
			}
		case pre == PrefixBin && c.Bin != "":
			path, _ := resolveRef(r, Paths{Bin: c.Bin})
			if st, err := os.Stat(path); err != nil || st.IsDir() {
				return fmt.Errorf("bin file not found: %s", r)
			}
		}
	}
	return nil
}

// portList gives the v0.1 port text of the items. gfToken replaces
// GameFilterPort.
func portList(items []string, gfToken string) (string, error) {
	out := make([]string, 0, len(items))
	for _, it := range items {
		if it == GameFilterPort {
			out = append(out, gfToken)
			continue
		}
		if !isPortItem(it) {
			return "", fmt.Errorf("port must be N, N-M or %s: %q", GameFilterPort, it)
		}
		out = append(out, it)
	}
	return strings.Join(out, ","), nil
}

// isPortItem tells if s is N or N-M with N and M from 0 to 65535, N <= M.
func isPortItem(s string) bool {
	lo, hi, rng := strings.Cut(s, "-")
	a, ok := portNumber(lo)
	if !ok {
		return false
	}
	if !rng {
		return true
	}
	b, ok := portNumber(hi)
	return ok && a <= b
}

func portNumber(s string) (int, bool) {
	if s == "" || len(s) > 5 {
		return 0, false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, _ := strconv.Atoi(s)
	return n, n <= 65535
}

// isName tells if s is a name of a bundle, target, action or game: a-z, 0-9
// and -, not at the start or the end.
func isName(s string) bool {
	if s == "" || s[0] == '-' || s[len(s)-1] == '-' {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}

// loadNamed reads a JSON object of named items. The "$schema" key is not
// an item.
func loadNamed[T any](path string) (map[string]T, error) {
	file := filepath.Base(path)
	data, err := readJSON(path)
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	if raw == nil {
		return nil, fmt.Errorf("%s: must be a JSON object", file)
	}
	if s, ok := raw["$schema"]; ok {
		var v string
		if err := json.Unmarshal(s, &v); err != nil {
			return nil, fmt.Errorf("%s: $schema must be a string", file)
		}
		delete(raw, "$schema")
	}
	out := make(map[string]T, len(raw))
	for name, b := range raw {
		if !isName(name) {
			return nil, fmt.Errorf("%s: name %q is not valid. Use a-z, 0-9 and -.", file, name)
		}
		var v T
		if err := decodeStrict(b, &v); err != nil {
			return nil, fmt.Errorf("%s: %q: %w", file, name, err)
		}
		out[name] = v
	}
	return out, nil
}

// readJSON reads a file and removes a UTF-8 BOM.
func readJSON(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("JSON file not found: %s", path)
		}
		return nil, err
	}
	return bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")), nil
}

func sortedKeys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// sortGames puts the games in build order: by Order, then by ID.
func sortGames(gs []*Game) {
	sort.SliceStable(gs, func(i, j int) bool {
		if gs[i].Order != gs[j].Order {
			return gs[i].Order < gs[j].Order
		}
		return gs[i].ID < gs[j].ID
	})
}
