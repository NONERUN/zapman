// Command import-flowseal makes the flowseal/ strategy files from the
// general*.bat files of Flowseal/zapret-discord-youtube.
//
// It reads the .bat files of an upstream ref from git (no checkout). For
// each .bat file it finds the winws args, maps each profile (the args
// between --new) to a v2 rule and uses the names of targets.json and
// actions.json for the same target and action. Then it builds the winws
// argv of the new rules and compares it with the .bat argv (the round
// trip). A flag or a value that it cannot map is an error.
//
// The rules get our two additions for winws2 (see derive.go): the derived
// --filter-l7 and the stock TLS hello "!". The compare adds them to the
// .bat argv too; all other differences stay.
//
// The status of a .bat file is:
//
//	same     the winws argv of the strategy file is the same as the .bat argv
//	changed  the argv is not the same; the lines show the differences
//	new      no strategy file has the name
//	removed  the strategy file has no .bat file in the ref
//	error    the .bat file cannot be mapped
//
// With -write it writes the new rules to flowseal/<name>.json for each
// changed and new file, with the upstream tag of the .bat file and rev 1.
// The about text stays. A file with the status same does not change. A
// removed file stays. Then run go generate ./internal/spec for the hashes
// of catalog.json.
//
// The exit code is 1 on an error and 2 when a status is not same and the
// file is not written (or is removed).
//
//	go run ./dev/import-flowseal [-git .] [-ref upstream/main] [-dir strategies] [-write]
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/NONERUN/zapman/dev/internal/jsonfmt"
	"github.com/NONERUN/zapman/dev/internal/upstream"
	"github.com/NONERUN/zapman/internal/spec"
)

// options are the command line options.
type options struct {
	Git   string // git folder with the upstream ref
	Ref   string // upstream ref with the .bat files
	Dir   string // catalog folder
	Write bool   // write the changed and new files
}

// The statuses of a .bat file.
const (
	statusSame    = "same"
	statusChanged = "changed"
	statusNew     = "new"
	statusRemoved = "removed"
	statusError   = "error"
)

// result is the import result of one .bat file.
type result struct {
	Name    string
	Status  string
	Tag     string   // upstream tag for a changed or new file
	Diff    []string // ours against Flowseal with the derived args, for a changed file
	Err     error
	Written bool
}

// rtRoot is the root of the paths of the round trip build. The paths are
// only for the compare.
const rtRoot = `C:\zapman`

// rtPaths are the folders of the round trip build.
var rtPaths = spec.Paths{
	Bin:     rtRoot + `\bin`,
	Lists:   rtRoot + `\lists`,
	User:    rtRoot + `\user`,
	Catalog: rtRoot + `\catalog`,
}

