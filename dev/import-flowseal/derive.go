package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/NONERUN/zapman/internal/spec"
)

// Our strategy files have two additions to the Flowseal .bat files (from
// v0.1). winws2 needs them, and for winws they do not change the result:
//
//   - --filter-l7 on each target with no --filter-l7 and no
//     --dpi-desync-any-protocol, so that winws2 can find the payload type.
//   - --dpi-desync-fake-tls=! (the stock hello) in a profile with
//     --dpi-desync-fake-tls-mod, no --dpi-desync-fake-tls and no
//     --dpi-desync-any-protocol. winws uses the stock hello in this profile
//     implicitly; winws2 has no implicit TLS fake. A profile with
//     --dpi-desync-any-protocol gets no stock hello: it is for all
//     payloads, and the hello would go to them (the Game Filter profiles).
//
// The importer derives the additions with the rules below. It adds them to
// the rules, and it adds them to the .bat argv before the compare. A target
// that fits no l7 rule is an error.

// l7Rule is one rule of the derived --filter-l7.
//
//	Filter           Ports                               Fake HTTP  --filter-l7
//	--filter-udp     443                                 -          quic
//	--filter-tcp     80, 443, 2053, 2083, 2087, 2096,    no         tls
//	                 8443 (any of them)
//	--filter-tcp     the same, with 80                   yes        tls,http
type l7Rule struct {
	udp      bool            // the rule is for --filter-udp, else --filter-tcp
	ports    map[string]bool // each port of the filter must be in ports
	need     string          // a port that the filter must have; empty: none
	fakeHTTP bool            // the action must have fake.http (TCP only)
	l7       []string
}

// webPorts are the TCP ports of the TLS and HTTP targets of Flowseal.
var webPorts = map[string]bool{"80": true, "443": true, "2053": true, "2083": true, "2087": true, "2096": true, "8443": true}

// l7Rules is the table of the derived --filter-l7.
var l7Rules = []l7Rule{
	{udp: true, ports: map[string]bool{"443": true}, l7: []string{"quic"}},
	{ports: webPorts, l7: []string{"tls"}},
	{ports: webPorts, need: "80", fakeHTTP: true, l7: []string{"tls", "http"}},
}

// match tells if the rule is for the target and the fakes.
func (r l7Rule) match(t *spec.Target, f *spec.Fakes) bool {
	ports, other := t.TCP, t.UDP
	if r.udp {
		ports, other = t.UDP, t.TCP
	}
	if len(ports) == 0 || len(other) > 0 {
		return false
	}
	for _, p := range ports {
		if !r.ports[p] {
			return false
		}
	}
	if r.need != "" && !slices.Contains(ports, r.need) {
		return false
	}
	return r.udp || r.fakeHTTP == (len(f.HTTP) > 0)
}

// derive adds the derived additions to the target and the fakes. It gives
// the args that the additions add to the winws argv.
func derive(t *spec.Target, a *spec.Action, f *spec.Fakes) ([]arg, error) {
	if t.AnyProtocol {
		return nil, nil
	}
	var out []arg
	if len(t.L7) == 0 {
		var found []l7Rule
		for _, r := range l7Rules {
			if r.match(t, f) {
				found = append(found, r)
			}
		}
		if len(found) != 1 {
			return nil, fmt.Errorf("%d rules of the derived --filter-l7 fit the target (--filter-tcp=%s --filter-udp=%s, fake http %v); one must fit",
				len(found), listText(t.TCP), listText(t.UDP), len(f.HTTP) > 0)
		}
		t.L7 = found[0].l7
		out = append(out, arg{Flag: "--filter-l7", Value: strings.Join(t.L7, ","), HasValue: true})
	}
	if a.TLSMod != "" && len(f.TLS) == 0 {
		f.TLS = []string{"!"}
		out = append(out, arg{Flag: "--dpi-desync-fake-tls", Value: "!", HasValue: true})
	}
	return out, nil
}

// withDerived gives the .bat argv with the derived args. The stock hello
// goes before the first --dpi-desync-fake-tls-mod, as in the build.
func withDerived(a *argv, derived [][]arg) *argv {
	out := &argv{WFTCP: a.WFTCP, WFUDP: a.WFUDP}
	for i, g := range a.Groups {
		g = slices.Clone(g)
		for _, d := range derived[i] {
			at := len(g)
			if d.Flag == "--dpi-desync-fake-tls" {
				at = slices.IndexFunc(g, func(x arg) bool { return x.Flag == "--dpi-desync-fake-tls-mod" })
			}
			g = slices.Insert(g, at, d)
		}
		out.Groups = append(out.Groups, g)
	}
	return out
}
