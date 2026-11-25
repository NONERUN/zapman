package spec

import (
	"strings"
	"unicode"
)

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

// resolveRef is ConvertFrom-ZapretSpecPathRef. The prefix test ignores case.
func resolveRef(ref string, p Paths) (string, bool) {
	for _, pre := range []struct {
		name string
		dir  string
	}{{"lists:", p.Lists}, {"user:", p.User}, {"bin:", p.Bin}} {
		if len(ref) >= len(pre.name) && strings.EqualFold(ref[:len(pre.name)], pre.name) {
			return joinPath(pre.dir, ref[len(pre.name):]), true
		}
	}
	return "", false
}

// isFileRef is Test-ZapretSpecFileRef. The prefix test is case-sensitive.
func isFileRef(ref string) bool {
	if isBlank(ref) {
		return false
	}
	return strings.HasPrefix(ref, "lists:") || strings.HasPrefix(ref, "user:") || strings.HasPrefix(ref, "bin:")
}

// psEnd removes one final "\n". The .NET regex anchor $ also matches before it.
func psEnd(s string) string {
	return strings.TrimSuffix(s, "\n")
}

// matchCutoffN is $cutoff -match '^n(\d+)$'. It gives the digits.
func matchCutoffN(cutoff string) (string, bool) {
	s := psEnd(cutoff)
	if len(s) < 2 || (s[0] != 'n' && s[0] != 'N') {
		return "", false
	}
	for _, r := range s[1:] {
		if !unicode.Is(unicode.Nd, r) {
			return "", false
		}
	}
	return s[1:], true
}

func isASCIIAlnum(r rune) bool {
	return r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

// matchLuaPayload is $v -match '^[a-z0-9_]+(,[a-z0-9_]+)*$' (ignore case).
func matchLuaPayload(v string) bool {
	s := psEnd(v)
	for _, part := range strings.Split(s, ",") {
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

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

func hasFold(list []string, v string) bool {
	for _, x := range list {
		if strings.EqualFold(x, v) {
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
