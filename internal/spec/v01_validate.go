package spec

import (
	"errors"
	"fmt"
	"strings"
)

// v01Fooling are the allowed fooling names.
var v01Fooling = map[string]bool{"ts": true, "md5sig": true, "badseq": true}

// v01L7 are the --filter-l7 names of winws (nfqws.c parse_l7_list). winws2
// knows them too (protocol.c l7proto_name). Both engines compare with case.
var v01L7 = map[string]bool{
	"http": true, "tls": true, "quic": true, "wireguard": true,
	"dht": true, "discord": true, "stun": true, "unknown": true,
}

// validate checks the spec. The rules are the same for both engines. Names
// and path prefixes are case-sensitive.
func (s *V01Spec) validate() error {
	if s.WF == nil {
		return errors.New("Strategy JSON has no wf.")
	}
	if len(s.Profiles) == 0 {
		return errors.New("Strategy has no profiles.")
	}
	for i := range s.Profiles {
		if err := s.Profiles[i].validate(); err != nil {
			return err
		}
	}
	return nil
}

func (pr *V01Profile) validate() error {
	if len(pr.Desync) == 0 {
		return errors.New("Strategy profile has no desync attacks.")
	}
	for _, d := range pr.Desync {
		if _, ok := v01Attacks[d]; !ok {
			return fmt.Errorf("Unknown desync attack '%s'.", d)
		}
	}
	for _, f := range pr.Fooling {
		if !v01Fooling[f] {
			return fmt.Errorf("Unknown fooling '%s'.", f)
		}
	}
	if pr.TTL != nil && (*pr.TTL < 1 || *pr.TTL > 255) {
		return errors.New("ttl must be an integer from 1 to 255.")
	}
	if pr.Cutoff != "" && pr.Cutoff != "none" {
		if _, ok := matchCutoffN(pr.Cutoff); !ok {
			return fmt.Errorf("cutoff must be nN or none: %s", pr.Cutoff)
		}
	}
	if pr.L7 != "" {
		for _, n := range strings.Split(pr.L7, ",") {
			if !v01L7[n] {
				return fmt.Errorf("Unknown l7 '%s'.", n)
			}
		}
		// The winws2 build has one payload group for quic and one for tls.
		if pr.hasL7("quic") && pr.hasL7("tls") {
			return errors.New("l7 cannot have both quic and tls.")
		}
	}
	for _, refs := range []StringList{pr.Hostlist, pr.HostlistExclude, pr.Ipset, pr.IpsetExclude} {
		for _, ref := range refs {
			if !isFileRef(ref) {
				return fmt.Errorf("Path prefix is not lists:/user:/bin: : %s", ref)
			}
		}
	}
	for _, list := range []StringList{pr.HostlistDomains, pr.HostlistExcludeDomains} {
		for _, d := range list {
			if isBlank(d) {
				return errors.New("Domain list has an empty item.")
			}
		}
	}
	if pr.SeqovlPattern != "" && !isFileRef(pr.SeqovlPattern) {
		return fmt.Errorf("seqovlPattern must be a bin:/lists:/user: path: %s", pr.SeqovlPattern)
	}
	for _, d := range v01FakeSlots {
		for _, ref := range d.get(&pr.Fake).Refs {
			if !isFakeValue(ref) {
				return fmt.Errorf("Fake value is not valid: '%s'. Use !, 0x<hex> or a bin:/lists:/user: path without spaces.", ref)
			}
		}
	}
	return nil
}

// hasL7 tells if the l7 list has the name.
func (pr *V01Profile) hasL7(name string) bool {
	for _, n := range strings.Split(pr.L7, ",") {
		if n == name {
			return true
		}
	}
	return false
}

// isFakeValue tells if ref is "!", 0x<hex> or a prefixed path, without
// spaces.
func isFakeValue(ref string) bool {
	if ref == "!" || isHexRef(ref) {
		return true
	}
	return isFileRef(ref) && !strings.ContainsAny(ref, " \t\r\n")
}

// isHexRef tells if ref is 0x and hex digits.
func isHexRef(ref string) bool {
	return strings.HasPrefix(ref, "0x") && isHex(ref[2:])
}
