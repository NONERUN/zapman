package main

import (
	"fmt"
	"sort"
	"strings"
)

// arg is one winws arg: --flag=value.
type arg struct {
	Flag     string
	Value    string
	HasValue bool // the arg has "="
}

// String gives the arg as text.
func (a arg) String() string {
	if !a.HasValue {
		return a.Flag
	}
	return a.Flag + "=" + a.Value
}

// argv is a winws command line: the wf ports and the profiles. --new
// separates the profiles.
type argv struct {
	WFTCP  []string
	WFUDP  []string
	Groups [][]arg
}

// prefix is a path prefix and its normal form.
type prefix struct{ from, to string }

// splitWords splits a command line on spaces and tabs. A space in double
// quotes does not split. The quotes stay in the words.
func splitWords(line string) []string {
	var out []string
	var b strings.Builder
	quoted, word := false, false
	for _, r := range line {
		switch {
		case r == '"':
			quoted = !quoted
			word = true
			b.WriteRune(r)
		case (r == ' ' || r == '\t') && !quoted:
			if word {
				out = append(out, b.String())
				b.Reset()
				word = false
			}
		default:
			word = true
			b.WriteRune(r)
		}
	}
	if word {
		out = append(out, b.String())
	}
	return out
}

// parseArgs makes the argv of the winws words. The words have no quotes.
// The path values get the normal form of prefixes: "bin/<file>" or
// "lists/<file>". --wf-tcp and --wf-udp must be before the first profile
// flag.
func parseArgs(words []string, prefixes []prefix) (*argv, error) {
	a := &argv{}
	var group []arg
	wf := map[string]bool{}
	for _, w := range words {
		if w == "--new" {
			if len(group) == 0 {
				return nil, fmt.Errorf("profile %d is empty", len(a.Groups)+1)
			}
			a.Groups = append(a.Groups, group)
			group = nil
			continue
		}
		if !strings.HasPrefix(w, "--") {
			return nil, fmt.Errorf("%q is not a flag", w)
		}
		x := arg{Flag: w}
		if f, v, ok := strings.Cut(w, "="); ok {
			x = arg{Flag: f, Value: normPath(v, prefixes), HasValue: true}
		}
		if strings.HasPrefix(x.Flag, "--wf-") {
			if len(a.Groups) > 0 || len(group) > 0 {
				return nil, fmt.Errorf("%s is after a profile flag", x.Flag)
			}
			if x.Flag != "--wf-tcp" && x.Flag != "--wf-udp" {
				return nil, fmt.Errorf("unknown flag %s", x.Flag)
			}
			if wf[x.Flag] {
				return nil, fmt.Errorf("%s is two times", x.Flag)
			}
			wf[x.Flag] = true
			ports := strings.Split(x.Value, ",")
			if x.Flag == "--wf-tcp" {
				a.WFTCP = ports
			} else {
				a.WFUDP = ports
			}
			continue
		}
		group = append(group, x)
	}
	if len(group) == 0 {
		return nil, fmt.Errorf("profile %d is empty", len(a.Groups)+1)
	}
	a.Groups = append(a.Groups, group)
	return a, nil
}

// normPath gives the normal form of a path value: the first prefix that
// matches (with no case) changes to its normal form, and \ changes to /.
// Other values do not change.
func normPath(v string, prefixes []prefix) string {
	for _, p := range prefixes {
		if len(v) > len(p.from) && strings.EqualFold(v[:len(p.from)], p.from) {
			return p.to + strings.ReplaceAll(v[len(p.from):], `\`, "/")
		}
	}
	return v
}

// tlsFlags are the flags with an order that is important across the flags:
// winws applies --dpi-desync-fake-tls-mod to the fake-tls flags.
var tlsFlags = map[string]bool{"--dpi-desync-fake-tls": true, "--dpi-desync-fake-tls-mod": true}

// diffArgv compares our argv and the Flowseal argv. It gives one line per
// difference; no lines when they are the same. The wf ports are sets: the
// build sorts them. In a profile the order of the flags is free, but the
// order of the values of one flag is important, and the order of the
// fake-tls and fake-tls-mod flags is important.
func diffArgv(ours, theirs *argv) []string {
	var out []string
	for _, wf := range []struct {
		name string
		a, b []string
	}{{"--wf-tcp", ours.WFTCP, theirs.WFTCP}, {"--wf-udp", ours.WFUDP, theirs.WFUDP}} {
		onlyA, onlyB := setDiff(wf.a, wf.b), setDiff(wf.b, wf.a)
		if len(onlyA) > 0 || len(onlyB) > 0 {
			out = append(out, fmt.Sprintf("%s: only ours: %s; only Flowseal: %s", wf.name, listText(onlyA), listText(onlyB)))
		}
	}
	if len(ours.Groups) != len(theirs.Groups) {
		out = append(out, fmt.Sprintf("profiles: ours %d, Flowseal %d", len(ours.Groups), len(theirs.Groups)))
	}
	for i := 0; i < len(ours.Groups) && i < len(theirs.Groups); i++ {
		for _, d := range diffGroup(ours.Groups[i], theirs.Groups[i]) {
			out = append(out, fmt.Sprintf("profile %d: %s", i+1, d))
		}
	}
	return out
}

// diffGroup compares the args of one profile.
func diffGroup(ours, theirs []arg) []string {
	va, order := flagValues(ours, nil)
	vb, order := flagValues(theirs, order)
	var out []string
	for _, f := range order {
		a, b := va[f], vb[f]
		switch {
		case a == nil:
			for _, x := range b {
				out = append(out, "only Flowseal: "+x.String())
			}
		case b == nil:
			for _, x := range a {
				out = append(out, "only ours: "+x.String())
			}
		case argsText(a) != argsText(b):
			out = append(out, fmt.Sprintf("%s: ours %s; Flowseal %s", f, argsText(a), argsText(b)))
		}
	}
	if len(out) > 0 {
		return out
	}
	if a, b := argsText(tlsArgs(ours)), argsText(tlsArgs(theirs)); a != b {
		out = append(out, fmt.Sprintf("fake-tls order: ours %s; Flowseal %s", a, b))
	}
	return out
}

// flagValues gives the args by flag. It adds the new flags to order.
func flagValues(args []arg, order []string) (map[string][]arg, []string) {
	m := map[string][]arg{}
	seen := map[string]bool{}
	for _, f := range order {
		seen[f] = true
	}
	for _, a := range args {
		m[a.Flag] = append(m[a.Flag], a)
		if !seen[a.Flag] {
			seen[a.Flag] = true
			order = append(order, a.Flag)
		}
	}
	return m, order
}

func tlsArgs(args []arg) []arg {
	var out []arg
	for _, a := range args {
		if tlsFlags[a.Flag] {
			out = append(out, a)
		}
	}
	return out
}

func argsText(args []arg) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = a.String()
	}
	return strings.Join(parts, " ")
}

// setDiff gives the items of a that are not in b, sorted.
func setDiff(a, b []string) []string {
	in := map[string]bool{}
	for _, x := range b {
		in[x] = true
	}
	var out []string
	for _, x := range a {
		if !in[x] {
			out = append(out, x)
			in[x] = true
		}
	}
	sort.Strings(out)
	return out
}

func listText(items []string) string {
	if len(items) == 0 {
		return "-"
	}
	return strings.Join(items, ",")
}
