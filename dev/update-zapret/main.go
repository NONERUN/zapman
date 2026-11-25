// Command update-zapret downloads the official zapret and zapret2 release
// zips and copies the engine files (exe, dll, sys, lua) into bin/. It
// writes the tags and the SHA256 of the files to bin/versions.json. The
// zips stay in .local/releases/ as a cache.
//
//	go run ./dev/update-zapret [-zapret-tag tag] [-zapret2-tag tag] [-dry-run] [-force]
//
// Without a tag it takes the latest release. When the tags in
// bin/versions.json are already the same, it does nothing; -force extracts
// the files again and writes bin/versions.json again. Before it replaces
// the files, it stops the zapret service and the winws / winws2 processes.
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// userAgent is the User-Agent of the GitHub requests.
const userAgent = "zapret-dev-update"

type options struct {
	ZapretTag  string // zapret tag; empty: latest
	Zapret2Tag string // zapret2 tag; empty: latest
	DryRun     bool   // show the tags only
	Force      bool   // download and extract again
	Bin        string // bin folder
	Cache      string // cache folder of the zips
}

func main() {
	var o options
	flag.StringVar(&o.ZapretTag, "zapret-tag", "", "zapret release tag; empty: latest")
	flag.StringVar(&o.Zapret2Tag, "zapret2-tag", "", "zapret2 release tag; empty: latest")
	flag.BoolVar(&o.DryRun, "dry-run", false, "show the tags, do not copy files")
	flag.BoolVar(&o.Force, "force", false, "download and extract again, write versions.json again")
	flag.StringVar(&o.Bin, "bin", "bin", "bin folder")
	flag.StringVar(&o.Cache, "cache", filepath.Join(".local", "releases"), "cache folder of the release zips")
	flag.Parse()
	if err := run(o, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "update-zapret:", err)
		os.Exit(1)
	}
}

// release is the part of a GitHub release that the tool uses.
type release struct {
	TagName     string  `json:"tag_name"`
	HTMLURL     string  `json:"html_url"`
	PublishedAt string  `json:"published_at"`
	Assets      []asset `json:"assets"`
}

type asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

// Manifest is bin/versions.json.
type Manifest struct {
	Updated string   `json:"updated"`
	Sources []Source `json:"sources"`
	Files   []File   `json:"files"`
}

type Source struct {
	ID        string `json:"id"`
	Repo      string `json:"repo"`
	Tag       string `json:"tag"`
	Published string `json:"published"`
	URL       string `json:"url"`
}

type File struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Source string `json:"source"`
}

// engineFile is a file of a release zip that goes to bin/.
type engineFile struct {
	Source string // source id: zapret or zapret2
	Suffix string // end of the zip entry name
	Dest   string // path in bin/, with /
}

// engineFiles are the only files that go to bin/.
var engineFiles = []engineFile{
	{"zapret", "/binaries/windows-x86_64/winws.exe", "winws.exe"},
	{"zapret", "/binaries/windows-x86_64/cygwin1.dll", "cygwin1.dll"},
	{"zapret", "/binaries/windows-x86_64/WinDivert.dll", "WinDivert.dll"},
	{"zapret", "/binaries/windows-x86_64/WinDivert64.sys", "WinDivert64.sys"},
	{"zapret2", "/binaries/windows-x86_64/winws2.exe", "winws2.exe"},
	{"zapret2", "/lua/zapret-lib.lua", "lua/zapret-lib.lua"},
	{"zapret2", "/lua/zapret-antidpi.lua", "lua/zapret-antidpi.lua"},
}

// sourceRepos are the GitHub repositories of the sources, in the order of
// versions.json.
var sourceRepos = []struct{ ID, Repo string }{
	{"zapret", "bol-van/zapret"},
	{"zapret2", "bol-van/zapret2"},
}

