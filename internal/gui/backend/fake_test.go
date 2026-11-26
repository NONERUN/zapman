package backend

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/NONERUN/zapman/internal/spec"
)

var ctx = context.Background()

func TestFakeInstallRunStop(t *testing.T) {
	f := NewFake()
	st, err := f.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.HasService() || st.BypassRunning {
		t.Fatalf("start state: %+v", st)
	}
	if err := f.StartService(ctx); err == nil {
		t.Fatal("StartService with no service: no error")
	}

	if err := f.Install(ctx, "general", spec.Winws2); err != nil {
		t.Fatal(err)
	}
	st, _ = f.Status(ctx)
	if st.Service != ServiceRunning || st.Installed != "general" || st.Running != "general" || st.BypassName != "winws2.exe" {
		t.Fatalf("after Install: %+v", st)
	}
	if s, _ := f.LoadSettings(ctx); s.Engine != spec.Winws2 || s.Strategy != "general" {
		t.Fatalf("settings after Install: %+v", s)
	}

	if err := f.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	st, _ = f.Status(ctx)
	if st.Service != ServiceStopped || st.BypassRunning || st.Running != "" {
		t.Fatalf("after Stop: %+v", st)
	}
	if err := f.StartService(ctx); err != nil {
		t.Fatal(err)
	}
	if st, _ = f.Status(ctx); !st.Live() || st.Running != "general" {
		t.Fatalf("after StartService: %+v", st)
	}

	if err := f.Run(ctx, "general (ALT)", spec.Winws); err != nil {
		t.Fatal(err)
	}
	st, _ = f.Status(ctx)
	if st.Service != ServiceStopped || st.Running != "general (ALT)" || st.Installed != "general" {
		t.Fatalf("after Run: %+v", st)
	}

	if err := f.RemoveServices(ctx); err != nil {
		t.Fatal(err)
	}
	if st, _ = f.Status(ctx); st.HasService() || st.BypassRunning || st.Divert != ServiceNone {
		t.Fatalf("after RemoveServices: %+v", st)
	}
	want := []string{"StartService", "Install", "Stop", "StartService", "Run", "RemoveServices"}
	if got := f.Calls(); !slices.Equal(got, want) {
		t.Fatalf("calls: %v, want %v", got, want)
	}
}

func TestFakeEngineErrors(t *testing.T) {
	f := NewFake()
	var me *MsgError
	if err := f.Run(ctx, "general (ALT2)", spec.Winws2); !errors.As(err, &me) || me.Key != "EngineNoFlags" {
		t.Fatalf("Run with no winws2 flags: %v", err)
	}
	f.Update(func(d *FakeData) { d.EngineMissing = map[spec.Engine]bool{spec.Winws2: true} })
	if err := f.CheckEngine(ctx, spec.Winws2); !errors.As(err, &me) || me.Key != "EngineNoWinws2" {
		t.Fatalf("CheckEngine: %v", err)
	}
	if err := f.Install(ctx, "general", spec.Winws2); err == nil {
		t.Fatal("Install with no engine files: no error")
	}
	if err := f.Install(ctx, "nothing", spec.Winws); err == nil {
		t.Fatal("Install of an unknown strategy: no error")
	}
	if err := f.CheckEngine(ctx, spec.Winws); err != nil {
		t.Fatal(err)
	}
}

func TestFakeFailNext(t *testing.T) {
	f := NewFake()
	boom := errors.New("boom")
	f.FailNext("Stop", boom)
	if err := f.Stop(ctx); !errors.Is(err, boom) {
		t.Fatalf("Stop: %v", err)
	}
	if err := f.Stop(ctx); err != nil {
		t.Fatalf("second Stop: %v", err)
	}
}

func TestFakeSuspendRestore(t *testing.T) {
	f := NewFake()
	snap, err := f.SuspendService(ctx)
	if err != nil || snap.Strategy != "" {
		t.Fatalf("Suspend with no service: %+v %v", snap, err)
	}
	if err := f.RestoreService(ctx, snap); err != nil {
		t.Fatal(err)
	}
	if err := f.Install(ctx, "general", spec.Winws); err != nil {
		t.Fatal(err)
	}
	if snap, err = f.SuspendService(ctx); err != nil {
		t.Fatal(err)
	}
	if st, _ := f.Status(ctx); st.HasService() {
		t.Fatalf("service after Suspend: %+v", st)
	}
	if err := f.RestoreService(ctx, snap); err != nil {
		t.Fatal(err)
	}
	if st, _ := f.Status(ctx); st.Service != ServiceRunning || st.Installed != "general" {
		t.Fatalf("after Restore: %+v", st)
	}
}

