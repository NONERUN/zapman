package spec

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Engine is the zapret exe.
type Engine string

// Engines.
const (
	Winws  Engine = "winws"  // zapret, winws.exe
	Winws2 Engine = "winws2" // zapret2, winws2.exe
)

// attackCaps tells which profile fields go into the winws2 Lua call of an
// attack.
type attackCaps struct {
	repeats       bool // repeats=
	fooling       bool // tcp_ts, tcp_md5, tcp_seq/tcp_ack, ip_ttl
	split         bool // pos=, seqovl=, seqovl_pattern=
	hostfakesplit bool // hostfakesplitMod pieces
	fakedsplit    bool // pattern=
}

// v01Attacks are the allowed desync names. Lookup ignores case.
var v01Attacks = map[string]attackCaps{
	"fake":          {repeats: true, fooling: true},
	"multisplit":    {split: true},
	"multidisorder": {split: true, fooling: true},
	"hostfakesplit": {repeats: true, fooling: true, hostfakesplit: true},
	"fakedsplit":    {repeats: true, fooling: true, split: true, fakedsplit: true},
	"syndata":       {repeats: true, fooling: true},
}

// payloadSlots maps a winws2 payload group to the fake slots it uses.
// Other groups use all slots.
var payloadSlots = map[string][]string{
	"tls_client_hello":          {"tls"},
	"http_req":                  {"http"},
	"quic_initial":              {"quic"},
	"discord_ip_discovery,stun": {"discord", "stun"},
}

// Build gives the argv lines of the spec for the engine: the --wf line, for
// winws2 the --lua-init line and the --blob line, then one line per profile.
// Each profile line but the last ends with " --new".
func Build(s *V01Spec, engine Engine, p Paths, gf GameFilter) ([]string, error) {
	if s == nil {
		return nil, errors.New("Strategy spec is empty.")
	}
	// ValidateSet of the module ignores case.
	switch Engine(strings.ToLower(string(engine))) {
	case Winws:
		return buildWinws(s, p, gf)
	case Winws2:
		return buildWinws2(s, p, gf)
	case "":
		return nil, errors.New("Engine is empty.")
	}
	return nil, fmt.Errorf("Unknown engine '%s'.", engine)
}

// profiles is @($spec.profiles): a missing or null list is one null profile.
func (s *V01Spec) profiles() []V01Profile {
	if s.Profiles == nil {
		return []V01Profile{{}}
	}
	return s.Profiles
}

func (pr *V01Profile) desync() ([]string, error) {
	if len(pr.Desync) < 1 {
		return nil, errors.New("Strategy profile has no desync attacks.")
	}
	for _, n := range pr.Desync {
		if _, ok := v01Attacks[strings.ToLower(n)]; !ok {
			return nil, fmt.Errorf("Unknown desync attack '%s'.", n)
		}
	}
	return pr.Desync, nil
}

func (pr *V01Profile) ttl() (int, bool, error) {
	if pr.TTL == nil {
		return 0, false, nil
	}
	if *pr.TTL < 1 || *pr.TTL > 255 {
		return 0, false, errors.New("ttl must be an integer from 1 to 255.")
	}
	return *pr.TTL, true, nil
}

// winws badseq defaults (nfq/params.h): when the increment flags are not
// given, badseq adds -10000 to seq and -66000 to ack.
const (
	badseqDefault = -10000
	badackDefault = -66000
)

// badseqIncrement gives badseqIncrement and true when JSON sets it.
func (pr *V01Profile) badseqIncrement() (int, bool) {
	if pr.BadseqIncrement == nil {
		return badseqDefault, false
	}
	return *pr.BadseqIncrement, true
}

func (pr *V01Profile) slot(name string) *FakeSlot {
	for _, d := range v01FakeSlots {
		if strings.EqualFold(d.name, name) {
			return d.get(&pr.Fake)
		}
	}
	return nil
}

