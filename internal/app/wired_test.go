package app

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/NONERUN/zapman/internal/config"
	"github.com/NONERUN/zapman/internal/engine"
	"github.com/NONERUN/zapman/internal/gui/backend"
	"github.com/NONERUN/zapman/internal/spec"
	"github.com/NONERUN/zapman/internal/tools/stratest"
	"github.com/NONERUN/zapman/internal/tools/web"
)

// seen is the state of the computer while the fake tests run.
type seen struct {
	req     stratest.Request
	service bool
	run     string
	ipset   engine.IPSetMode
}

// fakeTester is a testRunner that keeps what it sees.
type fakeTester struct {
	e    *env
	got  *seen
	err  error
	feed func(stratest.Sink)
}

func (f *fakeTester) Run(_ context.Context, req stratest.Request, sink stratest.Sink) error {
	st, _ := f.e.b.Status(ctx)
	mode, _ := f.e.l.IPSetMode()
	f.got = &seen{req: req, service: st.HasService(), run: st.Running, ipset: mode}
	if f.feed != nil {
		f.feed(sink)
	}
	return f.err
}

// wiredEnv gives the env with the Tools of internal/tools and the fake
// tests.
func wiredEnv(t *testing.T) (*env, *wired, *fakeTester) {
	t.Helper()
	e := newEnv(t)
	w, ok := NewTools(e.b).(*wired)
	if !ok {
		t.Fatal("NewTools does not give *wired")
	}
	ft := &fakeTester{e: e}
	w.tester = func(engine.Layout, *spec.Catalog, config.Config) testRunner { return ft }
	e.b.SetTools(w)
	return e, w, ft
}

// sinkRec is a backend.TestSink that keeps all output.
type sinkRec struct {
	lines   []string
	results []backend.TestResult
	sums    []backend.TestSummary
}

func (s *sinkRec) Line(text string)                { s.lines = append(s.lines, text) }
func (s *sinkRec) Strategy(r backend.TestResult)   { s.results = append(s.results, r) }
func (s *sinkRec) Summary(sum backend.TestSummary) { s.sums = append(s.sums, sum) }

