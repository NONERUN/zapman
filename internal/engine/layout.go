// Package engine finds the folders of zapman, examines bin\ against
// bin\versions.json, builds the command line of winws.exe or winws2.exe and
// runs it in the foreground.
//
// The package does not know about the Windows service. Before a foreground
// run, the caller must make sure that the service is stopped: two engines
// on the same traffic do not work.
package engine

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/NONERUN/zapman/internal/config"
	"github.com/NONERUN/zapman/internal/spec"
)

// Layout is the set of folders of one zapman folder.
type Layout struct {
	Root       string
	Bin        string // engine files and versions.json
	Strategies string // the stock catalog
	Lists      string // the stock lists
	User       string // the user data
}

// NewLayout gives the layout of the zapman folder root.
func NewLayout(root string) Layout {
	root = filepath.Clean(root)
	return Layout{
		Root:       root,
		Bin:        filepath.Join(root, "bin"),
		Strategies: filepath.Join(root, "strategies"),
		Lists:      filepath.Join(root, "lists"),
		User:       filepath.Join(root, "user"),
	}
}

// ConfigPath gives the path of user\config.json.
func (l Layout) ConfigPath() string { return filepath.Join(l.User, config.FileName) }

// VersionsPath gives the path of bin\versions.json.
func (l Layout) VersionsPath() string { return filepath.Join(l.Bin, VersionsFile) }

// ExeName gives the file name of the engine e.
func ExeName(e spec.Engine) (string, error) {
	switch e {
	case spec.Winws:
		return "winws.exe", nil
	case spec.Winws2:
		return "winws2.exe", nil
	}
	return "", fmt.Errorf("%q is not an engine. Use winws or winws2", e)
}

// Exe gives the path of the exe of the engine e. When the file is missing,
// it gives an error. It does not use the other engine.
func (l Layout) Exe(e spec.Engine) (string, error) {
	name, err := ExeName(e)
	if err != nil {
		return "", err
	}
	p := filepath.Join(l.Bin, name)
	st, err := os.Stat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("%s is missing. Update the engine files in %s", name, l.Bin)
	}
	if err != nil {
		return "", err
	}
	if st.IsDir() {
		return "", fmt.Errorf("%s is a folder, not a file", p)
	}
	return p, nil
}

// LoadCatalog loads the stock catalog. Each bin: ref must be a file in bin\.
func (l Layout) LoadCatalog() (*spec.Catalog, error) {
	return spec.LoadCatalog(l.Strategies, l.Bin)
}

// Paths gives the folders for spec.Build. ActiveFiles has the names X of
// the files user\fakes\ACTIVE_<X>.bin. The test of the prefix and of the
// extension is not case-sensitive; X must be an active name
// (spec.IsActiveName). A missing fakes folder gives no active files.
func (l Layout) Paths() (spec.Paths, error) {
	p := spec.Paths{Bin: l.Bin, Lists: l.Lists, User: l.User, Catalog: l.Strategies}
	entries, err := os.ReadDir(filepath.Join(l.User, spec.ActiveFakesDir))
	if errors.Is(err, fs.ErrNotExist) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		if name, ok := activeName(e.Name()); ok {
			if p.ActiveFiles == nil {
				p.ActiveFiles = map[string]bool{}
			}
			p.ActiveFiles[name] = true
		}
	}
	return p, nil
}

// activeName gives X of the file name ACTIVE_<X>.bin.
func activeName(file string) (string, bool) {
	const pre, ext = "ACTIVE_", ".bin"
	if len(file) <= len(pre)+len(ext) ||
		!strings.EqualFold(file[:len(pre)], pre) ||
		!strings.EqualFold(file[len(file)-len(ext):], ext) {
		return "", false
	}
	name := file[len(pre) : len(file)-len(ext)]
	return name, spec.IsActiveName(name)
}

// The contents of a new user list. An empty hostlist or ipset matches all
// hosts, thus a new list has one address that is not used.
const (
	newHostlist = "# Do not make this file empty: an empty list matches all hosts.\r\ndomain.example.abc\r\n"
	newIpset    = "203.0.113.113/32\r\n"
)