// tlsRefs is Get-ZapretSpecTlsFakeRefs.
func (pr *V01Profile) tlsRefs() []string {
	var out []string
	for _, r := range pr.Fake.TLS.Refs {
		if !isBlank(r) {
			out = append(out, r)
		}
	}
	return out
}

// payloadGroups is Get-ZapretSpecPayloadGroups.
func (pr *V01Profile) payloadGroups() []string {
	if !isBlank(pr.Payload) {
		return []string{pr.Payload}
	}
	if pr.AnyProtocol {
		return []string{"all"}
	}
	l7 := pr.L7
	switch {
	case containsFold(l7, "tls") && containsFold(l7, "http"):
		return []string{"tls_client_hello", "http_req"}
	case containsFold(l7, "quic"):
		return []string{"quic_initial"}
	case containsFold(l7, "discord") || containsFold(l7, "stun"):
		return []string{"discord_ip_discovery,stun"}
	case containsFold(l7, "tls"):
		return []string{"tls_client_hello"}
	case containsFold(l7, "http"):
		return []string{"http_req"}
	}
	return []string{""}
}

// slotPayload is the winws2 Lua payload filter of each fake slot in a group
// with more than one slot (for example "all"). It copies the fake choice of
// winws (desync.c): TCP http -> fake-http, tls -> fake-tls, other data ->
// fake-unknown; UDP quic -> fake-quic, discord -> fake-discord, stun ->
// fake-stun, other data but wireguard and dht -> fake-unknown-udp.
var slotPayload = map[string]string{
	"quic":       "quic_initial",
	"tls":        "tls_client_hello",
	"http":       "http_req",
	"discord":    "discord_ip_discovery",
	"stun":       "stun",
	"unknown":    "~empty,http_req,tls_client_hello",
	"unknownUdp": "~empty,quic_initial,wireguard_initiation,dht,discord_ip_discovery,stun",
}

// slotL4 is the protocol of the packets that winws sends the fake slot for.
var slotL4 = map[string]string{
	"quic": "udp", "tls": "tcp", "http": "tcp", "discord": "udp",
	"stun": "udp", "unknown": "tcp", "unknownUdp": "udp",
}

// fakeCall is one winws2 fake call: a blob ref and its Lua payload filter.
// An empty payload means the filter of the group. tlsMod tells if the call
// gets tls_mod.
type fakeCall struct {
	ref     string
	payload string
	tlsMod  bool
}

// l4 gives "tcp" or "udp" when the profile has only that filter, else "".
func (pr *V01Profile) l4() string {
	switch {
	case !isBlank(pr.TCP) && isBlank(pr.UDP):
		return "tcp"
	case isBlank(pr.TCP) && !isBlank(pr.UDP):
		return "udp"
	}
	return ""
}

