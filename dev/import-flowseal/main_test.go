package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/NONERUN/zapman/internal/spec"
)

// root is the repository root.
var root = filepath.Join("..", "..")

// testTag is the upstream tag of the import test.
const testTag = "1.10.4"

// knownDiff are the differences of the flowseal/ files from the .bat files
// of testTag. They come from v0.1: the targets have --filter-l7 for the
// winws2 payload groups, and a fake-tls-mod with no fake-tls file has the
// stock hello "!", because winws2 has no implicit TLS fake.
var knownDiff = regexp.MustCompile(`^profile \d+: only ours: (--filter-l7=[a-z,]+|--dpi-desync-fake-tls=!)$`)

// TestImport imports the .bat files of testTag from git. Each file must
// map and pass the round trip, and each flowseal/ file must be the same
// as its .bat file but for knownDiff. The test is skipped when git or the
// tag is not available.
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
		if r.Status != statusSame && r.Status != statusChanged {
			t.Errorf("%s: status %s: %v", r.Name, r.Status, r.Err)
		}
		for _, d := range r.Diff {
			if !knownDiff.MatchString(d) {
				t.Errorf("%s: %s", r.Name, d)
			}
		}
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

	r, err := m.rule(words("--filter-udp=19294-19344,50000-50100 --filter-l7=discord,stun --dpi-desync=fake " +
		"--dpi-desync-fake-discord=bin/ACTIVE_DISCORD_UDP.bin --dpi-desync-fake-stun=bin/ACTIVE_DISCORD_UDP.bin --dpi-desync-repeats=6"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Target.Name != "discord-voice" || r.Action.Name != "fake-discord-6" {
		t.Errorf("got %s / %s, want discord-voice / fake-discord-6", mustJSON(r.Target), mustJSON(r.Action))
	}

	r, err = m.rule(words("--filter-tcp=%GameFilterTCP% --ipset=lists/ipset-all.txt --ipset-exclude=lists/ipset-exclude.txt " +
		"--ipset-exclude=lists/ipset-exclude-user.txt --hostlist=lists/list-general.txt --hostlist=lists/list-general-user.txt " +
		"--dpi-desync=fake --dpi-desync-fake-tls=0x00 --dpi-desync-cutoff=n3"))
	if err != nil {
		t.Fatal(err)
	}
	if got := mustJSON(r.Target); got != `{"tcp":["$gameFilter"],"lists":["general","ipset-all"],"exclude":["exclude-ips"]}` {
		t.Errorf("target: %s", got)
	}
	if got := mustJSON(r.Action); got != `{"desync":["fake"],"cutoff":3,"fake":{"tls":["0x00"]}}` {
		t.Errorf("action: %s", got)
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
	} {
		_, err := m.rule(words(tc.args))
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
