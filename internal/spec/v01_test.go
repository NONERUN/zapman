package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEdge checks rare cases. testdata/edge.*.txt is the argv for
// testdata/edge.json with Game Filter 12 / 13. -update writes it.
func TestEdge(t *testing.T) {
	s, err := LoadFile(filepath.Join("testdata", "edge.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, eng := range []Engine{Winws, Winws2} {
		path := filepath.Join("testdata", "edge."+string(eng)+".txt")
		lines, err := Build(s, eng, goldenPaths, GameFilter{TCP: "12", UDP: "13"})
		if err != nil {
			t.Fatalf("%s: %v", eng, err)
		}
		got := strings.Join(lines, "\n") + "\n"
		if *update {
			writeGolden(t, path, got)
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		w := strings.ReplaceAll(string(want), "\r\n", "\n")
		if got != w {
			t.Errorf("%s: argv differs\n%s", eng, firstDiff(got, w))
		}
	}
}

func TestFakeSlotNull(t *testing.T) {
	s, err := Parse([]byte(`{"specVersion":"0.1.0","wf":{},"profiles":[{"desync":["fake"],"fake":{"quic":null}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	f := s.Profiles[0].Fake
	if !f.QUIC.Set || len(f.QUIC.Refs) != 0 || f.TLS.Set {
		t.Fatalf("slots: quic=%+v tls=%+v", f.QUIC, f.TLS)
	}
}

const okProfile = `"udp":"443","l7":"quic","desync":["fake"],"fake":{"quic":["bin:q.bin"]}`

func specWith(profile string) string {
	return `{"specVersion":"0.1.0","wf":{"tcp":["443"],"udp":["443"]},"profiles":[{` + profile + `}]}`
}

func TestErrors(t *testing.T) {
	both := []Engine{Winws, Winws2}
	z1 := []Engine{Winws}
	z2 := []Engine{Winws2}
	cases := []struct {
		name    string
		json    string
		fail    []Engine // engines that give the error
		pass    []Engine // engines that build
		wantErr string
	}{
		{"parse: no specVersion", `{"wf":{},"profiles":[]}`, both, nil, "has no specVersion"},
		{"parse: blank specVersion", `{"specVersion":" ","wf":{},"profiles":[]}`, both, nil, "has no specVersion"},
		{"parse: wrong specVersion", `{"specVersion":"0.2.0","wf":{},"profiles":[]}`, both, nil, "specVersion '0.2.0' does not match ZapretSpec 0.1.0"},
		{"parse: number specVersion", `{"specVersion":0.1,"wf":{},"profiles":[]}`, both, nil, "specVersion '0.1' does not match"},
		{"parse: null", `null`, both, nil, "Strategy JSON is empty"},
		{"parse: bad json", `{"specVersion":"0.1.0",`, both, nil, "Strategy JSON is not valid"},
		{"parse: unknown root field", `{"specVersion":"0.1.0","wf":{},"profiles":[],"extra":1}`, both, nil, `unknown field "extra"`},
		{"parse: unknown wf field", `{"specVersion":"0.1.0","wf":{"tcpx":[]},"profiles":[]}`, both, nil, `unknown field "tcpx"`},
		{"parse: unknown profile field", specWith(okProfile + `,"fakeTls":["x"]`), both, nil, `unknown field "fakeTls"`},
		{"parse: unknown fake slot", specWith(`"desync":["fake"],"fake":{"dtls":["bin:x.bin"]}`), both, nil, `unknown field "dtls"`},
		{"parse: ttl not int", specWith(okProfile + `,"ttl":"8"`), both, nil, "Strategy JSON is not valid"},

		{"no wf", `{"specVersion":"0.1.0","profiles":[]}`, both, nil, "Strategy JSON has no wf."},
		{"no profiles", `{"specVersion":"0.1.0","wf":{}}`, both, nil, "Strategy profile has no desync attacks."},
		{"empty desync", specWith(`"udp":"443","desync":[]`), both, nil, "Strategy profile has no desync attacks."},
		{"unknown desync", specWith(`"udp":"443","desync":["fake","disorder2"]`), both, nil, "Unknown desync attack 'disorder2'."},
		{"bad fooling", specWith(okProfile + `,"fooling":["datanoack"]`), z2, z1, "Unknown fooling 'datanoack'."},
		{"ttl 0", specWith(okProfile + `,"ttl":0`), both, nil, "ttl must be an integer from 1 to 255."},
		{"ttl 256", specWith(okProfile + `,"ttl":256`), both, nil, "ttl must be an integer from 1 to 255."},
		{"luaPayload space", specWith(okProfile + `,"luaPayload":"all, quic"`), z2, z1, "luaPayload must contain comma-separated payload names."},
		{"luaPayload empty item", specWith(okProfile + `,"luaPayload":"all,,quic"`), z2, z1, "luaPayload must contain comma-separated payload names."},
		{"missing prefix", specWith(okProfile + `,"hostlist":["list-general.txt"]`), both, nil, "Path prefix is not lists:/user:/bin: : list-general.txt"},
		{"prefix case", specWith(okProfile + `,"ipset":["Bin:ipset.txt"]`), nil, both, ""},
		{"seqovlPattern no prefix", specWith(`"tcp":"443","desync":["multisplit"],"seqovl":1,"seqovlPattern":"0x00"`), z1, z2, "seqovlPattern must be a bin:/lists:/user: path: 0x00"},
		{"fake without blob", specWith(`"tcp":"80","payload":"http_req","desync":["fake"]`), z2, z1, "Strategy fake has no blob for payload group 'http_req'."},
		{"tls fake without tls slot", specWith(`"tcp":"443","l7":"tls","desync":["fake"]`), z2, z1, "Strategy fake has no blob for payload group 'tls_client_hello'."},
		{"parse: defaultTlsFake removed", specWith(okProfile + `,"defaultTlsFake":false`), both, nil, `unknown field "defaultTlsFake"`},
		{"unknown fake with tcp and udp", specWith(`"tcp":"443","udp":"443","payload":"all","desync":["fake"],"fake":{"unknown":["bin:u.bin"]}`), z2, z1, "Fake slot 'unknown' needs a profile with only tcp or only udp."},
		{"blob ref without file", specWith(`"tcp":"80","payload":"http_req","desync":["fake"],"fake":{"http":["bin:"]}`), z2, z1, "Blob ref has no file name: bin:"},
	}
	p := goldenPaths
	gf := GameFilter{TCP: "12", UDP: "12"}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, perr := Parse([]byte(c.json))
			for _, eng := range c.fail {
				err := perr
				if err == nil {
					_, err = Build(s, eng, p, gf)
				}
				if err == nil {
					t.Errorf("%s: no error", eng)
				} else if !strings.Contains(err.Error(), c.wantErr) {
					t.Errorf("%s: error %q, want %q", eng, err, c.wantErr)
				}
			}
			for _, eng := range c.pass {
				if perr != nil {
					t.Fatalf("parse: %v", perr)
				}
				if _, err := Build(s, eng, p, gf); err != nil {
					t.Errorf("%s: %v", eng, err)
				}
			}
		})
	}
}

// TestFakePayloadFilters checks the Lua payload filters of winws2 fakes in a
// group of more than one slot.
func TestFakePayloadFilters(t *testing.T) {
	cases := []struct {
		name    string
		profile string
		want    []string
		absent  []string
	}{
		{"tcp all", `"tcp":"443","payload":"all","desync":["fake"],"fake":{"tls":["bin:a.bin"],"http":["bin:a.bin"],"unknown":["bin:u.bin"],"quic":["bin:q.bin"]}`,
			[]string{"--lua-desync=fake:payload=tls_client_hello,http_req:blob=a ", "--lua-desync=fake:payload=~empty,http_req,tls_client_hello:blob=u"},
			[]string{"blob=q"}},
		{"udp all", `"udp":"443","payload":"all","desync":["fake"],"fake":{"quic":["bin:q.bin"],"unknownUdp":["bin:u.bin"],"tls":["bin:a.bin"]}`,
			[]string{"--lua-desync=fake:payload=quic_initial:blob=q", "--lua-desync=fake:payload=~empty,quic_initial,wireguard_initiation,dht,discord_ip_discovery,stun:blob=u"},
			[]string{"blob=a"}},
		{"discord and stun", `"udp":"443","l7":"discord,stun","desync":["fake"],"fake":{"discord":["bin:d.bin","bin:s.bin"],"stun":["bin:s.bin"]}`,
			[]string{"--lua-desync=fake:payload=discord_ip_discovery:blob=d", "--lua-desync=fake:blob=s"},
			nil},
		{"tls_mod only on tls slot", `"tcp":"443","payload":"all","tlsMod":"rnd","desync":["fake"],"fake":{"tls":["bin:a.bin"],"http":["bin:a.bin"],"unknown":["!"]}`,
			[]string{"--lua-desync=fake:payload=tls_client_hello:blob=a:tls_mod=rnd ", "--lua-desync=fake:payload=http_req:blob=a ", "blob=tls_clienthello_www_google_com "},
			nil},
		{"tls_mod only on last tls ref", `"tcp":"443","l7":"tls","tlsMod":"rnd","desync":["fake"],"fake":{"tls":["bin:a.bin","bin:b.bin"]}`,
			[]string{"--lua-desync=fake:blob=a ", "--lua-desync=fake:blob=b:tls_mod=rnd "},
			nil},
		{"no tls_mod without tls slot", `"tcp":"80","l7":"http","tlsMod":"rnd","desync":["fake"],"fake":{"http":["bin:h.bin"]}`,
			nil,
			[]string{"tls_mod"}},
		{"luaPayload wins", `"udp":"443","payload":"all","luaPayload":"all","desync":["fake"],"fake":{"unknownUdp":["bin:u.bin"]}`,
			[]string{"--lua-desync=fake:payload=all:blob=u"},
			[]string{"~empty"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, err := Parse([]byte(specWith(c.profile)))
			if err != nil {
				t.Fatal(err)
			}
			lines, err := Build(s, Winws2, goldenPaths, GameFilter{TCP: "12", UDP: "12"})
			if err != nil {
				t.Fatal(err)
			}
			got := lines[len(lines)-1] + " "
			for _, w := range c.want {
				if !strings.Contains(got, w) {
					t.Errorf("no %q in\n%s", w, got)
				}
			}
			for _, a := range c.absent {
				if strings.Contains(got, a) {
					t.Errorf("%q in\n%s", a, got)
				}
			}
		})
	}
}

// TestBadseq checks the badseq defaults of winws (nfq/params.h).
func TestBadseq(t *testing.T) {
	cases := []struct {
		name   string
		extra  string
		z1, z2 string
		absent string
	}{
		{"default", ``, "--dpi-desync-fooling=badseq --", ":tcp_seq=-10000:tcp_ack=-66000", "badseq-increment"},
		{"set", `,"badseqIncrement":2`, "--dpi-desync-badseq-increment=2", ":tcp_seq=2:tcp_ack=-66000", ""},
		{"zero", `,"badseqIncrement":0`, "--dpi-desync-badseq-increment=0", "blob=a:tcp_ack=-66000", "tcp_seq"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, err := Parse([]byte(specWith(`"tcp":"443","l7":"tls","desync":["fake"],"fake":{"tls":["bin:a.bin"]},"fooling":["badseq"]` + c.extra)))
			if err != nil {
				t.Fatal(err)
			}
			for _, eng := range []Engine{Winws, Winws2} {
				lines, err := Build(s, eng, goldenPaths, GameFilter{TCP: "12", UDP: "12"})
				if err != nil {
					t.Fatal(err)
				}
				got := lines[len(lines)-1] + " --"
				want := c.z1
				if eng == Winws2 {
					want = c.z2
				}
				if !strings.Contains(got, want) {
					t.Errorf("%s: no %q in\n%s", eng, want, got)
				}
				if c.absent != "" && strings.Contains(got, c.absent) && (eng == Winws2) == (c.absent == "tcp_seq") {
					t.Errorf("%s: %q in\n%s", eng, c.absent, got)
				}
			}
		})
	}
}

func TestEngine(t *testing.T) {
	s, err := Parse([]byte(specWith(okProfile)))
	if err != nil {
		t.Fatal(err)
	}
	p, gf := goldenPaths, GameFilter{TCP: "12", UDP: "12"}
	if _, err := Build(s, "", p, gf); err == nil || err.Error() != "Engine is empty." {
		t.Errorf("empty engine: %v", err)
	}
	if _, err := Build(s, "winws3", p, gf); err == nil {
		t.Error("winws3: no error")
	}
	if _, err := Build(nil, Winws, p, gf); err == nil {
		t.Error("nil spec: no error")
	}
	a, _ := Build(s, "WINWS2", p, gf)
	b, _ := Build(s, Winws2, p, gf)
	if strings.Join(a, "\n") != strings.Join(b, "\n") {
		t.Error("engine name must ignore case")
	}
}

func TestLoadFile(t *testing.T) {
	if _, err := LoadFile(""); err == nil {
		t.Error("empty path: no error")
	}
	missing := filepath.Join(t.TempDir(), "none.json")
	if _, err := LoadFile(missing); err == nil || !strings.Contains(err.Error(), "Strategy JSON not found") {
		t.Errorf("missing file: %v", err)
	}
	bom := filepath.Join(t.TempDir(), "bom.json")
	if err := os.WriteFile(bom, []byte("\xef\xbb\xbf"+specWith(okProfile)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(bom); err != nil {
		t.Errorf("BOM: %v", err)
	}
}