func TestFakeSettings(t *testing.T) {
	f := NewFake()
	s, err := f.LoadSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	s.Games = []string{"sky", "sker", "sky"}
	s.GameFilter = GameFilterUDP
	s.Language = "en"
	if err := f.SaveSettings(ctx, s); err != nil {
		t.Fatal(err)
	}
	got, _ := f.LoadSettings(ctx)
	if !slices.Equal(got.Games, []string{"sker", "sky"}) || got.GameFilter != GameFilterUDP || got.Language != "en" {
		t.Fatalf("saved: %+v", got)
	}
	s.Games = []string{"nope"}
	if err := f.SaveSettings(ctx, s); err == nil {
		t.Fatal("unknown game: no error")
	}
	s.Games, s.GameFilter = nil, "bad"
	if err := f.SaveSettings(ctx, s); err == nil {
		t.Fatal("bad Game Filter mode: no error")
	}

	f.Update(func(d *FakeData) { d.ConfigErr = errors.New("bad JSON") })
	_, err = f.LoadSettings(ctx)
	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("LoadSettings with a bad file: %v", err)
	}
}

func TestFakeIPSet(t *testing.T) {
	f := NewFake()
	if m, err := f.IPSet(ctx); err != nil || m != IPSetLoaded {
		t.Fatalf("IPSet = %q, %v", m, err)
	}
	for _, m := range []IPSetMode{IPSetAny, IPSetNone, IPSetLoaded} {
		if err := f.SetIPSet(ctx, m); err != nil {
			t.Fatalf("SetIPSet(%q): %v", m, err)
		}
	}
	if err := f.SetIPSet(ctx, IPSetNone); err != nil {
		t.Fatal(err)
	}
	f.Update(func(d *FakeData) { d.IPSetBackup = false })
	var m *MsgError
	if err := f.SetIPSet(ctx, IPSetLoaded); !errors.As(err, &m) || m.Key != "IpsetNoBackup" {
		t.Fatalf("loaded with no backup: %v", err)
	}
	if err := f.SetIPSet(ctx, "bad"); err == nil {
		t.Fatal("bad mode: no error")
	}
}

func TestFakeForeignEngines(t *testing.T) {
	f := NewFake()
	f.Update(func(d *FakeData) { d.Foreign = []string{"winws.exe (PID 7)"} })
	if got, _ := f.ForeignEngines(ctx); len(got) != 1 {
		t.Fatalf("foreign = %v", got)
	}
	if err := f.Run(ctx, "general", "winws"); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.ForeignEngines(ctx); len(got) != 0 {
		t.Fatalf("foreign after Run = %v", got)
	}
}

func TestFakeFakes(t *testing.T) {
	f := NewFake()
	if err := f.SetFake(ctx, FakeGame, "stun.bin"); err != nil {
		t.Fatal(err)
	}
	c, _ := f.Fakes(ctx)
	if c.Game != "stun.bin" || c.GameState != FakeMatched {
		t.Fatalf("catalog: %+v", c)
	}
	if err := f.SetFake(ctx, FakeGame, "nope.bin"); err == nil {
		t.Fatal("unknown file: no error")
	}
	if err := f.SetFake(ctx, "x", "stun.bin"); err == nil {
		t.Fatal("unknown slot: no error")
	}
}

func TestFakeDownload(t *testing.T) {
	f := NewFake()
	var steps [][2]int64
	if err := f.DownloadIPSet(ctx, func(done, total int64) { steps = append(steps, [2]int64{done, total}) }); err != nil {
		t.Fatal(err)
	}
	if len(steps) != 5 || steps[0][1] != -1 || steps[4][0] != steps[4][1] {
		t.Fatalf("steps: %v", steps)
	}
	if f.Data().IPSetDownloads != 1 {
		t.Fatal("no download count")
	}

	f.Delay = time.Hour
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := f.CheckHosts(c, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled CheckHosts: %v", err)
	}
}