// fakeCallsForGroup gives the fake calls of the group. A group of one slot
// (tls_client_hello, http_req, quic_initial) uses the refs of that slot. In
// other groups each slot gets the payload filter of slotPayload, so a fake
// goes only to the payload type that winws sends it for. When the slots of
// one ref have positive filters, one call gets all of them. luaPayload, when
// set, replaces these filters. There is no implicit TLS hello: use "!" in a
// slot to get the stock hello.
//
// tls_mod goes only on the last ref of the tls slot, and only when the call
// sends it to TLS hello packets: in the tls_client_hello group or with the
// tls_client_hello filter. winws also changes only the TLS fake
// (desync.c, fake-tls-mod). This call does not join other slots.
func (pr *V01Profile) fakeCallsForGroup(group string) ([]fakeCall, error) {
	var slots []string
	for g, s := range payloadSlots {
		if strings.EqualFold(group, g) {
			slots = s
		}
	}
	filter := len(slots) != 1 && isBlank(pr.LuaPayload)
	if slots == nil {
		for _, d := range v01FakeSlots {
			slots = append(slots, d.name)
		}
	}
	l4 := pr.l4()
	tlsLast := ""
	if refs := pr.tlsRefs(); len(refs) > 0 && !isBlank(pr.TLSMod) {
		tlsLast = refs[len(refs)-1]
	}
	var calls []fakeCall
	pos := map[string]int{} // ref (lower case) -> index of its call with a positive filter
	for _, name := range slots {
		sl := pr.slot(name)
		if !sl.Set {
			continue
		}
		payload := ""
		if filter {
			if l4 != "" && l4 != slotL4[name] {
				// winws never sends this slot on this protocol.
				continue
			}
			if l4 == "" && (name == "unknown" || name == "unknownUdp") {
				return nil, fmt.Errorf("Fake slot '%s' needs a profile with only tcp or only udp.", name)
			}
			payload = slotPayload[name]
		}
		seen := foldSet{}
		for _, r := range sl.Refs {
			if isBlank(r) || seen.has(r) {
				continue
			}
			seen.add(r)
			key := strings.ToLower(r)
			if name == "tls" && tlsLast != "" && strings.EqualFold(r, tlsLast) &&
				(strings.EqualFold(group, "tls_client_hello") || payload == "tls_client_hello") {
				calls = append(calls, fakeCall{ref: r, payload: payload, tlsMod: true})
				continue
			}
			if strings.HasPrefix(payload, "~") {
				calls = append(calls, fakeCall{ref: r, payload: payload})
				continue
			}
			if i, ok := pos[key]; ok {
				if payload != "" {
					calls[i].payload += "," + payload
				}
				continue
			}
			pos[key] = len(calls)
			calls = append(calls, fakeCall{ref: r, payload: payload})
		}
	}
	// A filter that is the same as the --payload group adds nothing.
	for i := range calls {
		if calls[i].payload != "" && sameItems(calls[i].payload, group) {
			calls[i].payload = ""
		}
	}
	return calls, nil
}

// sameItems tells if two comma lists have the same items.
func sameItems(a, b string) bool {
	x, y := strings.Split(a, ","), strings.Split(b, ",")
	if len(x) != len(y) {
		return false
	}
	set := foldSet{}
	for _, s := range x {
		set.add(s)
	}
	for _, s := range y {
		if !set.has(s) {
			return false
		}
	}
	return true
}

// needOutRange is Test-ZapretSpecNeedOutRange.
func (pr *V01Profile) needOutRange() bool {
	if strings.EqualFold(pr.Cutoff, "none") {
		return false
	}
	if _, ok := matchCutoffN(pr.Cutoff); ok {
		return true
	}
	return !isBlank(pr.TCP) || containsFold(pr.L7, "quic") || containsFold(pr.Payload, "quic") || pr.Fake.QUIC.Set
}

// commonFilters is Add-ZapretSpecCommonFilters.
func (pr *V01Profile) commonFilters(p Paths, gf GameFilter) ([]string, error) {
	var parts []string
	if !isBlank(pr.L3) {
		parts = append(parts, "--filter-l3="+pr.L3)
	}
	if !isBlank(pr.UDP) {
		parts = append(parts, "--filter-udp="+applyGameFilter(pr.UDP, gf))
	}
	if !isBlank(pr.TCP) {
		parts = append(parts, "--filter-tcp="+applyGameFilter(pr.TCP, gf))
	}
	if !isBlank(pr.L7) {
		parts = append(parts, "--filter-l7="+pr.L7)
	}
	for _, l := range []struct {
		refs StringList
		flag string
	}{
		{pr.Hostlist, "--hostlist"},
		{pr.HostlistExclude, "--hostlist-exclude"},
		{pr.Ipset, "--ipset"},
		{pr.IpsetExclude, "--ipset-exclude"},
	} {
		for _, ref := range l.refs {
			path, ok := resolveRef(ref, p)
			if !ok {
				return nil, fmt.Errorf("Path prefix is not lists:/user:/bin: : %s", ref)
			}
			parts = append(parts, l.flag+"="+quote(path))
		}
	}
	if len(pr.HostlistDomains) > 0 {
		parts = append(parts, "--hostlist-domains="+strings.Join(pr.HostlistDomains, ","))
	}
	if len(pr.HostlistExcludeDomains) > 0 {
		parts = append(parts, "--hostlist-exclude-domains="+strings.Join(pr.HostlistExcludeDomains, ","))
	}
	return parts, nil
}

