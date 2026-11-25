// Command convert-v2 converts the v0.1 strategy files to the strategy
// format v2 one time.
//
// It reads strategies/*.json (specVersion 0.1.0). It makes catalog.json,
// the list bundles, targets and actions, the game modules sky, sker and
// photon from "general (ALT11 Photon)", and the v2 strategy files in
// flowseal/. An action is a named preset only when 3 or more rules use it.
// Each strategy gets the upstream tag of its .bat file (from the upstream
// ref in git, see dev/upstream-versions) and rev 1. Each game gets
// version 1. It writes the files to a temporary folder and checks that the
// argv is the same as the v0.1 argv for each strategy, engine and Game
// Filter. Then it writes the files to the strategies folder and deletes the
// v0.1 files.
//
// This program is only for the migration. Delete it after the migration.
//
//	go run ./dev/convert-v2 [-dir strategies] [-bin bin] [-lists lists] [-git .] [-ref upstream/main] [-dry]
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/NONERUN/zapman/dev/internal/upstream"
	"github.com/NONERUN/zapman/internal/spec"
)

// options are the command line options.
type options struct {
	Dir   string // strategies folder
	Bin   string // bin folder for the bin: check; empty: no check
	Lists string // lists folder for the seed files
	Git   string // git folder with the upstream ref
	Ref   string // upstream ref for the upstream tags
	Dry   bool   // do not write and do not delete
}

func main() {
	var o options
	flag.StringVar(&o.Dir, "dir", "strategies", "strategies folder")
	flag.StringVar(&o.Bin, "bin", "bin", "bin folder (empty: no bin: check)")
	flag.StringVar(&o.Lists, "lists", "lists", "lists folder (seed files)")
	flag.StringVar(&o.Git, "git", ".", "git folder with the upstream ref")
	flag.StringVar(&o.Ref, "ref", upstream.DefaultRef, "upstream ref for the upstream tags")
	flag.BoolVar(&o.Dry, "dry", false, "check and show the report; do not write")
	flag.Parse()
	if err := run(o, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "convert-v2:", err)
		os.Exit(1)
	}
}

// run does the conversion and writes the report to w.
func run(o options, w io.Writer) error {
	old, names, err := readV01(o.Dir)
	if err != nil {
		return err
	}
	tags, err := upstreamTags(upstream.Repo{Dir: o.Git, Ref: o.Ref}, names)
	if err != nil {
		return err
	}
	res, err := convert(old, names, o.Lists, tags)
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "convert-v2-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err := writeFiles(tmp, res.files); err != nil {
		return err
	}
	if err := checkParity(tmp, o.Bin, old, names, res, w); err != nil {
		return fmt.Errorf("parity: %w (nothing is written)", err)
	}
	res.report(w)
	if o.Dry {
		fmt.Fprintln(w, "dry run: nothing is written")
		return nil
	}
	if err := writeFiles(o.Dir, res.files); err != nil {
		return err
	}
	// The v2 strategies are in flowseal/. Delete all v0.1 files of the root.
	for _, n := range names {
		if err := os.Remove(filepath.Join(o.Dir, n+".json")); err != nil {
			return err
		}
		fmt.Fprintf(w, "deleted %s.json\n", n)
	}
	fmt.Fprintf(w, "wrote %d files to %s\n", len(res.files), o.Dir)
	return nil
}

// upstreamTags gives the upstream tag of each strategy. The Photon strategy
// becomes game modules: it gets no tag.
func upstreamTags(repo upstream.Repo, names []string) (map[string]string, error) {
	tags := map[string]string{}
	for _, n := range names {
		if n == photonName {
			continue
		}
		tag, _, err := repo.Tag(n)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", n, err)
		}
		tags[n] = tag
	}
	return tags, nil
}

// readV01 reads the v0.1 strategy files of dir. The names are sorted.
func readV01(dir string) (map[string]*spec.V01Spec, []string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, nil, err
	}
	old := map[string]*spec.V01Spec{}
	var names []string
	for _, f := range files {
		base := filepath.Base(f)
		if base == spec.CatalogFile || base == spec.ListsFile || base == spec.TargetsFile || base == spec.ActionsFile {
			return nil, nil, fmt.Errorf("%s is in %s: the folder is v2 already", base, dir)
		}
		s, err := spec.LoadFile(f)
		if err != nil {
			return nil, nil, err
		}
		name := strings.TrimSuffix(base, ".json")
		old[name] = s
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil, nil, fmt.Errorf("no strategy files in %s", dir)
	}
	sort.Strings(names)
	return old, names, nil
}

// writeFiles writes the files (by slash path) into dir.
func writeFiles(dir string, files map[string][]byte) error {
	for name, data := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