// rtPrefixes are the path prefixes of the round trip build. They give the
// paths of the .bat files: Flowseal keeps all fakes in bin\ and all lists
// in lists\.
var rtPrefixes = []prefix{
	{rtPaths.Catalog + `\` + spec.FakesDir + `\`, "bin/"},
	{rtPaths.User + `\` + spec.ActiveFakesDir + `\`, "bin/"},
	{rtPaths.Bin + `\`, "bin/"},
	{rtPaths.Lists + `\`, "lists/"},
	{rtPaths.User + `\`, "lists/"},
}

// rtGameFilter keeps the Game Filter variables of the .bat files.
var rtGameFilter = spec.GameFilter{TCP: gfTCPVar, UDP: gfUDPVar}

func main() {
	var o options
	flag.StringVar(&o.Git, "git", ".", "git folder with the upstream ref")
	flag.StringVar(&o.Ref, "ref", upstream.DefaultRef, "upstream ref or tag with the .bat files")
	flag.StringVar(&o.Dir, "dir", "strategies", "catalog folder")
	flag.BoolVar(&o.Write, "write", false, "write the changed and new strategy files")
	flag.Parse()
	res, err := run(o, os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "import-flowseal:", err)
		os.Exit(1)
	}
	os.Exit(exitCode(res))
}

// exitCode gives 1 when a result is an error, 2 when a result is not same
// and not written, else 0.
func exitCode(res []result) int {
	code := 0
	for _, r := range res {
		switch {
		case r.Status == statusError:
			return 1
		case r.Status != statusSame && !r.Written:
			code = 2
		}
	}
	return code
}

// run imports the .bat files of the ref and writes the report to w.
func run(o options, w io.Writer) ([]result, error) {
	repo := upstream.Repo{Dir: o.Git, Ref: o.Ref}
	cat, err := spec.LoadCatalog(o.Dir, "")
	if err != nil {
		return nil, err
	}
	names, err := repo.Bats()
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no general*.bat files in %s", repo.Ref)
	}
	files, err := cat.StrategyNames()
	if err != nil {
		return nil, err
	}
	have := map[string]bool{}
	for _, n := range files {
		have[n] = true
	}
	m := newMapper(cat)
	var res []result
	bats := map[string]bool{}
	for _, name := range names {
		bats[name] = true
		r := importBat(repo, cat, m, have, name, o)
		res = append(res, r)
	}
	for _, name := range files {
		if bats[name] {
			continue
		}
		st, err := cat.Strategy(name)
		switch {
		case err != nil:
			res = append(res, result{Name: name, Status: statusError, Err: err})
		case st.Origin() == spec.OriginFlowseal:
			res = append(res, result{Name: name, Status: statusRemoved})
		}
	}
	report(w, res)
	if o.Write {
		for _, r := range res {
			if r.Written {
				fmt.Fprintln(w, "Files are written. Run: go generate ./internal/spec")
				break
			}
		}
	}
	return res, nil
}

// importBat imports one .bat file.
func importBat(repo upstream.Repo, cat *spec.Catalog, m *mapper, have map[string]bool, name string, o options) result {
	r := result{Name: name}
	fail := func(err error) result {
		r.Status, r.Err = statusError, err
		return r
	}
	data, err := repo.Show(name + ".bat")
	if err != nil {
		return fail(err)
	}
	theirs, err := parseBat(data)
	if err != nil {
		return fail(err)
	}
	rules, derived, err := m.rules(theirs)
	if err != nil {
		return fail(err)
	}
	// The .bat argv with our derived additions (see derive).
	want := withDerived(theirs, derived)

	sf := spec.StrategyFile{Schema: "../schema/strategy.json", Spec: spec.Version, Rev: 1, Rules: rules}
	var old *spec.Strategy
	if have[name] {
		old, err = cat.Strategy(name)
	}
	switch {
	case err != nil:
		return fail(err)
	case old == nil:
		r.Status = statusNew
	case old.Origin() != spec.OriginFlowseal:
		return fail(fmt.Errorf("the strategy is in %s/, not in %s/", old.Origin(), spec.OriginFlowseal))
	default:
		sf.About = old.About
		ours, err := buildArgv(cat, old)
		if err != nil {
			return fail(fmt.Errorf("build of %s/%s.json: %w", spec.OriginFlowseal, name, err))
		}
		r.Diff = diffArgv(ours, want)
		r.Status = statusChanged
		if len(r.Diff) == 0 {
			r.Status = statusSame
		}
	}

	// The round trip: the new file must give the .bat argv.
	sf.Upstream = "0"
	if r.Status != statusSame {
		tag, _, err := repo.Tag(name)
		if err != nil {
			return fail(fmt.Errorf("upstream tag: %w", err))
		}
		sf.Upstream, r.Tag = tag, tag
	}
	text, err := jsonfmt.Marshal(sf)
	if err != nil {
		return fail(err)
	}
	file := jsonfmt.Pretty(string(text))
	st, err := cat.ParseStrategy(name, spec.OriginFlowseal, file)
	if err != nil {
		return fail(fmt.Errorf("new file: %w", err))
	}
	ours, err := buildArgv(cat, st)
	if err != nil {
		return fail(fmt.Errorf("build of the new file: %w", err))
	}
	if d := diffArgv(ours, want); len(d) > 0 {
		return fail(fmt.Errorf("round trip: the new file does not give the .bat argv:\n\t%s", strings.Join(d, "\n\t")))
	}

	if o.Write && r.Status != statusSame {
		path := filepath.Join(o.Dir, string(spec.OriginFlowseal), name+".json")
		if err := os.WriteFile(path, file, 0o644); err != nil {
			return fail(err)
		}
		r.Written = true
	}
	return r
}

// buildArgv builds the winws argv of the strategy with no games.
// The active fakes use the user files: the path is bin\ACTIVE_X.bin, as
// in the .bat files.
func buildArgv(cat *spec.Catalog, st *spec.Strategy) (*argv, error) {
	p := rtPaths
	p.ActiveFiles = map[string]bool{}
	for n := range cat.Active {
		p.ActiveFiles[n] = true
	}
	lines, err := spec.Build(st, nil, spec.Winws, p, rtGameFilter)
	if err != nil {
		return nil, err
	}
	var words []string
	for _, l := range lines {
		for _, w := range splitWords(l) {
			words = append(words, strings.ReplaceAll(w, `"`, ""))
		}
	}
	a, err := parseArgs(words, rtPrefixes)
	if err != nil {
		return nil, errors.New("argv of the build: " + err.Error())
	}
	return a, nil
}

// report writes the table of the results and the details.
func report(w io.Writer, res []result) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "BAT\tSTATUS\tUPSTREAM\tNOTE")
	for _, r := range res {
		note := ""
		switch {
		case r.Written:
			note = "written"
		case r.Status == statusChanged:
			note = fmt.Sprintf("%d differences", len(r.Diff))
		case r.Status == statusRemoved:
			note = "not deleted"
		}
		tag := r.Tag
		if tag == "" {
			tag = "-"
		}
		fmt.Fprintf(tw, "%s.bat\t%s\t%s\t%s\n", r.Name, r.Status, tag, note)
	}
	tw.Flush()
	for _, r := range res {
		switch {
		case r.Err != nil:
			fmt.Fprintf(w, "\n%s.bat: error: %v\n", r.Name, r.Err)
		case len(r.Diff) > 0:
			fmt.Fprintf(w, "\n%s.bat: ours (%s/%s.json) against Flowseal:\n", r.Name, spec.OriginFlowseal, r.Name)
			for _, d := range r.Diff {
				fmt.Fprintf(w, "\t%s\n", d)
			}
		}
	}
}