func wfLine(s *V01Spec, tcpFlag, udpFlag string, gf GameFilter) (string, error) {
	if s.WF == nil {
		return "", errors.New("Strategy JSON has no wf.")
	}
	return tcpFlag + "=" + csv(s.WF.TCP, gf) + " " + udpFlag + "=" + csv(s.WF.UDP, gf), nil
}

func finishProfiles(lines []string, profiles [][]string) []string {
	for i, parts := range profiles {
		line := strings.Join(parts, " ")
		if i < len(profiles)-1 {
			line += " --new"
		}
		lines = append(lines, line)
	}
	return lines
}

// buildWinws is ConvertTo-ZapretSpecWinwsArgList.
func buildWinws(s *V01Spec, p Paths, gf GameFilter) ([]string, error) {
	wf, err := wfLine(s, "--wf-tcp", "--wf-udp", gf)
	if err != nil {
		return nil, err
	}
	var out [][]string
	for i := range s.profiles() {
		pr := &s.profiles()[i]
		parts, err := pr.commonFilters(p, gf)
		if err != nil {
			return nil, err
		}
		if !isBlank(pr.IPID) {
			parts = append(parts, "--ip-id="+pr.IPID)
		}
		desync, err := pr.desync()
		if err != nil {
			return nil, err
		}
		parts = append(parts, "--dpi-desync="+strings.Join(desync, ","))
		if pr.Repeats != nil {
			parts = append(parts, "--dpi-desync-repeats="+strconv.Itoa(*pr.Repeats))
		}
		ttl, hasTTL, err := pr.ttl()
		if err != nil {
			return nil, err
		}
		if hasTTL {
			parts = append(parts, "--dpi-desync-ttl="+strconv.Itoa(ttl))
		}
		if pr.AnyProtocol {
			parts = append(parts, "--dpi-desync-any-protocol=1")
		}
		if !isBlank(pr.Cutoff) && !strings.EqualFold(pr.Cutoff, "none") {
			parts = append(parts, "--dpi-desync-cutoff="+pr.Cutoff)
		}
		if pr.Seqovl != nil {
			parts = append(parts, "--dpi-desync-split-seqovl="+strconv.Itoa(*pr.Seqovl))
		}
		if !isBlank(pr.SplitPos) {
			parts = append(parts, "--dpi-desync-split-pos="+pr.SplitPos)
		}
		if len(pr.Fooling) > 0 {
			parts = append(parts, "--dpi-desync-fooling="+strings.Join(pr.Fooling, ","))
		}
		if inc, set := pr.badseqIncrement(); set && hasFold(pr.Fooling, "badseq") {
			parts = append(parts, "--dpi-desync-badseq-increment="+strconv.Itoa(inc))
		}
		if !isBlank(pr.SeqovlPattern) {
			path, ok := resolveRef(pr.SeqovlPattern, p)
			if !ok {
				return nil, fmt.Errorf("seqovlPattern must be a bin:/lists:/user: path: %s", pr.SeqovlPattern)
			}
			parts = append(parts, "--dpi-desync-split-seqovl-pattern="+quote(path))
		}
		if !isBlank(pr.HostfakesplitMod) {
			parts = append(parts, "--dpi-desync-hostfakesplit-mod="+pr.HostfakesplitMod)
		}
		if fds := pr.FakedsplitPattern; !isBlank(fds) {
			if path, ok := resolveRef(fds, p); ok {
				fds = quote(path)
			}
			parts = append(parts, "--dpi-desync-fakedsplit-pattern="+fds)
		}
		for _, d := range v01FakeSlots {
			for _, ref := range d.get(&pr.Fake).Refs {
				if isBlank(ref) {
					continue
				}
				if path, ok := resolveRef(ref, p); ok {
					ref = quote(path)
				}
				parts = append(parts, d.flag+"="+ref)
			}
		}
		if !isBlank(pr.TLSMod) {
			parts = append(parts, "--dpi-desync-fake-tls-mod="+pr.TLSMod)
		}
		out = append(out, parts)
	}
	return finishProfiles([]string{wf}, out), nil
}