func run(o options, w io.Writer) error {
	if st, err := os.Stat(o.Bin); err != nil || !st.IsDir() {
		return errors.New("the bin folder is not found")
	}
	manifestPath := filepath.Join(o.Bin, "versions.json")
	pinned := readManifest(manifestPath)
	tags := map[string]string{"zapret": o.ZapretTag, "zapret2": o.Zapret2Tag}
	rels := map[string]*release{}
	same := true
	for _, s := range sourceRepos {
		rel, err := getRelease(s.Repo, tags[s.ID])
		if err != nil {
			return err
		}
		rels[s.ID] = rel
		old := pinned.tag(s.ID)
		fmt.Fprintf(w, "%-9s%s -> %s  %s\n", s.ID+":", old, rel.TagName, rel.HTMLURL)
		if old != rel.TagName {
			same = false
		}
	}
	if o.DryRun {
		fmt.Fprintln(w, "Dry run. No files copied.")
		return nil
	}
	if same && !o.Force {
		fmt.Fprintln(w, "Pinned tags already match. Use -force to extract again and rewrite versions.json.")
		return nil
	}
	zips := map[string]string{}
	sums := map[string]map[string]string{}
	for _, s := range sourceRepos {
		rel := rels[s.ID]
		a, err := zipAsset(rel, s.ID)
		if err != nil {
			return err
		}
		dir := filepath.Join(o.Cache, s.ID+"-"+rel.TagName)
		path := filepath.Join(dir, a.Name)
		if err := cachedAsset(a, path, o.Force, w); err != nil {
			return err
		}
		zips[s.ID] = path
		sa, ok := sumsAsset(rel)
		if !ok {
			fmt.Fprintf(w, "%s %s has no %s: the files are not compared with it.\n", s.ID, rel.TagName, sumsName)
			continue
		}
		sp := filepath.Join(dir, sa.Name)
		if err := cachedAsset(sa, sp, o.Force, w); err != nil {
			return err
		}
		b, err := os.ReadFile(sp)
		if err != nil {
			return err
		}
		sums[s.ID] = parseSums(b)
	}
	stopBinLocks(w)
	files, err := extract(zips, sums, o.Bin, w)
	if err != nil {
		return err
	}
	m := Manifest{Updated: time.Now().Format("2006-01-02"), Files: files}
	for _, s := range sourceRepos {
		rel := rels[s.ID]
		m.Sources = append(m.Sources, Source{
			ID: s.ID, Repo: "https://github.com/" + s.Repo, Tag: rel.TagName,
			Published: rel.PublishedAt, URL: rel.HTMLURL,
		})
	}
	if err := writeManifest(manifestPath, m); err != nil {
		return err
	}
	fmt.Fprintln(w, "Wrote", manifestPath)
	fmt.Fprintln(w, "Tags are in bin/versions.json. Note a bump in CHANGELOG.md.")
	return nil
}

// readManifest reads bin/versions.json. A missing or bad file gives an
// empty manifest: then no tag is pinned.
func readManifest(path string) *Manifest {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var m Manifest
	if json.Unmarshal(b, &m) != nil {
		return nil
	}
	return &m
}

// tag gives the pinned tag of the source id, or "".
func (m *Manifest) tag(id string) string {
	if m == nil {
		return ""
	}
	for _, s := range m.Sources {
		if s.ID == id {
			return s.Tag
		}
	}
	return ""
}

// marshalManifest gives the text of bin/versions.json.
func marshalManifest(m Manifest) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func writeManifest(path string, m Manifest) error {
	b, err := marshalManifest(m)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func getRelease(repo, tag string) (*release, error) {
	url := "https://api.github.com/repos/" + repo + "/releases/latest"
	if strings.TrimSpace(tag) != "" {
		url = "https://api.github.com/repos/" + repo + "/releases/tags/" + tag
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	var rel release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("%s: %w", url, err)
	}
	if rel.TagName == "" {
		return nil, fmt.Errorf("%s: no tag_name", url)
	}
	return &rel, nil
}

// zipAsset gives the first asset <prefix>-*.zip of the release that is not
// an OpenWrt build.
func zipAsset(rel *release, prefix string) (asset, error) {
	for _, a := range rel.Assets {
		ok, _ := filepath.Match(prefix+"-*.zip", a.Name)
		if ok && !strings.Contains(a.Name, "openwrt") {
			return a, nil
		}
	}
	return asset{}, fmt.Errorf("no zip asset for %s in %s", prefix, rel.TagName)
}

// cacheHit tells if the file at path is a complete download of a: the size
// is the asset size (or the asset has no size).
func cacheHit(path string, a asset) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular() && (a.Size < 1 || st.Size() == a.Size)
}

