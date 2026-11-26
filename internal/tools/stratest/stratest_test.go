package stratest

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NONERUN/zapman/internal/spec"
	"github.com/NONERUN/zapman/internal/tools/command"
)

type fakeRun struct{ e *fakeEngine }

func (r fakeRun) Stop() error {
	r.e.mu.Lock()
	defer r.e.mu.Unlock()
	r.e.stops++
	return nil
}

type fakeEngine struct {
	mu       sync.Mutex
	names    []string
	namesErr error
	fail     map[string]bool
	started  []string
	stops    int
	stopAll  int
	onStart  func(name string)
}

func (e *fakeEngine) Names(spec.Engine) ([]string, error) { return e.names, e.namesErr }

func (e *fakeEngine) Start(_ context.Context, name string, _ spec.Engine) (Running, error) {
	e.mu.Lock()
	e.started = append(e.started, name)
	e.mu.Unlock()
	if e.onStart != nil {
		e.onStart(name)
	}
	if e.fail[name] {
		return nil, errors.New("winws.exe stopped at start with code 0x00000001.\nLast output: bad")
	}
	return fakeRun{e}, nil
}

func (e *fakeEngine) StopAll() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stopAll++
	return nil
}

type fakeSystem struct {
	elevated, service bool
	tsCalls           int
}

func (s *fakeSystem) Elevated() bool                  { return s.elevated }
func (s *fakeSystem) ServiceInstalled() (bool, error) { return s.service, nil }
func (s *fakeSystem) EnableTimestamps() error         { s.tsCalls++; return nil }

type sink struct {
	mu      sync.Mutex
	lines   []string
	results []Result
	sum     *Summary
}

func (s *sink) Line(t string)     { s.mu.Lock(); s.lines = append(s.lines, t); s.mu.Unlock() }
func (s *sink) Strategy(r Result) { s.results = append(s.results, r) }
func (s *sink) Summary(x Summary) { s.sum = &x }

func (s *sink) has(t *testing.T, line string) {
	t.Helper()
	if !slices.Contains(s.lines, line) {
		t.Errorf("no line %q in:\n%s", line, strings.Join(s.lines, "\n"))
	}
}

// curlFor gives a fake curl: the strategy "good" gives 200 for all, the
// strategy "bad" gives an SSL error for TLS1.2. The current strategy is in
// *cur.
func curlFor(cur *string, mu *sync.Mutex) command.Func {
	return func(_ context.Context, exe string, args ...string) (command.Result, error) {
		if exe != `C:\curl.exe` {
			return command.Result{}, fmt.Errorf("exe %q", exe)
		}
		mu.Lock()
		name := *cur
		mu.Unlock()
		tls12 := slices.Contains(args, "--tlsv1.2")
		dpi := slices.Contains(args, "--range")
		switch {
		case dpi && name == "bad" && tls12:
			return command.Result{Output: "000 65536 0 5.004", Code: 28}, nil
		case dpi:
			return command.Result{Output: "200 65536 1024 0.5"}, nil
		case name == "bad" && tls12:
			return command.Result{Output: "000curl: (60) SSL certificate problem", Code: 60}, nil
		case slices.Contains(args, "--tlsv1.3"):
			return command.Result{Output: "000curl: (35) schannel: failed", Code: 35}, nil
		}
		return command.Result{Output: "200"}, nil
	}
}

func newTester(t *testing.T, names ...string) (*Tester, *fakeEngine, *fakeSystem) {
	t.Helper()
	var mu sync.Mutex
	cur := ""
	e := &fakeEngine{names: names, fail: map[string]bool{}}
	e.onStart = func(n string) { mu.Lock(); cur = n; mu.Unlock() }
	sys := &fakeSystem{elevated: true}
	tt := &Tester{
		Engine:   e,
		System:   sys,
		Curl:     curlFor(&cur, &mu),
		CurlPath: `C:\curl.exe`,
		Ping: func(_ context.Context, host string, _ time.Duration) (time.Duration, error) {
			if host == "9.9.9.9" {
				return 0, errors.New("timeout")
			}
			return 24 * time.Millisecond, nil
		},
		Suite: func(context.Context) ([]DPITarget, error) {
			return []DPITarget{{ID: "US.CF-01", Provider: "Cloudflare", Country: "US", Host: "a.example"}, {ID: "X-02", Provider: "P", Host: "b.example"}}, nil
		},
		Settings:   Settings{StdTimeout: 4, StdParallel: 4, DPITimeout: 5, DPIRange: 1024, DPIParallel: 2},
		ResultsDir: t.TempDir(),
		TempDir:    t.TempDir(),
		Now:        func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) },
	}
	return tt, e, sys
}

