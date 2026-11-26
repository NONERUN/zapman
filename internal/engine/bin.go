package engine

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// VersionsFile is the file in bin\ with the release tags and the SHA256 of
// the engine files. dev/update-zapret writes it.
const VersionsFile = "versions.json"

// Versions is the data of bin\versions.json.
type Versions struct {
	Updated string          `json:"updated"`
	Sources []VersionSource `json:"sources"`
	Files   []VersionFile   `json:"files"`
}

// VersionSource is the release that gave some engine files.
type VersionSource struct {
	ID        string `json:"id"`
	Repo      string `json:"repo"`
	Tag       string `json:"tag"`
	Published string `json:"published"`
	URL       string `json:"url"`
}

// VersionFile is one engine file. Path is relative to bin\ with "/".
type VersionFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Source string `json:"source"`
}

// RequiredExes are the files that bin\ must always have: both engines.
var RequiredExes = []string{"winws.exe", "winws2.exe"}

// FileState is the result of the test of one engine file.
type FileState int

// The file states.
const (
	// FileOK: the file is there and its SHA256 is correct.
	FileOK FileState = iota
	// FileMissing: versions.json has the file, but it is not in bin\.
	FileMissing
	// FileMismatch: the SHA256 of the file is not the SHA256 in
	// versions.json.
	FileMismatch
	// FileUnlisted: the file is required (RequiredExes), but
	// versions.json does not have it.
	FileUnlisted
)

func (s FileState) String() string {
	switch s {
	case FileOK:
		return "ok"
	case FileMissing:
		return "missing"
	case FileMismatch:
		return "hash mismatch"
	case FileUnlisted:
		return "not in versions.json"
	}
	return fmt.Sprintf("FileState(%d)", int(s))
}

// BinFile is the result for one file.
type BinFile struct {
	Path   string // relative to bin\, with "/"
	Source string // VersionFile.Source
	Want   string // SHA256 from versions.json
	Got    string // SHA256 of the file; empty when the file is missing
	State  FileState
}

// BinReport is the result of CheckBin, in the order of versions.json. The
// required exes that versions.json does not have go last.
type BinReport struct {
	Versions Versions
	Files    []BinFile
}

// OK tells if each file is FileOK.
func (r *BinReport) OK() bool { return len(r.Problems()) == 0 }

// Problems gives the files that are not FileOK.
func (r *BinReport) Problems() []BinFile {
	var out []BinFile
	for _, f := range r.Files {
		if f.State != FileOK {
			out = append(out, f)
		}
	}
	return out
}

// Err gives nil when OK, else an error that names each problem file.
func (r *BinReport) Err() error {
	p := r.Problems()
	if len(p) == 0 {
		return nil
	}
	parts := make([]string, len(p))
	for i, f := range p {
		parts[i] = f.Path + ": " + f.State.String()
	}
	return fmt.Errorf("the engine files in bin are not correct (%s). Update the engine files", strings.Join(parts, "; "))
}

// CheckBin examines the files in the folder bin against bin\versions.json.
// It does not use the network. The error is not nil only when
// versions.json is missing or not correct; the file problems are in the
// report.
func CheckBin(bin string) (*BinReport, error) {
	data, err := os.ReadFile(filepath.Join(bin, VersionsFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s is missing in %s. Update the engine files", VersionsFile, bin)
	}
	if err != nil {
		return nil, err
	}
	r := &BinReport{}
	if err := json.Unmarshal(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")), &r.Versions); err != nil {
		return nil, fmt.Errorf("%s is not correct: %w", VersionsFile, err)
	}
	listed := map[string]bool{}
	for _, vf := range r.Versions.Files {
		if !isRelSlash(vf.Path) {
			return nil, fmt.Errorf("%s: %q is not a relative path in bin", VersionsFile, vf.Path)
		}
		listed[strings.ToLower(vf.Path)] = true
		f := BinFile{Path: vf.Path, Source: vf.Source, Want: strings.ToLower(vf.SHA256)}
		f.Got, err = FileSHA256(filepath.Join(bin, filepath.FromSlash(vf.Path)))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			f.State = FileMissing
		case err != nil:
			return nil, err
		case f.Got != f.Want:
			f.State = FileMismatch
		}
		r.Files = append(r.Files, f)
	}
	for _, exe := range RequiredExes {
		if !listed[exe] {
			r.Files = append(r.Files, BinFile{Path: exe, State: FileUnlisted})
		}
	}
	return r, nil
}

// FileSHA256 gives the SHA256 (lowercase hex) of a file. For a .lua file
// it first changes CRLF and CR to LF, as dev/update-zapret does: git can
// change the line ends of these text files.
func FileSHA256(p string) (string, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	if strings.EqualFold(filepath.Ext(p), ".lua") {
		b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
		b = bytes.ReplaceAll(b, []byte("\r"), []byte("\n"))
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// isRelSlash tells if p is a clean relative path with "/" and no "..".
func isRelSlash(p string) bool {
	return p != "" && !strings.ContainsAny(p, `\:`) && path.Clean(p) == p &&
		!strings.HasPrefix(p, "/") && p != ".." && !strings.HasPrefix(p, "../")
}
