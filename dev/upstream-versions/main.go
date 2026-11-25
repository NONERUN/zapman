// Command upstream-versions compares the upstream tag of each Flowseal
// strategy with the upstream repository.
//
// For each strategies/flowseal/<name>.json it finds the last commit of the
// ref that changes <name>.bat, and the first upstream tag (by creator date)
// that has this commit. It writes a table: name, file tag, computed tag,
// status. Status is ok, outdated, no .bat upstream or no tag yet.
//
// With -write it sets upstream to the computed tag and rev to 1 in each
// file where the tag is different. Then do the sync of the strategy with
// the .bat file of that tag.
//
//	go run ./dev/upstream-versions [-git dir] [-ref upstream/main] [-dir strategies] [-write]
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/NONERUN/zapman/dev/internal/upstream"
	"github.com/NONERUN/zapman/internal/spec"
)

type options struct {
	Git   string // git folder with the upstream ref
	Ref   string // upstream ref
	Dir   string // catalog folder
	Write bool   // write the computed tags
}

func main() {
	var o options
	flag.StringVar(&o.Git, "git", ".", "git folder with the upstream ref")
	flag.StringVar(&o.Ref, "ref", upstream.DefaultRef, "upstream ref")
	flag.StringVar(&o.Dir, "dir", "strategies", "catalog folder")
	flag.BoolVar(&o.Write, "write", false, "set upstream to the computed tag and rev to 1 where the tag changed")
	flag.Parse()
	n, err := run(o, os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "upstream-versions:", err)
		os.Exit(1)
	}
	if n > 0 && !o.Write {
		os.Exit(2)
	}
}

// row is one line of the table.
type row struct {
	name, file, computed, status string
	path                         string
}

// The values of row.status.
const (
	statusOK       = "ok"
	statusOutdated = "outdated"
	statusNoBat    = "no .bat upstream"
	statusNoTag    = "no tag yet"
)

// run writes the table to w and gives the number of files that are not ok.
func run(o options, w io.Writer) (int, error) {
	rows, err := scan(o)
	if err != nil {
		return 0, err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "name\tfile tag\tcomputed tag\tstatus")
	bad := 0
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.name, dash(r.file), dash(r.computed), r.status)
		if r.status != statusOK {
			bad++
		}
	}
	if err := tw.Flush(); err != nil {
		return 0, err
	}
	if !o.Write {
		return bad, nil
	}
	for _, r := range rows {
		if r.status != statusOutdated {
			continue
		}
		if err := writeTag(r.path, r.computed); err != nil {
			return bad, err
		}
		fmt.Fprintf(w, "wrote %s: upstream %s, rev 1\n", r.path, r.computed)
	}
	return bad, nil
}

// scan reads the flowseal/ strategies and finds their upstream tags.
func scan(o options) ([]row, error) {
	files, err := filepath.Glob(filepath.Join(o.Dir, string(spec.OriginFlowseal), "*.json"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no strategies in %s", filepath.Join(o.Dir, string(spec.OriginFlowseal)))
	}
	sort.Strings(files)
	repo := upstream.Repo{Dir: o.Git, Ref: o.Ref}
	var rows []row
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var v struct {
			Upstream string `json:"upstream"`
		}
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		r := row{name: strings.TrimSuffix(filepath.Base(f), ".json"), file: v.Upstream, path: f}
		tag, _, err := repo.Tag(r.name)
		switch {
		case errors.Is(err, upstream.ErrNoBat):
			r.status = statusNoBat
		case errors.Is(err, upstream.ErrNoTag):
			r.status = statusNoTag
		case err != nil:
			return nil, err
		case tag == v.Upstream:
			r.computed, r.status = tag, statusOK
		default:
			r.computed, r.status = tag, statusOutdated
		}
		rows = append(rows, r)
	}
	return rows, nil
}

var (
	upstreamRe = regexp.MustCompile(`(?m)^(\s*"upstream":\s*)"[^"]*"`)
	revRe      = regexp.MustCompile(`(?m)^(\s*"rev":\s*)[0-9]+`)
	specRe     = regexp.MustCompile(`(?m)^(\s*)("spec":\s*[0-9]+,)(\r?\n)`)
)

// writeTag sets upstream to tag and rev to 1 in the strategy file. It
// changes only these values and keeps the format of the file. When the
// fields are not in the file, it adds them after "spec".
func writeTag(path, tag string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text, err := setTag(string(data), tag)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

func setTag(text, tag string) (string, error) {
	hasUp, hasRev := upstreamRe.MatchString(text), revRe.MatchString(text)
	if hasUp != hasRev {
		return "", errors.New("the file has only one of upstream and rev")
	}
	if !hasUp {
		if !specRe.MatchString(text) {
			return "", errors.New(`no "spec" line`)
		}
		return specRe.ReplaceAllString(text, `${1}${2}${3}${1}"upstream": "`+tag+`",${3}${1}"rev": 1,${3}`), nil
	}
	text = upstreamRe.ReplaceAllString(text, `${1}"`+tag+`"`)
	return revRe.ReplaceAllString(text, `${1}1`), nil
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