func TestStandard(t *testing.T) {
	tt, e, sys := newTester(t, "good", "broken", "bad")
	e.fail["broken"] = true
	s := &sink{}
	if err := tt.Run(context.Background(), Request{Type: Standard, Engine: spec.Winws}, s); err != nil {
		t.Fatal(err)
	}
	for _, l := range []string{
		"[OK] Administrator rights detected",
		"[OK] curl.exe found",
		"Engine: winws",
		"Test type: standard",
		"[INFO] Targets: 17",
		"                 Mode: STANDARD",
		"                 Total configs:  3",
		"  [1/3] good",
		"  [2/3] broken",
		"  [3/3] bad",
		"  > Starting config (winws)...",
		"  > winws.exe stopped at start with code 0x00000001.\nLast output: bad",
		"  DiscordMain              HTTP:OK    TLS1.2:OK    TLS1.3:UNSUP | Ping: 24 ms",
		"  DiscordMain              HTTP:OK    TLS1.2:SSL   TLS1.3:UNSUP | Ping: 24 ms",
		"  Quad9DNS9999             Ping: Timeout",
		"All tests finished.",
		"=== ANALYTICS ===",
		"good : HTTP OK:  24, ERR:   0, UNSUP:  12, Ping OK:  16, Fail:   1",
		"bad  : HTTP OK:  12, ERR:  12, UNSUP:  12, Ping OK:  16, Fail:   1",
		"Best config: good",
	} {
		s.has(t, l)
	}
	if len(s.results) != 3 || s.results[1].Kind != KindFail || s.results[1].Error == "" {
		t.Fatalf("results %+v", s.results)
	}
	g := s.results[0]
	if g.Kind != "standard" || len(g.Targets) != 17 {
		t.Fatalf("good %+v", g)
	}
	if want := (Target{Name: "DiscordMain", HTTP: "OK", TLS12: "OK", TLS13: "UNSUP", Ping: "24 ms"}); g.Targets[0] != want {
		t.Errorf("target 0 %+v", g.Targets[0])
	}
	if want := (Target{Name: "Quad9DNS9999", Ping: "Timeout"}); g.Targets[16] != want {
		t.Errorf("target 16 %+v", g.Targets[16])
	}
	want := Summary{Type: Standard, Best: "good", Rows: []SumRow{
		{Name: "good", OK: 24, Unsup: 12, PingOK: 16, PingFail: 1},
		{Name: "bad", OK: 12, Err: 12, Unsup: 12, PingOK: 16, PingFail: 1},
	}}
	if s.sum == nil || s.sum.Best != want.Best || !slices.Equal(s.sum.Rows, want.Rows) {
		t.Errorf("summary %+v", s.sum)
	}
	if sys.tsCalls != 3 || e.stops != 2 || e.stopAll != 4 {
		t.Errorf("timestamps %d stops %d stopAll %d", sys.tsCalls, e.stops, e.stopAll)
	}
	path := filepath.Join(tt.ResultsDir, "test_results_2026-01-02_03-04-05.txt")
	s.has(t, "Results saved to "+path)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, sub := range []string{
		"\uFEFFConfig: good (Type: standard)\r\n  DiscordMain : HTTP:OK    TLS1.2:OK    TLS1.3:UNSUP | Ping: 24 ms\r\n",
		"Config: broken (Type: fail)\r\n  winws.exe stopped at start with code 0x00000001.\r\n  Last output: bad\r\n\r\n",
		"  Quad9DNS9999 :  | Ping: Timeout\r\n",
		"=== ANALYTICS ===\r\ngood : HTTP OK:  24",
		"Best strategy: good\r\n",
	} {
		if !strings.Contains(text, sub) {
			t.Errorf("no %q in the file:\n%s", sub, text)
		}
	}
}