// registerBlobs is Register-ZapretSpecBlobs.
func registerBlobs(s *V01Spec, p Paths) (*blobRegistry, error) {
	reg := newBlobRegistry(p)
	for i := range s.profiles() {
		pr := &s.profiles()[i]
		for _, d := range v01FakeSlots {
			for _, ref := range d.get(&pr.Fake).Refs {
				if err := reg.add(ref); err != nil {
					return nil, err
				}
			}
		}
		if _, err := pr.desync(); err != nil {
			return nil, err
		}
		if !isBlank(pr.SeqovlPattern) {
			if err := reg.add(pr.SeqovlPattern); err != nil {
				return nil, err
			}
		}
		if isFileRef(pr.FakedsplitPattern) {
			if err := reg.add(pr.FakedsplitPattern); err != nil {
				return nil, err
			}
		}
	}
	return reg, nil
}

// luaFoolingArgs is ConvertTo-ZapretSpecLuaFoolingArgs.
func (pr *V01Profile) luaFoolingArgs() ([]string, error) {
	var out []string
	for _, f := range pr.Fooling {
		switch strings.ToLower(f) {
		case "ts":
			out = append(out, "tcp_ts=-600000")
		case "md5sig":
			out = append(out, "tcp_md5")
		case "badseq":
			// winws badseq changes seq by the increment and ack by
			// the badack default. tcp_seq=0 does nothing: do not write it.
			if inc, _ := pr.badseqIncrement(); inc != 0 {
				out = append(out, "tcp_seq="+strconv.Itoa(inc))
			}
			out = append(out, "tcp_ack="+strconv.Itoa(badackDefault))
		default:
			return nil, fmt.Errorf("Unknown fooling '%s'.", f)
		}
	}
	ttl, hasTTL, err := pr.ttl()
	if err != nil {
		return nil, err
	}
	if hasTTL {
		out = append(out, "ip_ttl="+strconv.Itoa(ttl))
	}
	return out, nil
}

// luaPayloadArgs is Get-ZapretSpecLuaPayloadArgs.
func (pr *V01Profile) luaPayloadArgs() ([]string, error) {
	if isBlank(pr.LuaPayload) {
		return nil, nil
	}
	if !matchLuaPayload(pr.LuaPayload) {
		return nil, errors.New("luaPayload must contain comma-separated payload names.")
	}
	return []string{"payload=" + pr.LuaPayload}, nil
}

// luaModArgs is Get-ZapretSpecLuaModArgs.
func (pr *V01Profile) luaModArgs(caps attackCaps, reg *blobRegistry) ([]string, error) {
	var out []string
	if caps.hostfakesplit && !isBlank(pr.HostfakesplitMod) {
		for _, piece := range strings.Split(pr.HostfakesplitMod, ",") {
			piece = strings.TrimSpace(piece)
			if isBlank(piece) {
				continue
			}
			// Stock zapret-antidpi.lua has no altorder. Keep the flag on z1 only.
			if len(piece) >= 9 && strings.EqualFold(piece[:9], "altorder=") {
				continue
			}
			out = append(out, piece)
		}
	}
	if caps.fakedsplit && !isBlank(pr.FakedsplitPattern) {
		pat := pr.FakedsplitPattern
		if isFileRef(pat) {
			pat, _ = reg.name(pat)
		}
		out = append(out, "pattern="+pat)
	}
	if !caps.split {
		return out, nil
	}
	if !isBlank(pr.SplitPos) {
		out = append(out, "pos="+pr.SplitPos)
	}
	if pr.Seqovl != nil {
		out = append(out, "seqovl="+strconv.Itoa(*pr.Seqovl))
	}
	if !isBlank(pr.SeqovlPattern) {
		n, ok := reg.name(pr.SeqovlPattern)
		if !ok {
			return nil, fmt.Errorf("seqovlPattern blob is missing: %s", pr.SeqovlPattern)
		}
		out = append(out, "seqovl_pattern="+n)
	}
	return out, nil
}

