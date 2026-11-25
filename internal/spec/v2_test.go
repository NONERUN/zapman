package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// v2Tree writes a strategies folder with the files and gives its path.
// Missing catalog files get a default.
func v2Tree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	defaults := map[string]string{
		ListsFile:   `{"$schema":"schema/lists.json","general":{"hostlist":["lists:list-general.txt"]},"exclude-hosts":{"hostlistExclude":["lists:list-exclude.txt"]}}`,
		TargetsFile: `{"quic-general":{"udp":["443"],"l7":["quic"],"lists":["general"],"exclude":["exclude-hosts"]},"gf-udp":{"udp":["$gameFilter"],"anyProtocol":true}}`,
		ActionsFile: `{"fake-quic":{"desync":["fake"],"repeats":6,"fake":{"quic":["bin:q.bin"]}},"fake-game":{"desync":["fake"],"cutoff":4,"fake":{"unknownUdp":["bin:g.bin"]}}}`,
	}
	for name, text := range defaults {
		if _, ok := files[name]; !ok {
			files[name] = text
		}
	}
	for name, text := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const okStrategy = `{"spec":2,"rules":[{"target":"quic-general","action":"fake-quic"},{"target":"gf-udp","action":"fake-game"}]}`

func TestV2Build(t *testing.T) {
	dir := v2Tree(t, map[string]string{
		"s.json":       okStrategy,
		"games/b.json": `{"id":"b","title":"B","order":2,"rules":[{"target":{"udp":["5000-6000"],"anyProtocol":true},"action":"fake-game"}]}`,
		"games/a.json": `{"id":"a","title":"A","order":2,"lists":{"ipset-a":{"ipset":["user:ipset-a.txt"],"seed":"lists:ipset-a.default.txt"}},
			"rules":[{"target":{"udp":["443","27000"],"lists":["ipset-a"]},"action":{"desync":["fake"],"ttl":8,"cutoff":"none","luaPayload":"all","fake":{"unknownUdp":["bin:g.bin"]}}}]}`,
	})
	c, err := LoadCatalog(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Games) != 2 || c.Games[0].ID != "a" || c.Games[1].ID != "b" {
		t.Fatalf("games are not in order: %v", c.Games)
	}
	st, err := c.Strategy("s")
	if err != nil {
		t.Fatal(err)
	}
	gf := GameFilter{TCP: "12", UDP: "13"}
	lines, err := Build(st, []*Game{c.Game("b"), c.Game("a")}, Winws, goldenPaths, gf)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"--wf-tcp= --wf-udp=443,5000-6000,27000,13",
		`--filter-udp=443,27000 --ipset="C:\zapman\user\ipset-a.txt" --dpi-desync=fake --dpi-desync-ttl=8 --dpi-desync-fake-unknown-udp="C:\zapman\bin\g.bin" --new`,
		`--filter-udp=5000-6000 --dpi-desync=fake --dpi-desync-any-protocol=1 --dpi-desync-cutoff=n4 --dpi-desync-fake-unknown-udp="C:\zapman\bin\g.bin" --new`,
		`--filter-udp=443 --filter-l7=quic --hostlist="C:\zapman\lists\list-general.txt" --hostlist-exclude="C:\zapman\lists\list-exclude.txt" --dpi-desync=fake --dpi-desync-repeats=6 --dpi-desync-fake-quic="C:\zapman\bin\q.bin" --new`,
		`--filter-udp=13 --dpi-desync=fake --dpi-desync-any-protocol=1 --dpi-desync-cutoff=n4 --dpi-desync-fake-unknown-udp="C:\zapman\bin\g.bin"`,
	}
	if got := strings.Join(lines, "\n"); got != strings.Join(want, "\n") {
		t.Errorf("argv differs\n%s", firstDiff(got, strings.Join(want, "\n")))
	}
	z2, err := Build(st, c.Games, Winws2, goldenPaths, gf)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(z2[3], "--filter-udp=443,27000 --ipset=\"C:\\zapman\\user\\ipset-a.txt\" --lua-desync=fake:payload=all:blob=g:ip_ttl=8 --new") {
		t.Errorf("winws2 game rule: %s", z2[3])
	}
	if _, err := Build(st, []*Game{c.Game("a"), c.Game("a")}, Winws, goldenPaths, gf); err == nil {
		t.Error("game two times: no error")
	}
	if _, err := Build(nil, nil, Winws, goldenPaths, gf); err == nil {
		t.Error("nil strategy: no error")
	}
}