func TestDPI(t *testing.T) {
	tt, _, _ := newTester(t, "good", "bad")
	s := &sink{}
	if err := tt.Run(context.Background(), Request{Type: DPI, Strategies: []string{"BAD", "good.json"}, Engine: spec.Winws2}, s); err != nil {
		t.Fatal(err)
	}
	for _, l := range []string{
		"  [1/2] good",
		"  [2/2] bad",
		"  > Running DPI checkers...",
		"[INFO] Targets: 2 (custom URL overrides suite). Range: 0-1023 bytes; Timeout: 5s",
		"[INFO] Starting DPI TCP 16-20 checks (parallel: 2)...",
		"=== [US][Cloudflare] US.CF-01 ===",
		"=== [][P] X-02 ===",
		"[HTTP] code=200 buf_up=65536 bytes (64 KB) buf_down=1024 bytes (1 KB) time=0.5s status=OK",
		"[TLS1.2] code=000 buf_up=65536 bytes (64 KB) buf_down=0 bytes (0 KB) time=5.004s status=LIKELY_BLOCKED",
		"  Pattern matches 16-20KB freeze; censor likely cutting this strategy.",
		"  No 16-20KB freeze pattern for this target.",
		"[OK] No 16-20KB freeze pattern detected across targets.",
		"[WARNING] Detected possible DPI TCP 16-20 blocking on one or more targets. Consider changing strategy/SNI/IP.",
		"good : OK:   6, FAIL:   0, UNSUP:   0, BLOCKED:   0",
		"bad  : OK:   4, FAIL:   0, UNSUP:   0, BLOCKED:   2",
		"Best config: good",
	} {
		s.has(t, l)
	}
	if len(s.results) != 2 {
		t.Fatalf("results %+v", s.results)
	}
	if want := (Target{Name: "US.CF-01", HTTP: "OK", TLS12: "LIKELY_BLOCKED", TLS13: "OK"}); s.results[1].Targets[0] != want {
		t.Errorf("target %+v", s.results[1].Targets[0])
	}
	if s.sum.Type != DPI || s.sum.Rows[1] != (SumRow{Name: "bad", OK: 4, Blocked: 2}) {
		t.Errorf("summary %+v", s.sum)
	}
	entries, _ := os.ReadDir(tt.TempDir)
	if len(entries) != 0 {
		t.Errorf("payload left: %v", entries)
	}
	data, err := os.ReadFile(filepath.Join(tt.ResultsDir, "test_results_2026-01-02_03-04-05.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{
		"Config: bad (Type: dpi)\r\n  Target: [US] US.CF-01 (Cloudflare)\r\n    HTTP: code=200  up=64 KB  down=1 KB  time=0.5s  status=OK\r\n",
		"  Target: X-02 (P)\r\n",
		"    TLS1.2: code=000  up=64 KB  down=0 KB  time=5.004s  status=LIKELY_BLOCKED\r\n",
	} {
		if !strings.Contains(string(data), sub) {
			t.Errorf("no %q in the file:\n%s", sub, data)
		}
	}
}

func TestDPICustomAndSuiteFail(t *testing.T) {
	tt, _, _ := newTester(t, "good")
	tt.Suite = func(context.Context) ([]DPITarget, error) { return nil, errors.New("offline") }
	s := &sink{}
	if err := tt.Run(context.Background(), Request{Type: DPI}, s); err != nil {
		t.Fatal(err)
	}
	s.has(t, "[WARN] Fetch dpi suite failed.")
	s.has(t, "No completed strategy results.")
	if s.sum == nil || s.sum.Best != "" || len(s.sum.Rows) != 0 {
		t.Errorf("summary %+v", s.sum)
	}

	tt.Settings.CustomHost = "my.example"
	s = &sink{}
	if err := tt.Run(context.Background(), Request{Type: DPI}, s); err != nil {
		t.Fatal(err)
	}
	s.has(t, "=== [💡][Custom] CUSTOM ===")
}

func TestPreflight(t *testing.T) {
	tt, e, sys := newTester(t, "good")
	sys.elevated, sys.service = false, true
	tt.CurlPath = ""
	s := &sink{}
	if err := tt.Run(context.Background(), Request{Type: Standard}, s); !errors.Is(err, ErrPreflight) {
		t.Fatalf("err %v", err)
	}
	s.has(t, "[ERROR] Run as Administrator to execute tests")
	s.has(t, "[ERROR] curl.exe not found")
	s.has(t, "[ERROR] Windows service 'zapret' is installed")
	s.has(t, "Fix the errors above and rerun.")
	if len(e.started) != 0 {
		t.Error("a strategy started")
	}

	tt, e, _ = newTester(t, "good")
	e.namesErr = errors.New("winws2.exe is missing")
	s = &sink{}
	if err := tt.Run(context.Background(), Request{Type: Standard, Engine: spec.Winws2}, s); err == nil {
		t.Fatal("no error")
	}
	s.has(t, "[ERROR] winws2.exe is missing")

	tt, _, _ = newTester(t, "good")
	s = &sink{}
	if err := tt.Run(context.Background(), Request{Type: Standard, Strategies: []string{"other"}, Engine: spec.Winws}, s); !errors.Is(err, ErrNoStrategies) {
		t.Fatalf("err %v", err)
	}
	s.has(t, "[ERROR] No strategies to test for winws.")
	if err := tt.Run(context.Background(), Request{Type: "x"}, s); !errors.Is(err, ErrUnknownType) {
		t.Fatalf("err %v", err)
	}
}

func TestCancel(t *testing.T) {
	tt, e, _ := newTester(t, "a", "b", "c")
	ctx, cancel := context.WithCancel(context.Background())
	e.onStart = func(n string) {
		if n == "b" {
			cancel()
		}
	}
	s := &sink{}
	err := tt.Run(ctx, Request{Type: Standard}, s)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err %v", err)
	}
	if !slices.Equal(e.started, []string{"a", "b"}) {
		t.Errorf("started %v", e.started)
	}
	s.has(t, "[INFO] Tests cancelled.")
	if s.sum != nil {
		t.Error("summary after a cancel")
	}
	if e.stopAll == 0 {
		t.Error("no StopAll at the end")
	}
}

