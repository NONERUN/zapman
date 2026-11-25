package spec

import (
	"fmt"
	"strings"
)

// Paths are the folders for the path prefixes of the refs.
type Paths struct {
	// Bin is the engine folder: bin: refs and the Lua files of winws2.
	Bin string
	// Lists is the folder for lists: refs.
	Lists string
	// User is the folder for user: refs and for the user copies of the
	// active fakes (see ActivePath).
	User string
	// Catalog is the root folder of the catalog for cat: refs.
	Catalog string
	// ActiveFiles are the names X of the active:X refs that have a user
	// file (ActivePath). The caller finds these files. The build does not
	// examine the disk. An active:X ref with no user file uses the default
	// of the catalog (catalog.json "active").
	ActiveFiles map[string]bool

	// activeDefault are the catalog defaults of the active fakes (name to
	// cat: ref). Build sets them from the strategy.
	activeDefault map[string]string
}

// The ref prefixes.
const (
	PrefixLists  = "lists:"
	PrefixUser   = "user:"
	PrefixBin    = "bin:"
	PrefixCat    = "cat:"
	PrefixActive = "active:"
)

// ActiveFakesDir is the folder in Paths.User with the user copies of the
// active fakes.
const ActiveFakesDir = "fakes"

// ActivePath gives the user file of the active fake name:
// <User>\fakes\ACTIVE_<name>.bin.
func (p Paths) ActivePath(name string) string {
	return joinPath(p.User, ActiveFakesDir+`\`+activeStem(name)+".bin")
}

// activeStem gives the file name of the active fake name without the
// extension. It is also the winws2 blob name.
func activeStem(name string) string { return "ACTIVE_" + name }

// isActiveName tells if s is a name of an active fake: A-Z and 0-9 in
// parts that _ separates.
func isActiveName(s string) bool {
	if s == "" || s[0] == '_' || s[len(s)-1] == '_' || strings.Contains(s, "__") {
		return false
	}
	for _, r := range s {
		if !(r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_') {
			return false
		}
	}
	return true
}

// GameFilter holds the ports for the $gf.Tcp and $gf.Udp tokens.
type GameFilter struct {
	TCP string
	UDP string
}

// joinPath is Join-Path of Windows PowerShell for a file system path.
func joinPath(parent, child string) string {
	child = strings.ReplaceAll(child, "/", `\`)
	parent = strings.ReplaceAll(parent, "/", `\`)
	return strings.TrimRight(parent, `\`) + `\` + strings.TrimLeft(child, `\`)
}

// leafName is Split-Path -Leaf.
func leafName(p string) string {
	p = strings.TrimRight(p, `\/`)
	if i := strings.LastIndexAny(p, `\/:`); i >= 0 {
		return p[i+1:]
	}
	return p
}

func quote(p string) string {
	return `"` + p + `"`
}

// refPrefixes are the prefixes of the file refs, in test order.
var refPrefixes = []string{PrefixLists, PrefixUser, PrefixCat, PrefixActive, PrefixBin}

// refPrefixText is the list of the prefixes for error texts.
const refPrefixText = "lists:/user:/cat:/active:/bin:"

// splitRef gives the prefix and the name of a file ref. The prefix test is
// case-sensitive. The name after the prefix must not be empty. The name of
// an active: ref must be an active name (isActiveName).
func splitRef(ref string) (prefix, name string, ok bool) {
	for _, pre := range refPrefixes {
		if len(ref) > len(pre) && strings.HasPrefix(ref, pre) {
			name = ref[len(pre):]
			if pre == PrefixActive && !isActiveName(name) {
				return "", "", false
			}
			return pre, name, true
		}
	}
	return "", "", false
}

// resolveRef gives the path of a file ref. An active:X ref gives
// Paths.ActivePath when X is in Paths.ActiveFiles, else the path of the
// catalog default. An active: ref with no user file and no default gives
// false (see checkActive).
func resolveRef(ref string, p Paths) (string, bool) {
	pre, name, ok := splitRef(ref)
	if !ok {
		return "", false
	}
	switch pre {
	case PrefixLists:
		return joinPath(p.Lists, name), true
	case PrefixUser:
		return joinPath(p.User, name), true
	case PrefixCat:
		return joinPath(p.Catalog, name), true
	case PrefixBin:
		return joinPath(p.Bin, name), true
	}
	if p.ActiveFiles[name] {
		return p.ActivePath(name), true
	}
	def, ok := p.activeDefault[name]
	if !ok || !strings.HasPrefix(def, PrefixCat) {
		return "", false
	}
	return resolveRef(def, p)
}

// isFileRef tells if ref is a path with a prefix of refPrefixes.
func isFileRef(ref string) bool {
	_, _, ok := splitRef(ref)
	return ok
}

// checkActive tells if each active: ref of the spec has a path in p.
func checkActive(s *V01Spec, p Paths) error {
	for i := range s.Profiles {
		for _, r := range s.Profiles[i].fileRefs() {
			if !strings.HasPrefix(r, PrefixActive) {
				continue
			}
			if _, ok := resolveRef(r, p); !ok {
				return s.at(i, fmt.Errorf("active fake %s has no user file and no catalog default", r))
			}
		}
	}
	return nil
}

// fileRefs gives the refs of the profile that can be file refs: the lists,
// the patterns and the fakes. "!" gives defaultTLSFake.
func (pr *V01Profile) fileRefs() []string {
	refs := []string{pr.SeqovlPattern, pr.FakedsplitPattern}
	for _, l := range []StringList{pr.Hostlist, pr.HostlistExclude, pr.Ipset, pr.IpsetExclude} {
		refs = append(refs, l...)
	}
	for _, d := range v01FakeSlots {
		for _, r := range d.get(&pr.Fake).Refs {
			if r == "!" {
				// winws2 sends this blob for "!".
				r = defaultTLSFake
			}
			refs = append(refs, r)
		}
	}
	return refs
}

// matchCutoffN tells if cutoff is n and decimal digits. It gives the digits.
func matchCutoffN(cutoff string) (string, bool) {
	if len(cutoff) < 2 || cutoff[0] != 'n' {
		return "", false
	}
	for _, r := range cutoff[1:] {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	return cutoff[1:], true
}

func isASCIIAlnum(r rune) bool {
	return r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

// matchLuaPayload tells if v is a comma list of payload names
// ([a-zA-Z0-9_]+).
func matchLuaPayload(v string) bool {
	for _, part := range strings.Split(v, ",") {
		if part == "" {
			return false
		}
		for _, r := range part {
			if !isASCIIAlnum(r) && r != '_' {
				return false
			}
		}
	}
	return true
}

func has(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// foldSet is a set with the case rules of a PowerShell hashtable.
type foldSet map[string]bool

func (s foldSet) has(k string) bool { return s[strings.ToLower(k)] }
func (s foldSet) add(k string)      { s[strings.ToLower(k)] = true }

func applyGameFilter(v string, gf GameFilter) string {
	v = strings.ReplaceAll(v, "$gf.Tcp", gf.TCP)
	return strings.ReplaceAll(v, "$gf.Udp", gf.UDP)
}

func csv(items []string, gf GameFilter) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		if isBlank(it) {
			continue
		}
		parts = append(parts, applyGameFilter(it, gf))
	}
	return strings.Join(parts, ",")
}