// cachedAsset downloads the asset to path, unless path is a cache hit and
// force is false.
func cachedAsset(a asset, path string, force bool, w io.Writer) error {
	if !force && cacheHit(path, a) {
		fmt.Fprintln(w, "Cache hit:", path)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	fmt.Fprintln(w, "Download:", a.URL)
	req, err := http.NewRequest(http.MethodGet, a.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", a.URL, resp.Status)
	}
	tmp := path + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, resp.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && !cacheHit(tmp, a) {
		err = fmt.Errorf("%s: the size is not %d bytes", a.URL, a.Size)
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// findEntry gives the first file entry of the zip whose name ends with
// suffix. The compare ignores case and the slash direction.
func findEntry(z *zip.Reader, suffix string) *zip.File {
	want := strings.ToLower(strings.ReplaceAll(suffix, `\`, "/"))
	for _, f := range z.File {
		name := strings.ReplaceAll(f.Name, `\`, "/")
		if strings.HasSuffix(name, "/") {
			continue
		}
		if strings.HasSuffix(strings.ToLower(name), want) {
			return f
		}
	}
	return nil
}

// sumsName is the release asset with the SHA256 of the release files.
const sumsName = "sha256sum.txt"

// sumsAsset gives the sumsName asset of the release.
func sumsAsset(rel *release) (asset, bool) {
	for _, a := range rel.Assets {
		if a.Name == sumsName {
			return a, true
		}
	}
	return asset{}, false
}

// parseSums reads the lines "<sha256>  <path>" of sumsName. The map key is
// the path in lower case with /. A "*" before the path (binary mode) is
// removed.
func parseSums(b []byte) map[string]string {
	sums := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) != 2 {
			continue
		}
		p := strings.TrimPrefix(f[1], "*")
		sums[strings.ToLower(strings.ReplaceAll(p, `\`, "/"))] = strings.ToLower(f[0])
	}
	return sums
}

// extract copies engineFiles from the zips (source id → zip path) into bin
// and gives the entries of versions.json. sums (source id → parseSums map)
// is optional. When an entry is in the sums of its source, the SHA256 must
// be the same. All entries are read and compared before bin is changed.
func extract(zips map[string]string, sums map[string]map[string]string, bin string, w io.Writer) ([]File, error) {
	readers := map[string]*zip.ReadCloser{}
	defer func() {
		for _, r := range readers {
			r.Close()
		}
	}()
	data := make([][]byte, len(engineFiles))
	var files []File
	for i, ef := range engineFiles {
		r, ok := readers[ef.Source]
		if !ok {
			var err error
			r, err = zip.OpenReader(zips[ef.Source])
			if err != nil {
				return nil, err
			}
			readers[ef.Source] = r
		}
		name, b, err := readEntry(&r.Reader, ef.Suffix)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", zips[ef.Source], err)
		}
		h := sha256.Sum256(b)
		sum := hex.EncodeToString(h[:])
		if s := sums[ef.Source]; s != nil {
			want, ok := s[strings.ToLower(name)]
			switch {
			case !ok:
				fmt.Fprintf(w, "%s is not in %s of %s: not compared.\n", name, sumsName, ef.Source)
			case want != sum:
				return nil, fmt.Errorf("%s: SHA256 %s, %s of %s has %s", name, sum, sumsName, ef.Source, want)
			}
		}
		data[i] = b
		files = append(files, File{Path: ef.Dest, SHA256: sum, Source: ef.Source})
	}
	for i, ef := range engineFiles {
		dest := filepath.Join(bin, filepath.FromSlash(ef.Dest))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(dest, data[i], 0o644); err != nil {
			return nil, err
		}
		fmt.Fprintf(w, "OK %s  %s\n", ef.Dest, files[i].SHA256)
	}
	return files, nil
}

// readEntry gives the name (with /) and the data of the zip entry that ends
// with suffix.
func readEntry(z *zip.Reader, suffix string) (string, []byte, error) {
	f := findEntry(z, suffix)
	if f == nil {
		return "", nil, fmt.Errorf("zip has no entry ending with %s", suffix)
	}
	src, err := f.Open()
	if err != nil {
		return "", nil, err
	}
	defer src.Close()
	b, err := io.ReadAll(src)
	if err != nil {
		return "", nil, err
	}
	return strings.ReplaceAll(f.Name, `\`, "/"), b, nil
}

// stopBinLocks stops the zapret service and the winws / winws2 processes,
// so that the files in bin/ can be replaced.
func stopBinLocks(w io.Writer) {
	if runtime.GOOS != "windows" {
		return
	}
	out, err := exec.Command("sc.exe", "query", "zapret").Output()
	if err == nil && !strings.Contains(string(out), "STOPPED") {
		fmt.Fprintln(w, "Stopping zapret service so bin files can be replaced.")
		_ = exec.Command("net.exe", "stop", "zapret").Run()
	}
	running := false
	for _, name := range []string{"winws.exe", "winws2.exe"} {
		out, err := exec.Command("tasklist.exe", "/FI", "IMAGENAME eq "+name, "/NH", "/FO", "CSV").Output()
		if err == nil && strings.Contains(strings.ToLower(string(out)), `"`+name+`"`) {
			running = true
		}
	}
	if running {
		fmt.Fprintln(w, "Stopping winws / winws2.")
		_ = exec.Command("taskkill.exe", "/F", "/IM", "winws.exe", "/IM", "winws2.exe").Run()
		time.Sleep(500 * time.Millisecond)
	}
}

// fileSHA256 gives the SHA256 of a file in bin/ as versions.json has it.
// A .lua file is hashed with LF line ends: the zip has LF, and a checkout
// on Windows can have CRLF.
func fileSHA256(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if strings.EqualFold(filepath.Ext(path), ".lua") {
		b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
		b = bytes.ReplaceAll(b, []byte("\r"), []byte("\n"))
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