// buildWinws2 is ConvertTo-ZapretSpecWinws2ArgList.
func buildWinws2(s *V01Spec, p Paths, gf GameFilter) ([]string, error) {
	wf, err := wfLine(s, "--wf-tcp-out", "--wf-udp-out", gf)
	if err != nil {
		return nil, err
	}
	lines := []string{wf, fmt.Sprintf("--lua-init=@%s --lua-init=@%s",
		quote(joinPath(p.Bin, `lua\zapret-lib.lua`)), quote(joinPath(p.Bin, `lua\zapret-antidpi.lua`)))}

	reg, err := registerBlobs(s, p)
	if err != nil {
		return nil, err
	}
	if len(reg.order) > 0 {
		parts := make([]string, 0, len(reg.order))
		for _, b := range reg.order {
			parts = append(parts, "--blob="+b.name+":"+b.value)
		}
		lines = append(lines, strings.Join(parts, " "))
	}

	var out [][]string
	for i := range s.profiles() {
		pr := &s.profiles()[i]
		parts, err := pr.commonFilters(p, gf)
		if err != nil {
			return nil, err
		}
		if n, ok := matchCutoffN(pr.Cutoff); ok {
			parts = append(parts, "--out-range=-d"+n)
		} else if pr.needOutRange() {
			parts = append(parts, "--out-range=-d10")
		}

		desync, err := pr.desync()
		if err != nil {
			return nil, err
		}
		foolArgs, err := pr.luaFoolingArgs()
		if err != nil {
			return nil, err
		}

		for _, group := range pr.payloadGroups() {
			if !isBlank(group) {
				parts = append(parts, "--payload="+group)
			}
			for _, attack := range desync {
				caps := v01Attacks[strings.ToLower(attack)]
				payloadArgs, err := pr.luaPayloadArgs()
				if err != nil {
					return nil, err
				}
				if strings.EqualFold(attack, "fake") {
					calls, err := pr.fakeCallsForGroup(group)
					if err != nil {
						return nil, err
					}
					if len(calls) < 1 {
						return nil, fmt.Errorf("Strategy fake has no blob for payload group '%s'.", group)
					}
					for _, call := range calls {
						ref := call.ref
						lua := append([]string{"fake"}, payloadArgs...)
						if call.payload != "" {
							lua = []string{"fake", "payload=" + call.payload}
						}
						name, ok := reg.name(ref)
						if !ok {
							return nil, fmt.Errorf("Strategy fake blob is missing: %s", ref)
						}
						lua = append(lua, "blob="+name)
						if pr.Repeats != nil {
							lua = append(lua, "repeats="+strconv.Itoa(*pr.Repeats))
						}
						if call.tlsMod {
							lua = append(lua, "tls_mod="+pr.TLSMod)
						}
						lua = append(lua, foolArgs...)
						if !isBlank(pr.IPID) {
							lua = append(lua, "ip_id="+pr.IPID)
						}
						parts = append(parts, "--lua-desync="+strings.Join(lua, ":"))
					}
					continue
				}

				lua := append([]string{attack}, payloadArgs...)
				mod, err := pr.luaModArgs(caps, reg)
				if err != nil {
					return nil, err
				}
				lua = append(lua, mod...)
				if pr.Repeats != nil && caps.repeats {
					lua = append(lua, "repeats="+strconv.Itoa(*pr.Repeats))
				}
				if caps.fooling {
					lua = append(lua, foolArgs...)
				}
				if !isBlank(pr.IPID) {
					lua = append(lua, "ip_id="+pr.IPID)
				}
				parts = append(parts, "--lua-desync="+strings.Join(lua, ":"))
			}
		}
		out = append(out, parts)
	}
	return finishProfiles(lines, out), nil
}
