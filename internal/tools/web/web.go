// Package web gets the files of the zapman repository from GitHub: the
// latest version tag, the IPSet list and the hosts template. It is the port
// of the web functions of Tools.ps1. The URLs are fields of Client, so the
// tests can use an httptest server.
package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// The URLs of the PowerShell version.
const (
	TagsURL     = "https://api.github.com/repos/NONERUN/zapman/tags?per_page=30"
	ReleasesURL = "https://github.com/NONERUN/zapman/releases/latest"
	IPSetURL    = "https://raw.githubusercontent.com/NONERUN/zapman/refs/heads/main/.service/ipset-service.txt"
	HostsURL    = "https://raw.githubusercontent.com/NONERUN/zapman/refs/heads/main/.service/hosts"
)

// UserAgent is the User-Agent of all downloads of the product.
const UserAgent = "zapman"

// The time limits. The PowerShell version has 8 s for the version check;
// WebClient of the downloads has 100 s.
const (
	VersionTimeout  = 8 * time.Second
	DownloadTimeout = 100 * time.Second
)

// The size limits of the bodies. The IPSet list is about 0.6 MB.
const (
	maxVersionBody = 1 << 20
	maxIPSetBody   = 64 << 20
	maxHostsBody   = 4 << 20
)

// ErrEmpty is the error when the server gives an empty file.
var ErrEmpty = errors.New("the downloaded file is empty")

// Progress gets the number of bytes that are done and the total number of
// bytes. The total is -1 when the server does not tell the size.
type Progress func(done, total int64)

// Client gets the files. Its zero value uses http.DefaultClient and the
// URLs of the PowerShell version.
type Client struct {
	HTTP *http.Client
	// The URLs; an empty URL is the constant of the same name.
	TagsURL, IPSetURL, HostsURL string
	// Nonce gives the value of the "t" query parameter of the hosts URL, so
	// that a cache does not give an old file. Nil is the current time in
	// nanoseconds.
	Nonce func() string
}

func (c *Client) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func or(s, def string) string {
	if s != "" {
		return s
	}
	return def
}

// HTTPError is the error of a response with a status that is not 200.
type HTTPError struct {
	URL    string
	Status string
}

func (e *HTTPError) Error() string { return fmt.Sprintf("GET %s: %s", e.URL, e.Status) }

// get reads the body of url, up to limit bytes, and gives the progress to
// p (p can be nil). A cancel of ctx gives an error that wraps the error of
// ctx.
func (c *Client) get(ctx context.Context, url, accept string, limit int64, p Progress) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("User-Agent", UserAgent)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := c.client().Do(req)
	if err != nil {
		return nil, ctxErr(ctx, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPError{URL: url, Status: resp.Status}
	}
	total := resp.ContentLength
	if total < 0 {
		total = -1
	}
	if total > limit {
		return nil, fmt.Errorf("GET %s: the file has %d bytes; the limit is %d", url, total, limit)
	}
	if p != nil {
		p(0, total)
	}
	var buf []byte
	chunk := make([]byte, 32<<10)
	for {
		n, err := resp.Body.Read(chunk)
		buf = append(buf, chunk[:n]...)
		if int64(len(buf)) > limit {
			return nil, fmt.Errorf("GET %s: the file is larger than %d bytes", url, limit)
		}
		if n > 0 && p != nil {
			p(int64(len(buf)), total)
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, ctxErr(ctx, fmt.Errorf("GET %s: %w", url, err))
		}
	}
	return buf, nil
}

// ctxErr makes sure that err wraps the error of ctx after a cancel or a
// timeout.
func ctxErr(ctx context.Context, err error) error {
	if c := ctx.Err(); c != nil && !errors.Is(err, c) {
		return fmt.Errorf("%w: %w", c, err)
	}
	return err
}

// DownloadIPSet downloads the IPSet list of the repository to dst (in the
// zapman layout, user\ipset-all.txt). It makes the folder of dst. It
// writes a temporary file in the same folder and then renames it, so dst
// is the old file or the full new file, never a part.
func (c *Client) DownloadIPSet(ctx context.Context, dst string, p Progress) error {
	ctx, cancel := context.WithTimeout(ctx, DownloadTimeout)
	defer cancel()
	body, err := c.get(ctx, or(c.IPSetURL, IPSetURL), "", maxIPSetBody, p)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return ErrEmpty
	}
	return WriteFileAtomic(dst, body)
}

// WriteFileAtomic writes data to a temporary file in the folder of path and
// renames it to path. It makes the folder when it is missing.
func WriteFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	ok := false
	defer func() {
		if !ok {
			_ = f.Close()
			_ = os.Remove(tmp)
		}
	}()
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	ok = true
	return nil
}
