package spec

import "strings"

// Paths are the folders for the lists:, user: and bin: prefixes.
type Paths struct {
	Bin   string
	Lists string
	User  string
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

// resolveRef gives the path of a lists:, user: or bin: ref. The prefix test
// is case-sensitive. The name after the prefix must not be empty.
func resolveRef(ref string, p Paths) (string, bool) {
	for _, pre := range []struct {
		name string
		dir  string
	}{{"lists:", p.Lists}, {"user:", p.User}, {"bin:", p.Bin}} {
		if len(ref) > len(pre.name) && strings.HasPrefix(ref, pre.name) {
			return joinPath(pre.dir, ref[len(pre.name):]), true
		}
	}
	return "", false
}

// isFileRef tells if ref is a lists:, user: or bin: path.
func isFileRef(ref string) bool {
	_, ok := resolveRef(ref, Paths{})
	return ok
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
