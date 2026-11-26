package web

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// HostsCheck is the result of CheckHosts.
type HostsCheck struct {
	// NeedsUpdate is true when the system hosts file does not have the
	// first or the last line of the template.
	NeedsUpdate bool
	// Template is the full text of the template, for the clipboard.
	Template string
}

// HostsPath gives %SystemRoot%\System32\drivers\etc\hosts.
func HostsPath() string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return filepath.Join(root, "System32", "drivers", "etc", "hosts")
}

// HostsTemplate downloads the hosts template of the repository.
func (c *Client) HostsTemplate(ctx context.Context, p Progress) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, DownloadTimeout)
	defer cancel()
	nonce := c.Nonce
	if nonce == nil {
		nonce = func() string { return strconv.FormatInt(time.Now().UnixNano(), 10) }
	}
	body, err := c.get(ctx, or(c.HostsURL, HostsURL)+"?t="+nonce(), "", maxHostsBody, p)
	if err != nil {
		return "", err
	}
	if len(body) == 0 {
		return "", ErrEmpty
	}
	return string(body), nil
}

// CheckHosts downloads the template and compares it with the hosts file at
// hostsPath (in the app: HostsPath()). A missing hosts file is empty.
func (c *Client) CheckHosts(ctx context.Context, hostsPath string, p Progress) (HostsCheck, error) {
	tpl, err := c.HostsTemplate(ctx, p)
	if err != nil {
		return HostsCheck{}, err
	}
	data, err := os.ReadFile(hostsPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return HostsCheck{}, err
	}
	return HostsCheck{NeedsUpdate: HostsNeedsUpdate(tpl, string(data)), Template: tpl}, nil
}

// HostsNeedsUpdate is the check of Get-ZapretHostsUpdateInfo: true when the
// template has a line and hosts does not contain its first or its last
// non-empty line. The compare ignores the case, as -like.
func HostsNeedsUpdate(template, hosts string) bool {
	var lines []string
	for l := range strings.Lines(template) {
		l = strings.TrimRight(l, "\r\n")
		if l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		return false
	}
	h := strings.ToLower(hosts)
	first := strings.ToLower(lines[0])
	last := strings.ToLower(lines[len(lines)-1])
	return !strings.Contains(h, first) || !strings.Contains(h, last)
}
