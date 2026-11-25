package spec

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestGames checks the game profiles of the stock catalog on "general
// (ALT11)" with Game Filter off. The golden files have the full argv; this
// test keeps the properties that were verified in the games, so that an
// -update of the golden files cannot lose them.
func TestGames(t *testing.T) {
	root := filepath.Join("..", "..")
	cat, err := LoadCatalog(filepath.Join(root, "strategies"), filepath.Join(root, "bin"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := cat.Strategy("general (ALT11)")
	if err != nil {
		t.Fatal(err)
	}
	gf := GameFilter{TCP: "12", UDP: "12"}
	build := func(eng Engine) []string {
		lines, err := Build(st, cat.Games, eng, goldenPaths, gf)
		if err != nil {
			t.Fatalf("%s: %v", eng, err)
		}
		return lines
	}
	check := func(ok bool, msg string) {
		t.Helper()
		if !ok {
			t.Error(msg)
		}
	}
	fakes := regexp.MustCompile(`--lua-desync=fake:`)

	z2 := build(Winws2)
	all := strings.Join(z2, "\n")
	var profiles []string
	for _, l := range z2 {
		if strings.HasPrefix(l, "--filter-") {
			profiles = append(profiles, l)
		}
	}
	if len(profiles) < 2 {
		t.Fatalf("winws2: %d profiles", len(profiles))
	}
	check(regexp.MustCompile(`--wf-udp-out=\S*5000-8000`).MatchString(all), "Sky ports must be captured with Game Filter off.")
	sky, ws := profiles[0], profiles[1]
	check(strings.Contains(sky, `--ipset="C:\zapman\user\ipset-sky.txt"`), "Sky UDP must use its own user list.")
	check(strings.Contains(sky, "--out-range=-n4"), "Sky UDP must stop fake injection after four packets.")
	check(strings.Contains(sky, "--lua-desync=fake:payload=all:blob=quic_initial_www_google_com:repeats=6:ip_ttl=8"), "Sky UDP must have the verified fake command.")
	check(len(fakes.FindAllString(sky, -1)) == 1, "Sky UDP must not get an extra fake.")
	check(strings.Contains(ws, "--hostlist-domains=live.ws.radiance.thatgamecompany.com"), "Sky TCP must only target the WebSocket endpoint.")
	check(strings.Contains(ws, "--lua-desync=multisplit:payload=all:pos=1"), "WebSocket must process encrypted application payloads.")
	check(!strings.Contains(ws, "--out-range="), "WebSocket processing must continue after the handshake.")

	var photon []string
	for _, p := range profiles {
		if strings.Contains(p, "--filter-udp=5055-5058,27000-27002") {
			photon = append(photon, p)
		}
	}
	if len(photon) != 2 {
		t.Fatalf("winws2: %d Photon profiles, want 2 (sker and photon)", len(photon))
	}
	sker := photon[0]
	check(strings.Contains(sker, `--ipset="C:\zapman\user\ipset-sker.txt"`), "The sker profile must come before the common Photon profile.")
	check(strings.Contains(sker, "--lua-desync=fake:payload=all:blob=ACTIVE_GAME_UDP:repeats=6:ip_ttl=8"), "Sker fakes must not reach the Photon server (TTL 8).")
	check(len(fakes.FindAllString(sker, -1)) == 1, "The sker profile must not get an extra fake.")

	z1 := strings.Join(build(Winws), "\n")
	check(strings.Contains(z1, "--dpi-desync-ttl=8"), "winws must get the Sky fake TTL.")
	check(strings.Contains(z1, "--dpi-desync-any-protocol=1"), "winws must process unknown Sky payloads.")
	check(!strings.Contains(z1, "--dpi-desync-cutoff=none"), "winws must not get the none cutoff.")
}
