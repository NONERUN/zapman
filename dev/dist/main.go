// Command dist makes the folder of the release archive on the local
// computer, so that you can see what the archive will have.
//
// It does the steps of the Release workflow (.github/workflows/release.yml)
// on the committed files of a git ref, not on the working tree:
//
//  1. git archive of the ref into a temporary folder;
//  2. for a tag vMAJOR.MINOR.PATCH, the version info of the exe (go-winres);
//     for an other tag the committed .syso (0.0.0.0) stays;
//  3. go build with the flags of the workflow and app.Version = the tag;
//  4. the files of the bundle into <out>/zapman-<tag>/, without docs/dev.md;
//  5. with -zip, also <out>/zapman-<tag>.zip.
//
// The workflow also runs go test and makes the .rar and the .tar.gz. This
// tool does not. When you change the bundle in the workflow, change
// bundleFiles and bundleDirs here.
//
// Run it in the root of the repository:
//
//	go run ./dev/dist [-ref HEAD] [-tag v0.0.0-dist.<hash>] [-out .local/dist] [-zip]
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// The files and folders of the bundle, as in "Prepare release bundle" of the
// workflow. bundleDrop is removed from the bundle after the copy.
var (
	bundleFiles = []string{"README.md", "LICENSE.txt", "CHANGELOG.md", "assets/app.LICENSE.txt"}
	bundleDirs  = []string{"bin", "lists", "strategies", "docs"}
	bundleDrop  = []string{"docs/dev.md"}
)

// releaseTag is the tag format of the workflow.
var releaseTag = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

// options are the command line options.
type options struct {
	Ref string // git ref of the source
	Tag string // the version; empty: v0.0.0-dist.<short hash of Ref>
	Out string // folder of the bundle
	Zip bool   // also make the zip
}

func main() {
	var o options
	flag.StringVar(&o.Ref, "ref", "HEAD", "git ref of the source")
	flag.StringVar(&o.Tag, "tag", "", "the version (default: v0.0.0-dist.<short hash of the ref>)")
	flag.StringVar(&o.Out, "out", filepath.Join(".local", "dist"), "folder of the bundle")
	flag.BoolVar(&o.Zip, "zip", false, "also make zapman-<tag>.zip")
	flag.Parse()
	if err := run(o); err != nil {
		fmt.Fprintln(os.Stderr, "dist:", err)
		os.Exit(1)
	}
}

func run(o options) error {
	if _, err := os.Stat("go.mod"); err != nil {
		return errors.New("run the tool in the root of the repository")
	}
	hash, err := output("git", "rev-parse", "--short", o.Ref+"^{commit}")
	if err != nil {
		return err
	}
	if o.Tag == "" {
		o.Tag = "v0.0.0-dist." + hash
	}
	if strings.ContainsAny(o.Tag, `/\:`) {
		return fmt.Errorf("tag %q: a tag is a part of a file name", o.Tag)
	}

	tmp, err := os.MkdirTemp("", "zapman-dist-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp) //nolint:errcheck // a temporary folder.
	src := filepath.Join(tmp, "src")
	if err := archive(o.Ref, src); err != nil {
		return err
	}

	if releaseTag.MatchString(o.Tag) {
		v := strings.TrimPrefix(o.Tag, "v")
		err := command(filepath.Join(src, "cmd", "zapman"), "go", "tool", "-modfile=../../tools/go.mod",
			"go-winres", "make", "--in", "winres.json", "--arch", "amd64", "--out", "rsrc",
			"--file-version", v, "--product-version", v)
		if err != nil {
			return err
		}
	}

	out, err := filepath.Abs(o.Out)
	if err != nil {
		return err
	}
	name := "zapman-" + o.Tag
	dst := filepath.Join(out, name)
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	exe := filepath.Join(dst, "zapman.exe")
	err = command(src, "go", "build", "-tags", "production", "-trimpath",
		"-ldflags", "-s -w -H windowsgui -X github.com/NONERUN/zapman/internal/app.Version="+o.Tag,
		"-o", exe, "./cmd/zapman")
	if err != nil {
		return err
	}

	for _, f := range bundleFiles {
		if err := copyFile(filepath.Join(src, filepath.FromSlash(f)), filepath.Join(dst, filepath.FromSlash(f))); err != nil {
			return err
		}
	}
	for _, d := range bundleDirs {
		if err := copyDir(filepath.Join(src, d), filepath.Join(dst, d)); err != nil {
			return err
		}
	}
	for _, f := range bundleDrop {
		if err := os.Remove(filepath.Join(dst, filepath.FromSlash(f))); err != nil {
			return err
		}
	}

	sum, err := sha256File(exe)
	if err != nil {
		return err
	}
	fmt.Printf("%s  %s\n", sum, exe)
	if o.Zip {
		z := dst + ".zip"
		if err := zipDir(dst, name, z); err != nil {
			return err
		}
		if sum, err = sha256File(z); err != nil {
			return err
		}
		fmt.Printf("%s  %s\n", sum, z)
	}
	fmt.Println(dst)
	return nil
}

// archive writes the files of the git ref to the folder dir.
func archive(ref, dir string) error {
	cmd := exec.Command("git", "archive", "--format=tar", ref)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	r, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	xerr := untar(r, dir)
	_, _ = io.Copy(io.Discard, r)
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("git archive %s: %w: %s", ref, err, strings.TrimSpace(stderr.String()))
	}
	return xerr
}

// untar writes the regular files and the folders of the tar stream r to dir.
func untar(r io.Reader, dir string) error {
	tr := tar.NewReader(r)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if !filepath.IsLocal(filepath.FromSlash(h.Name)) {
			return fmt.Errorf("git archive: bad path %q", h.Name)
		}
		p := filepath.Join(dir, filepath.FromSlash(h.Name))
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(p, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := writeFile(p, tr); err != nil {
				return err
			}
		}
	}
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		return copyFile(p, filepath.Join(dst, rel))
	})
}

func copyFile(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // read-only file.
	return writeFile(dst, f)
}

func writeFile(path string, r io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, r)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// zipDir writes the folder dir to the zip file path. The names in the zip
// start with root/, as in the release zip.
func zipDir(dir, root, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		w, err := zw.Create(root + "/" + filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, in)
		if cerr := in.Close(); err == nil {
			err = cerr
		}
		return err
	})
	if cerr := zw.Close(); err == nil {
		err = cerr
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck // read-only file.
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// command runs name with args in the folder dir. The output goes to the
// output of the tool.
func command(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

// output runs name with args and gives the output without the line end.
func output(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(b)), nil
}