func TestFakeDiag(t *testing.T) {
	f := NewFake()
	items, _ := f.Diagnostics(ctx)
	i := slices.IndexFunc(items, func(d DiagItem) bool { return d.Action != "" })
	if err := f.RunDiagAction(ctx, items[i]); err != nil {
		t.Fatal(err)
	}
	items, _ = f.Diagnostics(ctx)
	if items[i].Status != DiagOK || items[i].Action != "" {
		t.Fatalf("item after the action: %+v", items[i])
	}
	if err := f.RunDiagAction(ctx, DiagItem{Action: "format-c"}); err == nil {
		t.Fatal("unknown action: no error")
	}
}

func TestFakeDiagTCPTimestamps(t *testing.T) {
	f := NewFake()
	tcp := func() DiagItem {
		items, err := f.Diagnostics(ctx)
		if err != nil {
			t.Fatal(err)
		}
		i := slices.IndexFunc(items, func(d DiagItem) bool { return d.ID == "tcp" })
		if i < 0 {
			t.Fatal("no tcp item")
		}
		return items[i]
	}
	if it := tcp(); it.Status != DiagFail {
		t.Fatalf("before the start: %+v", it)
	}
	if err := f.Run(ctx, "general", spec.Winws); err != nil {
		t.Fatal(err)
	}
	if it := tcp(); it.Status != DiagOK || it.Text != "TCP timestamps are enabled" {
		t.Fatalf("after Run: %+v", it)
	}
}

type sink struct {
	lines   []string
	results []TestResult
	sum     *TestSummary
}

func (s *sink) Line(text string)      { s.lines = append(s.lines, text) }
func (s *sink) Strategy(r TestResult) { s.results = append(s.results, r) }
func (s *sink) Summary(x TestSummary) { s.sum = &x }

func TestFakeTests(t *testing.T) {
	f := NewFake()
	var s sink
	req := TestRequest{Type: TestStandard, Strategies: []string{"a", "b", "c"}, Engine: spec.Winws}
	if err := f.RunTests(ctx, req, &s); err != nil {
		t.Fatal(err)
	}
	if len(s.results) != 3 || s.results[1].Kind != "fail" || s.sum == nil || len(s.sum.Rows) != 2 {
		t.Fatalf("results: %+v, summary: %+v", s.results, s.sum)
	}
	if s.sum.Best != "a" || s.sum.Rows[0].PingFail != 1 || s.sum.Rows[0].Unsup != 1 {
		t.Fatalf("summary: %+v", s.sum)
	}
	if !slices.Contains(s.lines, "[2/3] b") {
		t.Fatalf("lines: %v", s.lines)
	}

	var d sink
	req.Type = TestDPI
	if err := f.RunTests(ctx, req, &d); err != nil {
		t.Fatal(err)
	}
	if d.sum.Rows[0].Blocked != 1 || d.sum.Rows[1].Fail != 1 || d.results[0].Targets[0].Ping != "" {
		t.Fatalf("DPI summary: %+v", d.sum)
	}

	if err := f.RunTests(ctx, TestRequest{Type: TestStandard}, &sink{}); err == nil {
		t.Fatal("no strategies: no error")
	}
	f.Delay = time.Hour
	c, cancel := context.WithCancel(ctx)
	cancel()
	if err := f.RunTests(c, req, &sink{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled RunTests: %v", err)
	}
}

func TestLoadCatalogInfo(t *testing.T) {
	list, games, err := LoadCatalogInfo(filepath.Join("..", "..", "..", "strategies"))
	if err != nil {
		t.Fatal(err)
	}
	if len(list) == 0 || len(games) == 0 {
		t.Fatalf("strategies: %d, games: %d", len(list), len(games))
	}
	for _, s := range list {
		if len(s.Engines) == 0 {
			t.Errorf("strategy %q: no engine", s.Name)
		}
	}
	for _, g := range games {
		if g.ID == "" || g.Title == "" {
			t.Errorf("game: %+v", g)
		}
	}
}