func TestClassify(t *testing.T) {
	for _, c := range []struct {
		out  string
		code int
		err  error
		want string
	}{
		{"200", 0, nil, "OK"},
		{"301\n", 0, nil, "OK"},
		{"000", 28, nil, "ERROR"},
		{"000curl: (6) Could not resolve host: x", 6, nil, "SSL"},
		{"000curl: (4) A requested feature, protocol or option was not found built-in", 4, nil, "ERROR"},
		{"000", 35, nil, "UNSUP"},
		{"curl: option --tls-max: is unknown, Unknown option", 2, nil, "UNSUP"},
		{"", 0, errors.New("no exe"), "ERROR"},
		{"600", 0, nil, "ERROR"},
	} {
		if got := ClassifyStd(c.out, c.code, c.err); got != c.want {
			t.Errorf("ClassifyStd(%q, %d) = %s, want %s", c.out, c.code, got, c.want)
		}
	}
	for _, c := range []struct {
		out  string
		code int
		want DPILine
	}{
		{"200 100 200 1.5", 0, DPILine{Label: "L", Code: "200", Up: 100, Down: 200, Time: 1.5, Status: "OK"}},
		{"000 100 0 5.01", 28, DPILine{Label: "L", Code: "000", Up: 100, Time: 5.01, Status: "LIKELY_BLOCKED"}},
		{"000 100 0 3.0", 28, DPILine{Label: "L", Code: "000", Up: 100, Time: 3, Status: "FAIL"}},
		{"", 35, DPILine{Label: "L", Code: "UNSUP", Time: -1, Status: "UNSUPPORTED"}},
		{"curl: (35) schannel", 35, DPILine{Label: "L", Code: "UNSUP", Time: -1, Status: "UNSUPPORTED"}},
		{"oops", 7, DPILine{Label: "L", Code: "ERR", Time: -1, Status: "FAIL"}},
		{"", -1, DPILine{Label: "L", Code: "NA", Time: -1, Status: "FAIL"}},
	} {
		if got := ClassifyDPI("L", c.out, c.code, 5); got != c.want {
			t.Errorf("ClassifyDPI(%q, %d) = %+v, want %+v", c.out, c.code, got, c.want)
		}
	}
	if got := kb(16896); got != "16.5" {
		t.Errorf("kb = %s", got)
	}
}

