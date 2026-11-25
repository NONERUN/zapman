package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NONERUN/zapman/internal/spec"
)

// root is the repository root.
var root = filepath.Join("..", "..")

// testTag is the upstream tag of the import test.
const testTag = "1.10.4"

// TestImport imports the .bat files of testTag from git. Each file must
// map, pass the round trip and be the same as its flowseal/ file (with
// the derived additions). The test is skipped when git or the tag is not
// available.
func TestImport(t *testing.T) {
	if err := exec.Command("git", "-C", root, "rev-parse", "--verify", "--quiet", testTag+"^{commit}").Run(); err != nil {
		t.Skipf("git or the upstream tag %s is not available: %v", testTag, err)
	}
	var log bytes.Buffer
	res, err := run(options{Git: root, Ref: testTag, Dir: filepath.Join(root, "strategies")}, &log)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(log.String())
	if len(res) != 22 {
		t.Errorf("got %d results, want 22", len(res))
	}
	for _, r := range res {
		if r.Status != statusSame {
			t.Errorf("%s: status %s: %v %v", r.Name, r.Status, r.Err, r.Diff)
		}
	}
	if c := exitCode(res); c != 0 {
		t.Errorf("exit code %d, want 0", c)
	}
}

func TestParseBat(t *testing.T) {
	bat := "@echo off\r\nset \"BIN=%~dp0bin\\\"\r\nset \"LISTS=%~dp0lists\\\"\r\n" +
		"start \"zapret: %~n0\" /min \"%BIN%winws.exe\" --wf-tcp=443,%GameFilterTCP% --wf-udp=443 ^\r\n" +
		"--filter-tcp=443,%GameFilterTCP% --hostlist=\"%LISTS%list general.txt\" --dpi-desync=fake --dpi-desync-fake-tls=^! --new ^\r\n" +
		"--filter-udp=443 --dpi-desync-fake-quic=\"%BIN%quic.bin\"\r\n"
	a, err := parseBat([]byte(bat))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(a.WFTCP, ","); got != "443,%GameFilterTCP%" {
		t.Errorf("wf-tcp: %s", got)
	}
	if len(a.Groups) != 2 {
		t.Fatalf("got %d profiles", len(a.Groups))
	}
	want := "--filter-tcp=443,%GameFilterTCP% --hostlist=lists/list general.txt --dpi-desync=fake --dpi-desync-fake-tls=!"
	if got := argsText(a.Groups[0]); got != want {
		t.Errorf("profile 1:\ngot  %s\nwant %s", got, want)
	}
	if got := argsText(a.Groups[1]); got != "--filter-udp=443 --dpi-desync-fake-quic=bin/quic.bin" {
		t.Errorf("profile 2: %s", got)
	}
}

func TestParseBatErrors(t *testing.T) {
	for _, tc := range []struct{ bat, want string }{
		{"echo hi\n", "no line starts winws.exe"},
		{"start \"%~dp0bin\\winws2.exe\" --new\n", "winws2"},
		{"start winws.exe --wf-tcp=443 --filter-tcp=%PORTS%\n", "unknown variable %PORTS%"},
		{"start winws.exe --wf-tcp=443 --filter-tcp=443 --new --new --dpi-desync=fake\n", "profile 2 is empty"},
		{"start winws.exe --filter-tcp=443 --wf-udp=443\n", "after a profile flag"},
	} {
		_, err := parseBat([]byte(tc.bat))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%q: got %v, want %q", tc.bat, err, tc.want)
		}
	}
}