func TestRunTestsRestoreRun(t *testing.T) {
	e, _, ft := wiredEnv(t)
	if err := e.b.Run(ctx, testStrategy, spec.Winws2); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(e.l.IPSetPath())
	if err != nil {
		t.Fatal(err)
	}
	ft.err = context.Canceled
	req := backend.TestRequest{Type: backend.TestStandard, Strategies: []string{testStrategy}, Engine: spec.Winws}
	if err := e.b.RunTests(ctx, req, &sinkRec{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want the error of the tests", err)
	}
	if ft.got == nil || ft.got.run != "" || ft.got.service {
		t.Fatalf("the engine ran during the tests: %+v", ft.got)
	}
	if ft.got.req.Type != stratest.Standard || ft.got.req.Engine != spec.Winws || !slices.Equal(ft.got.req.Strategies, req.Strategies) {
		t.Errorf("request: %+v", ft.got.req)
	}
	after, _ := os.ReadFile(e.l.IPSetPath())
	if string(after) != string(before) {
		t.Error("the standard tests changed the ipset")
	}
	st, _ := e.b.Status(ctx)
	if st.Running != testStrategy || st.BypassName != "winws2.exe" {
		t.Fatalf("the run did not start again: %+v", st)
	}
	if len(e.cmdLines) != 2 || e.cmdLines[0] != e.cmdLines[1] {
		t.Errorf("command lines: %q", e.cmdLines)
	}
}

func TestRunTestsDPIService(t *testing.T) {
	e, _, ft := wiredEnv(t)
	if err := e.b.Install(ctx, testStrategy, spec.Winws); err != nil {
		t.Fatal(err)
	}
	list := e.l.IPSetPath()
	if err := os.WriteFile(list, []byte("10.0.0.0/8\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	req := backend.TestRequest{Type: backend.TestDPI, Strategies: []string{"all"}, Engine: spec.Winws}
	if err := e.b.RunTests(ctx, req, &sinkRec{}); err != nil {
		t.Fatal(err)
	}
	if ft.got == nil || ft.got.service || ft.got.ipset != engine.IPSetAny || ft.got.req.Type != stratest.DPI {
		t.Fatalf("during the tests: %+v", ft.got)
	}
	if b, _ := os.ReadFile(list); string(b) != "10.0.0.0/8\n" {
		t.Errorf("ipset after the tests: %q", b)
	}
	if _, err := os.Stat(filepath.Join(e.l.User, TestIPSetBackup)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the test backup is still there: %v", err)
	}
	st, _ := e.b.Status(ctx)
	if st.Service != backend.ServiceRunning || st.Installed != testStrategy {
		t.Errorf("the service did not come back: %+v", st)
	}
}

func TestRunTestsLeftover(t *testing.T) {
	e, _, ft := wiredEnv(t)
	if err := os.MkdirAll(e.l.User, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.l.IPSetPath(), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.l.User, TestIPSetBackup), []byte("1.2.3.4/32\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	req := backend.TestRequest{Type: backend.TestStandard, Engine: spec.Winws}
	if err := e.b.RunTests(ctx, req, &sinkRec{}); err != nil {
		t.Fatal(err)
	}
	if ft.got.ipset != engine.IPSetLoaded {
		t.Errorf("the earlier ipset did not come back before the tests: %v", ft.got.ipset)
	}
}

func TestRunTestsErrors(t *testing.T) {
	e, _, ft := wiredEnv(t)
	e.h.elevated = false
	req := backend.TestRequest{Type: backend.TestStandard, Engine: spec.Winws}
	if err := e.b.RunTests(ctx, req, &sinkRec{}); msgKey(err) != "AdminRequired" {
		t.Errorf("not elevated: %v", err)
	}
	e.h.elevated = true
	e.b.l.Bin = t.TempDir()
	if err := e.b.RunTests(ctx, req, &sinkRec{}); err == nil {
		t.Error("no engine files: no error")
	}
	if ft.got != nil {
		t.Error("the tests ran")
	}
}

func TestRunTestsSink(t *testing.T) {
	e, _, ft := wiredEnv(t)
	ft.feed = func(s stratest.Sink) {
		s.Line("head")
		s.Strategy(stratest.Result{Name: "a", Kind: stratest.KindFail, Error: "bad"})
		s.Strategy(stratest.Result{Name: "b", Kind: string(stratest.Standard), Targets: []stratest.Target{
			{Name: "x", HTTP: "OK", TLS12: "ERROR", TLS13: "UNSUP", Ping: "12ms"},
		}})
		s.Summary(stratest.Summary{Type: stratest.Standard, Best: "b", Rows: []stratest.SumRow{
			{Name: "b", OK: 1, Err: 2, Unsup: 3, PingOK: 4, PingFail: 5, Fail: 6, Blocked: 7},
		}})
	}
	rec := &sinkRec{}
	if err := e.b.RunTests(ctx, backend.TestRequest{Type: backend.TestStandard, Engine: spec.Winws}, rec); err != nil {
		t.Fatal(err)
	}
	want := []backend.TestResult{
		{Name: "a", Kind: "fail", Error: "bad"},
		{Name: "b", Kind: "standard", Targets: []backend.TestTarget{{Name: "x", HTTP: "OK", TLS12: "ERROR", TLS13: "UNSUP", Ping: "12ms"}}},
	}
	if !slices.Equal(rec.lines, []string{"head"}) || len(rec.results) != 2 || len(rec.sums) != 1 {
		t.Fatalf("sink: %+v", rec)
	}
	for i := range want {
		if rec.results[i].Name != want[i].Name || rec.results[i].Kind != want[i].Kind ||
			rec.results[i].Error != want[i].Error || !slices.Equal(rec.results[i].Targets, want[i].Targets) {
			t.Errorf("result %d: %+v", i, rec.results[i])
		}
	}
	sum := rec.sums[0]
	if sum.Type != backend.TestStandard || sum.Best != "b" ||
		!slices.Equal(sum.Rows, []backend.TestSumRow{{Name: "b", OK: 1, Err: 2, Unsup: 3, PingOK: 4, PingFail: 5, Fail: 6, Blocked: 7}}) {
		t.Errorf("summary: %+v", sum)
	}
}

// allEngine is a stratest.Engine that gives all names of the catalog.
type allEngine struct {
	stratest.Engine
	cat *spec.Catalog
}

func (a allEngine) Names(spec.Engine) ([]string, error) { return a.cat.StrategyNames() }

func TestSupportedEngine(t *testing.T) {
	e := newEnv(t)
	// A copy of the catalog where one strategy has a luaPayload that only
	// winws2 rejects.
	dir := filepath.Join(t.TempDir(), "strategies")
	if err := os.CopyFS(dir, os.DirFS(e.l.Strategies)); err != nil {
		t.Fatal(err)
	}
	alt := filepath.Join(dir, "flowseal", "general (ALT).json")
	data, err := os.ReadFile(alt)
	if err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(string(data), `"repeats": 6,`, `"repeats": 6, "luaPayload": "all, quic",`, 1)
	if err := os.WriteFile(alt, []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	e.l.Strategies = dir
	cat, err := e.l.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	infos, _, err := backend.LoadCatalogInfo(e.l.Strategies)
	if err != nil {
		t.Fatal(err)
	}
	all, _ := cat.StrategyNames()
	for _, eng := range []spec.Engine{spec.Winws, spec.Winws2} {
		got, err := supportedEngine{Engine: allEngine{cat: cat}, cat: cat}.Names(eng)
		if err != nil {
			t.Fatal(err)
		}
		var want []string
		for _, in := range infos {
			if in.Supports(eng) {
				want = append(want, in.Name)
			}
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s: got %q, want %q", eng, got, want)
		}
		if has := slices.Contains(got, "general (ALT)"); has != (eng == spec.Winws) {
			t.Errorf("%s: general (ALT) in the list is %v; %d of %d strategies", eng, has, len(got), len(all))
		}
	}
}

// bodyRT answers each request with the status 200 and the body.
type bodyRT string

func (b bodyRT) RoundTrip(r *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK, Status: "200 OK", Request: r,
		Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(b))),
	}, nil
}

func TestLatestVersionNoTag(t *testing.T) {
	e, w, _ := wiredEnv(t)
	w.web = &web.Client{HTTP: &http.Client{Transport: bodyRT("{}")}}
	if v, err := e.b.LatestVersion(ctx); v != "" || err != nil {
		t.Errorf("no tag: %q, %v", v, err)
	}
	w.web = &web.Client{HTTP: &http.Client{Transport: bodyRT(`{"tag_name":"v1.2.3"}`)}}
	if v, err := e.b.LatestVersion(ctx); v != "v1.2.3" || err != nil {
		t.Errorf("tag: %q, %v", v, err)
	}
}