func TestTargets(t *testing.T) {
	ts := StdTargets()
	if len(ts) != 17 {
		t.Fatalf("%d targets", len(ts))
	}
	if ts[0] != (StdTarget{Name: "DiscordMain", URL: "https://discord.com", PingHost: "discord.com"}) {
		t.Errorf("%+v", ts[0])
	}
	if got := ParseTarget("x", "https://a.example/p/q"); got.PingHost != "a.example" {
		t.Errorf("%+v", got)
	}
	if got := ParseTarget("x", "PING: 1.1.1.1"); got != (StdTarget{Name: "x", PingHost: "1.1.1.1"}) {
		t.Errorf("%+v", got)
	}
}

func TestBest(t *testing.T) {
	rows := []row{
		{SumRow: SumRow{Name: "a", OK: 0, PingOK: 0}, kind: "standard"},
		{SumRow: SumRow{Name: "b", OK: 0, PingOK: 3}, kind: "standard"},
	}
	if got := best(rows); got != "b" {
		t.Errorf("best %q", got)
	}
	rows = append(rows, row{SumRow: SumRow{Name: "c", OK: 5, PingOK: 1}, kind: "standard"},
		row{SumRow: SumRow{Name: "d", OK: 5, PingOK: 2}, kind: "standard"},
		row{SumRow: SumRow{Name: "e", OK: 5, PingOK: 2}, kind: "standard"})
	if got := best(rows); got != "d" {
		t.Errorf("best %q", got)
	}
	if got := best([]row{{SumRow: SumRow{Name: "x"}, kind: "dpi"}}); got != "x" {
		t.Errorf("best %q", got)
	}
}

func TestSettingsFromEnv(t *testing.T) {
	env := map[string]string{"TEST_CURL_TIMEOUT": " 7 ", "MONITOR_RANGE": "abc", "MONITOR_MAX_PARALLEL": "0", "MONITOR_HOST": "h.example"}
	s := SettingsFromEnv(func(k string) string { return env[k] })
	d := DefaultSettings()
	if s.StdTimeout != 7 || s.DPIRange != d.DPIRange || s.DPIParallel != d.DPIParallel || s.CustomHost != "h.example" {
		t.Errorf("%+v", s)
	}
	if p := DefaultParallel(); p < 8 || p > 16 {
		t.Errorf("parallel %d", p)
	}
}

func TestFetchSuite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "zapman" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if r.URL.Path == "/bad" {
			_, _ = w.Write([]byte("{"))
			return
		}
		_, _ = w.Write([]byte(`[{"id":"US.CF-01","provider":"Cloudflare","country":"US","host":"a.example","extra":1}]`))
	}))
	defer srv.Close()
	got, err := FetchSuite(context.Background(), srv.Client(), srv.URL+"/s", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != (DPITarget{ID: "US.CF-01", Provider: "Cloudflare", Country: "US", Host: "a.example"}) {
		t.Errorf("%+v", got)
	}
	if _, err := FetchSuite(context.Background(), srv.Client(), srv.URL+"/bad", time.Second); err == nil {
		t.Error("no error for bad JSON")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := FetchSuite(ctx, srv.Client(), srv.URL+"/s", time.Second); !errors.Is(err, context.Canceled) {
		t.Errorf("err %v", err)
	}
}