func TestMapRule(t *testing.T) {
	cat, err := spec.LoadCatalog(filepath.Join(root, "strategies"), "")
	if err != nil {
		t.Fatal(err)
	}
	m := newMapper(cat)
	words := func(s string) []arg {
		a, err := parseArgs(strings.Fields(s), nil)
		if err != nil {
			t.Fatal(err)
		}
		return a.Groups[0]
	}

	r, _, err := m.rule(words("--filter-udp=19294-19344,50000-50100 --filter-l7=discord,stun --dpi-desync=fake " +
		"--dpi-desync-fake-discord=bin/ACTIVE_DISCORD_UDP.bin --dpi-desync-fake-stun=bin/ACTIVE_DISCORD_UDP.bin --dpi-desync-repeats=6"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Target.Name != "discord-voice" || r.Action.Name != "fake-discord-6" {
		t.Errorf("got %s / %s, want discord-voice / fake-discord-6", mustJSON(r.Target), mustJSON(r.Action))
	}

	r, d, err := m.rule(words("--filter-tcp=%GameFilterTCP% --dpi-desync-any-protocol=1 --ipset=lists/ipset-all.txt --ipset-exclude=lists/ipset-exclude.txt " +
		"--ipset-exclude=lists/ipset-exclude-user.txt --hostlist=lists/list-general.txt --hostlist=lists/list-general-user.txt " +
		"--dpi-desync=fake --dpi-desync-fake-tls-mod=none --dpi-desync-cutoff=n3"))
	if err != nil {
		t.Fatal(err)
	}
	if got := mustJSON(r.Target); !strings.Contains(got, `"tcp":["$gameFilter"]`) ||
		!strings.Contains(got, `"lists":["general","ipset-all"],"exclude":["exclude-ips"]`) || strings.Contains(got, "l7") {
		t.Errorf("target: %s", got)
	}
	if got := mustJSON(r.Action); got != `{"desync":["fake"],"cutoff":3,"tlsMod":"none"}` {
		t.Errorf("action: %s", got)
	}
	if len(d) > 0 {
		t.Errorf("any-protocol: derived %s", argsText(d))
	}

	// The derived additions.
	for _, tc := range []struct{ args, l7, derived string }{
		{"--filter-udp=443 --dpi-desync=fake --dpi-desync-fake-quic=0x00", `"l7":["quic"]`, "--filter-l7=quic"},
		{"--filter-tcp=443,2053 --dpi-desync=fake", `"l7":["tls"]`, "--filter-l7=tls"},
		{"--filter-tcp=80,443 --dpi-desync=fake --dpi-desync-fake-http=0x00", `"l7":["tls","http"]`, "--filter-l7=tls,http"},
		{"--filter-tcp=443 --dpi-desync=fake --dpi-desync-fake-tls-mod=none", `"l7":["tls"]`, "--filter-l7=tls --dpi-desync-fake-tls=!"},
		{"--filter-tcp=443 --filter-l7=tls --dpi-desync=fake --dpi-desync-fake-tls=0x00 --dpi-desync-fake-tls-mod=none", `"l7":["tls"]`, ""},
	} {
		r, d, err := m.rule(words(tc.args))
		if err != nil {
			t.Errorf("%s: %v", tc.args, err)
			continue
		}
		if got := mustJSON(r.Target); r.Target.Name == "" && !strings.Contains(got, tc.l7) {
			t.Errorf("%s: target %s, want %s", tc.args, got, tc.l7)
		}
		if got := argsText(d); got != tc.derived {
			t.Errorf("%s: derived %q, want %q", tc.args, got, tc.derived)
		}
	}

	for _, tc := range []struct{ args, want string }{
		{"--filter-tcp=443 --dpi-desync=fake --dpi-desync-foo=1", "unknown flag --dpi-desync-foo"},
		{"--filter-tcp=443 --dpi-desync=fake --dpi-desync-repeats=6 --dpi-desync-repeats=7", "--dpi-desync-repeats is two times"},
		{"--filter-tcp=443 --dpi-desync=fake --dpi-desync-cutoff=d10", "only nN is supported"},
		{"--filter-tcp=443 --dpi-desync=fake --dpi-desync-any-protocol=0", "only 1 is supported"},
		{"--filter-tcp=%GameFilterUDP% --dpi-desync=fake", "not the Game Filter variable"},
		{"--filter-tcp=443 --dpi-desync=fake --dpi-desync-fake-tls=bin/none.bin", "none.bin is not in"},
		{"--filter-tcp=443 --dpi-desync=fake --dpi-desync-fake-quic=bin/ACTIVE_NONE.bin", "active fake NONE"},
		{"--filter-tcp=443 --hostlist=lists/list-google.txt --hostlist=lists/other.txt --dpi-desync=fake", "no bundles"},
		{"--filter-tcp=%GameFilterTCP% --dpi-desync=fake", "0 rules of the derived --filter-l7"},
		{"--filter-udp=443,50000 --dpi-desync=fake", "0 rules of the derived --filter-l7"},
		{"--filter-tcp=443 --dpi-desync=fake --dpi-desync-fake-http=0x00", "0 rules of the derived --filter-l7"},
		{"--filter-tcp=443 --filter-udp=443 --dpi-desync=fake", "0 rules of the derived --filter-l7"},
	} {
		_, _, err := m.rule(words(tc.args))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want %q", tc.args, err, tc.want)
		}
	}
}

func TestDiffArgv(t *testing.T) {
	parse := func(s string) *argv {
		a, err := parseArgs(strings.Fields(s), nil)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	ours := parse("--wf-tcp=80,443 --wf-udp=443 --filter-tcp=443 --dpi-desync=fake --dpi-desync-fake-tls=a --dpi-desync-fake-tls-mod=m --dpi-desync-fake-tls=b")
	same := parse("--wf-tcp=443,80 --wf-udp=443 --dpi-desync-fake-tls=a --dpi-desync-fake-tls-mod=m --dpi-desync=fake --dpi-desync-fake-tls=b --filter-tcp=443")
	if d := diffArgv(ours, same); len(d) > 0 {
		t.Errorf("got differences: %v", d)
	}
	other := parse("--wf-tcp=443 --wf-udp=443 --filter-tcp=443 --dpi-desync=fake --dpi-desync-fake-tls=a --dpi-desync-fake-tls=b --dpi-desync-fake-tls-mod=m")
	want := []string{
		"--wf-tcp: only ours: 80; only Flowseal: -",
		"profile 1: fake-tls order: ours --dpi-desync-fake-tls=a --dpi-desync-fake-tls-mod=m --dpi-desync-fake-tls=b; " +
			"Flowseal --dpi-desync-fake-tls=a --dpi-desync-fake-tls=b --dpi-desync-fake-tls-mod=m",
	}
	if got := diffArgv(ours, other); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