func TestV2Errors(t *testing.T) {
	rule := func(target, action string) string {
		return `{"spec":2,"rules":[{"target":"quic-general","action":"fake-quic"},{"target":` + target + `,"action":` + action + `}]}`
	}
	cases := []struct {
		name    string
		files   map[string]string
		wantErr string
	}{
		{"spec 1", map[string]string{"s.json": `{"spec":1,"rules":[]}`}, "s.json: spec must be 2, not 1"},
		{"no rules", map[string]string{"s.json": `{"spec":2,"rules":[]}`}, "s.json: Strategy has no rules."},
		{"unknown root field", map[string]string{"s.json": `{"spec":2,"wf":{},"rules":[]}`}, `unknown field "wf"`},
		{"unknown rule field", map[string]string{"s.json": `{"spec":2,"rules":[{"target":"gf-udp","action":"fake-game","note":1}]}`}, `unknown field "note"`},
		{"unknown target", map[string]string{"s.json": rule(`"quic"`, `"fake-quic"`)}, `s.json: rules[1]: unknown target "quic"`},
		{"unknown action", map[string]string{"s.json": rule(`"gf-udp"`, `"fake"`)}, `s.json: rules[1]: unknown action "fake"`},
		{"empty name", map[string]string{"s.json": rule(`""`, `"fake-quic"`)}, "name is empty"},
		{"inline target unknown field", map[string]string{"s.json": rule(`{"udp":["1"],"ipset":["user:x.txt"]}`, `"fake-quic"`)}, `unknown field "ipset"`},
		{"inline action unknown field", map[string]string{"s.json": rule(`"gf-udp"`, `{"desync":["fake"],"payload":"all"}`)}, `unknown field "payload"`},
		{"bad port", map[string]string{"s.json": rule(`{"udp":["443,80"]}`, `"fake-game"`)}, `s.json: rules[1]: udp: port must be N, N-M or $gameFilter: "443,80"`},
		{"bad range", map[string]string{"s.json": rule(`{"tcp":["90-80"]}`, `"fake-game"`)}, `tcp: port must be`},
		{"old gf token", map[string]string{"s.json": rule(`{"tcp":["$gf.Tcp"]}`, `"fake-game"`)}, `tcp: port must be`},
		{"unknown l7", map[string]string{"s.json": rule(`{"udp":["1"],"l7":["tls,http"]}`, `"fake-game"`)}, "Unknown l7 'tls,http'."},
		{"unknown bundle", map[string]string{"s.json": rule(`{"udp":["1"],"lists":["google"]}`, `"fake-game"`)}, `unknown list bundle "google"`},
		{"include bundle in exclude", map[string]string{"s.json": rule(`{"udp":["1"],"exclude":["general"]}`, `"fake-game"`)}, `bundle "general" in exclude has hostlist or ipset refs`},
		{"exclude bundle in lists", map[string]string{"s.json": rule(`{"udp":["1"],"lists":["exclude-hosts"]}`, `"fake-game"`)}, `bundle "exclude-hosts" in lists has exclude refs`},
		{"v0.1 rule in action", map[string]string{"s.json": rule(`"gf-udp"`, `{"desync":["fake"],"fooling":["TS"]}`)}, "s.json: rules[1]: Unknown fooling 'TS'."},
		{"cutoff text", map[string]string{"s.json": rule(`"gf-udp"`, `{"desync":["fake"],"cutoff":"n4"}`)}, `cutoff must be a number or "none"`},
		{"cutoff negative", map[string]string{"s.json": rule(`"gf-udp"`, `{"desync":["fake"],"cutoff":-1}`)}, `cutoff must be a number or "none"`},
		{"empty fake slot", map[string]string{"s.json": rule(`"gf-udp"`, `{"desync":["fake"],"fake":{"quic":[]}}`)}, "Fake slot 'quic' is empty."},
		{"bad named target", map[string]string{TargetsFile: `{"t":{"udp":["x"]}}`, "s.json": okStrategy}, `targets.json: "t": udp: port must be`},
		{"bad named action", map[string]string{ActionsFile: `{"a":{"desync":["Fake"]}}`, "s.json": okStrategy}, `actions.json: "a": Unknown desync attack 'Fake'.`},
		{"bad name", map[string]string{ActionsFile: `{"Fake":{"desync":["fake"]}}`, "s.json": okStrategy}, `actions.json: name "Fake" is not valid`},
		{"empty bundle", map[string]string{ListsFile: `{"x":{}}`, "s.json": okStrategy}, `lists.json: "x": bundle has no refs`},
		{"bundle prefix", map[string]string{ListsFile: `{"x":{"ipset":["ipset.txt"]}}`, "s.json": okStrategy}, `lists.json: "x": Path prefix is not lists:/user:/bin: : ipset.txt`},
		{"unknown bundle field", map[string]string{ListsFile: `{"x":{"hostlistDomains":["a.com"]}}`, "s.json": okStrategy}, `unknown field "hostlistDomains"`},
		{"game id", map[string]string{"games/g.json": `{"id":"h","title":"G","order":1,"rules":[{"target":"gf-udp","action":"fake-game"}]}`, "s.json": okStrategy}, `games/g.json: id must be "g"`},
		{"game no title", map[string]string{"games/g.json": `{"id":"g","title":"","order":1,"rules":[{"target":"gf-udp","action":"fake-game"}]}`, "s.json": okStrategy}, `games/g.json: title is empty`},
		{"game updater", map[string]string{"games/g.json": `{"id":"g","title":"G","order":1,"updater":"x","rules":[{"target":"gf-udp","action":"fake-game"}]}`, "s.json": okStrategy}, `unknown updater "x"`},
		{"game bundle clash", map[string]string{"games/g.json": `{"id":"g","title":"G","order":1,"lists":{"general":{"ipset":["user:x.txt"]}},"rules":[{"target":"gf-udp","action":"fake-game"}]}`, "s.json": okStrategy}, `games/g.json: lists: "general" is also in lists.json`},
		{"game seed", map[string]string{"games/g.json": `{"id":"g","title":"G","order":1,"lists":{"x":{"ipset":["user:x.txt"],"seed":"user:x.txt"}},"rules":[{"target":"gf-udp","action":"fake-game"}]}`, "s.json": okStrategy}, `seed must be a lists: path: user:x.txt`},
		{"game rule", map[string]string{"games/g.json": `{"id":"g","title":"G","order":1,"rules":[{"target":"gf-udp","action":"nope"}]}`, "s.json": okStrategy}, `games/g.json: rules[0]: unknown action "nope"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := v2Tree(t, c.files)
			cat, err := LoadCatalog(dir, "")
			if err == nil {
				_, err = cat.Strategy("s")
			}
			if err == nil {
				t.Fatal("no error")
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("error %q, want %q", err, c.wantErr)
			}
		})
	}
}

func TestV2Bin(t *testing.T) {
	dir := v2Tree(t, map[string]string{"s.json": okStrategy})
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "q.bin"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCatalog(dir, bin); err == nil || !strings.Contains(err.Error(), `actions.json: "fake-game": bin file not found: bin:g.bin`) {
		t.Errorf("missing bin file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bin, "g.bin"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadCatalog(dir, bin)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Strategy("s"); err != nil {
		t.Error(err)
	}
}

func TestPortLess(t *testing.T) {
	st := &Strategy{rules: []resolvedRule{
		{tcp: []string{"443", GameFilterPort}, udp: []string{"50000-50100"}},
		{tcp: []string{"2053", "80"}, udp: []string{"5055-5058", "443", "5000-8000"}},
		{tcp: []string{"443"}, udp: []string{"5000"}},
	}}
	tcp, udp := WFPorts(st, nil)
	if got := strings.Join(tcp, ","); got != "80,443,2053,$gf.Tcp" {
		t.Errorf("tcp: %s", got)
	}
	if got := strings.Join(udp, ","); got != "443,5000,5000-8000,5055-5058,50000-50100" {
		t.Errorf("udp: %s", got)
	}
}
