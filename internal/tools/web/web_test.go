package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// server serves body at all paths and records the last request.
func server(t *testing.T, status int, body string) (*httptest.Server, *atomic.Pointer[http.Request]) {
	t.Helper()
	var last atomic.Pointer[http.Request]
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		last.Store(r)
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(s.Close)
	return s, &last
}

func TestVersionFromBody(t *testing.T) {
	for _, tc := range []struct{ body, want string }{
		{"", ""},
		{"  ", ""},
		{`[{"name":"v0.1.0"},{"name":"v0.10.2"},{"name":"v0.9.9"},{"name":"latest"},{"name":"V0.2.0"}]`, "v0.10.2"},
		{`[{"name":"nightly"}]`, ""},
		{`[]`, ""},
		{`[broken`, ""},
		{`{"tag_name":"0.3.1"}`, "v0.3.1"},
		{`{"name":"x"}`, ""},
		{"0.4.0\nmore", "v0.4.0"},
		{"V1.2.3", "v1.2.3"},
	} {
		if got := VersionFromBody(tc.body); got != tc.want {
			t.Errorf("VersionFromBody(%q) = %q, want %q", tc.body, got, tc.want)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{
		{"v0.1.1", "0.1.1", 0},
		{"v0.1.1", "v0.1.2", -1},
		{"v0.10.0", "v0.9.9", 1},
		{"bad", "v0.0.0", -1},
		{"v0.0.1", "bad", 1},
		{"x", "y", 0},
	} {
		if got := CompareVersions(tc.a, tc.b); got != tc.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestLatestVersion(t *testing.T) {
	s, last := server(t, http.StatusOK, `[{"name":"v0.1.1"},{"name":"v0.2.0"}]`)
	c := &Client{HTTP: s.Client(), TagsURL: s.URL + "/tags"}
	v, err := c.LatestVersion(t.Context())
	if err != nil || v != "v0.2.0" {
		t.Fatalf("LatestVersion = %q, %v", v, err)
	}
	r := last.Load()
	if r.Header.Get("User-Agent") != "zapman" || r.Header.Get("Cache-Control") != "no-cache" || r.Header.Get("Accept") != "application/vnd.github+json" {
		t.Errorf("headers = %v", r.Header)
	}
}

func TestLatestVersionErrors(t *testing.T) {
	s, _ := server(t, http.StatusForbidden, "rate limit")
	c := &Client{HTTP: s.Client(), TagsURL: s.URL}
	var he *HTTPError
	if _, err := c.LatestVersion(t.Context()); !errors.As(err, &he) {
		t.Errorf("403: err = %v", err)
	}
	s2, _ := server(t, http.StatusOK, `[]`)
	c = &Client{HTTP: s2.Client(), TagsURL: s2.URL}
	if _, err := c.LatestVersion(t.Context()); !errors.Is(err, ErrNoVersion) {
		t.Errorf("no tags: err = %v", err)
	}
}

func TestCancel(t *testing.T) {
	block := make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("1.2.3.0/24\n"))
		w.(http.Flusher).Flush()
		<-block
	}))
	defer s.Close()
	defer close(block)
	ctx, cancel := context.WithCancel(t.Context())
	c := &Client{HTTP: s.Client(), IPSetURL: s.URL}
	dst := filepath.Join(t.TempDir(), "user", "ipset-all.txt")
	err := c.DownloadIPSet(ctx, dst, func(done, _ int64) {
		if done > 0 {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(dst); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("dst exists after a cancel: %v", err)
	}
	ents, _ := os.ReadDir(filepath.Dir(dst))
	if len(ents) != 0 {
		t.Errorf("files left: %v", ents)
	}
}

func TestDownloadIPSet(t *testing.T) {
	body := strings.Repeat("10.0.0.0/8\n", 10000)
	s, last := server(t, http.StatusOK, body)
	dst := filepath.Join(t.TempDir(), "user", "ipset-all.txt")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	var calls int
	var lastDone, lastTotal int64
	c := &Client{HTTP: s.Client(), IPSetURL: s.URL + "/ipset"}
	if err := c.DownloadIPSet(t.Context(), dst, func(done, total int64) {
		calls++
		lastDone, lastTotal = done, total
	}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dst)
	if err != nil || string(got) != body {
		t.Fatalf("file: %d bytes, %v", len(got), err)
	}
	if calls < 2 || lastDone != int64(len(body)) || lastTotal != int64(len(body)) {
		t.Errorf("progress: %d calls, last %d/%d", calls, lastDone, lastTotal)
	}
	if last.Load().URL.Path != "/ipset" {
		t.Errorf("path = %s", last.Load().URL.Path)
	}

	s2, _ := server(t, http.StatusNotFound, "no")
	c = &Client{HTTP: s2.Client(), IPSetURL: s2.URL}
	if err := c.DownloadIPSet(t.Context(), dst, nil); err == nil {
		t.Error("404: no error")
	}
	if got, _ := os.ReadFile(dst); string(got) != body {
		t.Error("404 changed the file")
	}
	s3, _ := server(t, http.StatusOK, "")
	c = &Client{HTTP: s3.Client(), IPSetURL: s3.URL}
	if err := c.DownloadIPSet(t.Context(), dst, nil); !errors.Is(err, ErrEmpty) {
		t.Errorf("empty: err = %v", err)
	}
}

func TestHostsNeedsUpdate(t *testing.T) {
	tpl := "\n# zapman start\n1.2.3.4 discord.media\n\n# zapman end\n\n"
	for _, tc := range []struct {
		tpl, hosts string
		want       bool
	}{
		{tpl, "", true},
		{tpl, "127.0.0.1 localhost\n# ZAPMAN START\nx\n# zapman end\n", false},
		{tpl, "# zapman start\n", true},
		{tpl, "# zapman end\n", true},
		{"\r\n\r\n", "", false},
		{"[a]*\r\n", "x [A]* y", false},
	} {
		if got := HostsNeedsUpdate(tc.tpl, tc.hosts); got != tc.want {
			t.Errorf("HostsNeedsUpdate(%q, %q) = %v", tc.tpl, tc.hosts, got)
		}
	}
}

func TestCheckHosts(t *testing.T) {
	tpl := "# start\r\n1.1.1.1 a.example\r\n# end\r\n"
	s, last := server(t, http.StatusOK, tpl)
	c := &Client{HTTP: s.Client(), HostsURL: s.URL + "/hosts", Nonce: func() string { return "n1" }}
	dir := t.TempDir()
	hosts := filepath.Join(dir, "hosts")
	hc, err := c.CheckHosts(t.Context(), hosts, nil)
	if err != nil || !hc.NeedsUpdate || hc.Template != tpl {
		t.Fatalf("missing hosts: %+v, %v", hc, err)
	}
	if q := last.Load().URL.Query().Get("t"); q != "n1" {
		t.Errorf("t = %q", q)
	}
	if err := os.WriteFile(hosts, []byte("127.0.0.1 localhost\r\n"+tpl), 0o644); err != nil {
		t.Fatal(err)
	}
	hc, err = c.CheckHosts(t.Context(), hosts, nil)
	if err != nil || hc.NeedsUpdate {
		t.Fatalf("hosts with template: %+v, %v", hc, err)
	}
}

func TestTimeout(t *testing.T) {
	block := make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-block }))
	defer s.Close()
	defer close(block)
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	c := &Client{HTTP: s.Client(), TagsURL: s.URL}
	if _, err := c.LatestVersion(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v", err)
	}
}

func TestHostsPath(t *testing.T) {
	t.Setenv("SystemRoot", filepath.FromSlash("/win"))
	if got, want := HostsPath(), filepath.Join(filepath.FromSlash("/win"), "System32", "drivers", "etc", "hosts"); got != want {
		t.Errorf("HostsPath = %q, want %q", got, want)
	}
}