// EnsureUserFiles makes the user: lists of the catalog bundles and of the
// games that are missing. It does not change a file that exists. The
// contents of a new file come from:
//   - the seed of the game bundle (a lists: file);
//   - else lists\<name>.default.txt, when it exists (user:ipset-all.txt
//     gets lists\ipset-all.default.txt);
//   - else one address that is not used (see newHostlist and newIpset).
//
// It also makes the missing active fakes (EnsureActiveFakes).
func (l Layout) EnsureUserFiles(cat *spec.Catalog, games []*spec.Game) error {
	type want struct {
		ref, seed string
		ipset     bool
	}
	var wants []want
	add := func(b *spec.Bundle, seed string) {
		for _, r := range b.Hostlist {
			wants = append(wants, want{r, seed, false})
		}
		for _, r := range b.HostlistExclude {
			wants = append(wants, want{r, seed, false})
		}
		for _, r := range b.Ipset {
			wants = append(wants, want{r, seed, true})
		}
		for _, r := range b.IpsetExclude {
			wants = append(wants, want{r, seed, true})
		}
	}
	for _, b := range cat.Lists {
		add(&b, "")
	}
	for _, g := range games {
		for _, b := range g.Lists {
			add(&b.Bundle, b.Seed)
		}
	}
	var errs []error
	for _, w := range wants {
		name, ok := strings.CutPrefix(w.ref, spec.PrefixUser)
		if !ok {
			continue
		}
		if err := l.ensureUserFile(name, w.seed, w.ipset); err != nil {
			errs = append(errs, err)
		}
	}
	errs = append(errs, l.EnsureActiveFakes(cat))
	return errors.Join(errs...)
}

// EnsureActiveFakes makes the missing files user\fakes\ACTIVE_<X>.bin. A
// new file is a copy of the default of X in catalog.json "active". It does
// not change a file that exists (Paths finds it, the test of the name is
// not case-sensitive): the user selected that file.
func (l Layout) EnsureActiveFakes(cat *spec.Catalog) error {
	p, err := l.Paths()
	if err != nil {
		return err
	}
	var errs []error
	for _, name := range slices.Sorted(maps.Keys(cat.Active)) {
		if p.ActiveFiles[name] {
			continue
		}
		src, err := l.CatalogFile(cat.Active[name])
		if err != nil {
			errs = append(errs, fmt.Errorf("default of active fake %s: %w", name, err))
			continue
		}
		data, err := os.ReadFile(src)
		if err != nil {
			errs = append(errs, fmt.Errorf("default of active fake %s: %w", name, err))
			continue
		}
		dst := filepath.Join(l.User, spec.ActiveFakesDir, "ACTIVE_"+name+".bin")
		if err := writeNew(dst, data); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// CatalogFile gives the path of the cat: ref in the stock catalog folder.
func (l Layout) CatalogFile(ref string) (string, error) {
	name, ok := strings.CutPrefix(ref, spec.PrefixCat)
	if !ok {
		return "", fmt.Errorf("%q is not a cat: ref", ref)
	}
	path := filepath.Join(l.Strategies, filepath.FromSlash(name))
	if !isInside(l.Strategies, path) {
		return "", fmt.Errorf("%q is not in the catalog folder", ref)
	}
	return path, nil
}

func (l Layout) ensureUserFile(name, seed string, ipset bool) error {
	path := filepath.Join(l.User, filepath.FromSlash(name))
	if !isInside(l.User, path) {
		return fmt.Errorf("user list %q is not in the user folder", name)
	}
	if _, err := os.Stat(path); err == nil || !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	var data []byte
	src := ""
	if s, ok := strings.CutPrefix(seed, spec.PrefixLists); ok {
		src = filepath.Join(l.Lists, filepath.FromSlash(s))
	} else {
		def := filepath.Join(l.Lists, strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))+".default.txt")
		if _, err := os.Stat(def); err == nil {
			src = def
		}
	}
	switch {
	case src != "":
		b, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("make %s: %w", name, err)
		}
		data = b
	case ipset:
		data = []byte(newIpset)
	default:
		data = []byte(newHostlist)
	}
	return writeNew(path, data)
}

// writeNew makes the file path with data. When the file exists, it does
// nothing.
func writeNew(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// O_EXCL: do not write over a file that an other process made.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// isInside tells if path is in the folder dir (not dir itself).
func isInside(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != "." && !filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
