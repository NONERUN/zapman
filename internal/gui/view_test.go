package gui

import (
	"strings"
	"testing"
	"time"

	"github.com/NONERUN/zapman/internal/gui/backend"
	"github.com/NONERUN/zapman/internal/gui/i18n"
)

func TestCellKind(t *testing.T) {
	for in, want := range map[string]string{
		"":               cellInfo,
		"-":              cellInfo,
		"n/a":            cellInfo,
		"OK":             cellOK,
		"24 ms":          cellOK,
		"ERROR":          cellErr,
		"ssl error":      cellErr,
		"FAIL":           cellErr,
		"UNSUP":          cellWarn,
		"Timeout":        cellWarn,
		"LIKELY_BLOCKED": cellWarn,
		"what":           cellInfo,
	} {
		if got := cellKind(in); got != want {
			t.Errorf("cellKind(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTrayTooltip(t *testing.T) {
	if s := trayTooltip("abc"); s != "abc" {
		t.Errorf("short = %q", s)
	}
	long := strings.Repeat("я", 200)
	s := []rune(trayTooltip(long))
	if len(s) != 127 || string(s[124:]) != "..." {
		t.Errorf("long has %d runes", len(s))
	}
}

func TestTestStep(t *testing.T) {
	if n, m, ok := testStep("[3/7] general"); !ok || n != 3 || m != 7 {
		t.Errorf("got %d %d %v", n, m, ok)
	}
	for _, s := range []string{"general", "[1/0]", "[a/2]"} {
		if _, _, ok := testStep(s); ok {
			t.Errorf("testStep(%q) is a step", s)
		}
	}
}

func TestJournalLimit(t *testing.T) {
	var j journal
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for i := range 100 {
		j.add(at, strings.Repeat("x", i))
	}
	lines := strings.Split(j.text(), "\n")
	if len(lines) != 80 {
		t.Fatalf("%d lines", len(lines))
	}
	if lines[0] != "03:04:05  "+strings.Repeat("x", 20) {
		t.Errorf("first line = %q", lines[0])
	}
}

func TestStatusLabelsBanner(t *testing.T) {
	en := i18n.EN
	cases := []struct {
		name    string
		st      backend.Status
		strats  bool
		lastErr string
		want    string
	}{
		{"none", backend.Status{DivertSys: true}, true, "", i18n.T(en, "BannerNoService")},
		{"pending", backend.Status{Divert: backend.ServiceStopPending}, true, "x", i18n.T(en, "BannerStopPending")},
		{"last error", backend.Status{DivertSys: true}, true, "x", i18n.T(en, "BannerLastError")},
		{"no sys", backend.Status{}, false, "", i18n.T(en, "BannerNoSys")},
		{"two engines", backend.Status{DivertSys: true, BypassRunning: true, BypassExes: []string{"winws.exe", "winws2.exe"}}, false, "", i18n.T(en, "BannerTwoEngines")},
		{"no strategies", backend.Status{DivertSys: true}, false, "", i18n.T(en, "BannerNoStrategies")},
		{"mismatch", backend.Status{DivertSys: true, Service: backend.ServiceStopped, Installed: "a", BypassName: "winws.exe"}, true, "", i18n.T(en, "BannerMismatch", "winws.exe")}, //nolint:gosec // G101: test data, not a credential.
		{"orphan", backend.Status{DivertSys: true, BypassRunning: true}, true, "", i18n.T(en, "BannerOrphan")},
		{"ok", backend.Status{DivertSys: true, BypassRunning: true, Service: backend.ServiceRunning, Installed: "a"}, true, "", ""},
	}
	for _, c := range cases {
		l := statusLabels(en, c.st, c.strats, c.lastErr)
		if l.Banner.Text != c.want {
			t.Errorf("%s: banner = %q, want %q", c.name, l.Banner.Text, c.want)
		}
	}
	l := statusLabels(en, backend.Status{BypassRunning: true}, true, "")
	if l.Installed != i18n.T(en, "StatusStrategyManual") {
		t.Errorf("manual = %q", l.Installed)
	}
	l = statusLabels(en, backend.Status{Service: backend.ServiceStopped, Installed: "gone"}, false, "")
	if l.Installed != i18n.T(en, "StatusStrategyGone", "gone") {
		t.Errorf("gone = %q", l.Installed)
	}
	// A missing WinDivert64.sys is red when the bypass cannot start and
	// yellow when the bypass runs, as the icon.
	if b := statusLabels(en, backend.Status{}, true, "").Banner; b.Fg != bannerRedFg {
		t.Errorf("no sys, off: %+v", b)
	}
	if b := statusLabels(en, backend.Status{Service: backend.ServiceRunning}, true, "").Banner; b.Text != i18n.T(en, "BannerNoSys") || b.Fg != bannerYellowFg {
		t.Errorf("no sys, live: %+v", b)
	}
}

func TestFakeSlot(t *testing.T) {
	files := []string{"a.bin", "b.bin"}
	s := newFakeSlot(i18n.EN, files, "b.bin", backend.FakeMatched)
	if len(s.Items) != 2 || s.Selected != 1 {
		t.Errorf("matched = %+v", s)
	}
	if s.picked(1) != "" || s.picked(0) != "a.bin" || s.picked(5) != "" {
		t.Error("matched picks are wrong")
	}
	s = newFakeSlot(i18n.EN, files, "c.bin", backend.FakeUnlisted)
	if len(s.Items) != 3 || s.Selected != 0 || s.Items[0] != i18n.T(i18n.EN, "FakesUnlisted") {
		t.Errorf("unlisted = %+v", s)
	}
	if s.picked(0) != "" || s.picked(2) != "b.bin" {
		t.Error("unlisted picks are wrong")
	}
}

func TestSummaryTable(t *testing.T) {
	if tb := summaryTable(i18n.EN, backend.TestSummary{Type: backend.TestStandard}); len(tb.Headers) != 0 {
		t.Errorf("empty = %+v", tb)
	}
	tb := summaryTable(i18n.EN, backend.TestSummary{Type: backend.TestDPI, Rows: []backend.TestSumRow{{Name: "a", OK: 3, Fail: 1}}})
	if len(tb.Headers) != 5 || len(tb.Rows) != 1 {
		t.Fatalf("dpi = %+v", tb)
	}
	r := tb.Rows[0]
	if r[1].Kind != cellOK || r[2].Kind != cellErr || r[3].Kind != cellOK || r[2].Text != "1" {
		t.Errorf("row = %+v", r)
	}
}

func TestStrategyTab(t *testing.T) {
	tab := strategyTab(i18n.EN, backend.TestResult{Name: "a", Kind: "standard", Targets: []backend.TestTarget{{Name: "x", HTTP: "OK", TLS13: "ERROR", Ping: "1 ms"}}})
	if len(tab.Table.Headers) != 5 || !tab.HasError {
		t.Fatalf("tab = %+v", tab)
	}
	if c := tab.Table.Rows[0][2]; c.Text != "-" || c.Kind != cellInfo {
		t.Errorf("empty cell = %+v", c)
	}
	tab = strategyTab(i18n.EN, backend.TestResult{Kind: "fail", Error: "boom"})
	if tab.Name != "?" || !tab.HasError || tab.Table.Error != "boom" {
		t.Errorf("fail tab = %+v", tab)
	}
}

func TestIconOf(t *testing.T) {
	ok := backend.Status{DivertSys: true}
	with := func(fn func(s *backend.Status)) backend.Status {
		s := ok
		fn(&s)
		return s
	}
	svcRuns := with(func(s *backend.Status) { s.Service = backend.ServiceRunning })
	winws := with(func(s *backend.Status) { s.BypassRunning, s.BypassExes = true, []string{"winws.exe"} })
	for _, c := range []struct {
		name    string
		st      backend.Status
		strats  bool
		lastErr string
		want    Icon
		alert   string
	}{
		{"nothing", ok, true, "", IconIdle, ""},
		{"service stopped", with(func(s *backend.Status) { s.Service, s.Installed = backend.ServiceStopped, "a" }), true, "", IconIdle, ""},
		{"service starts", with(func(s *backend.Status) { s.Service = backend.ServiceStartPending }), true, "", IconIdle, ""},
		{"service runs", svcRuns, true, "", IconLive, ""},
		{"winws runs", winws, true, "", IconLive, ""},
		{"winws runs, service stopped", with(func(s *backend.Status) { s.BypassRunning, s.Service = true, backend.ServiceStopped }), true, "", IconLive, ""},

		{"divert stuck", with(func(s *backend.Status) { s.Divert = backend.ServiceStopPending }), true, "", IconError, "AlertStopPending"},
		{"divert stuck, live", with(func(s *backend.Status) { s.Divert, s.Service = backend.ServiceStopPending, backend.ServiceRunning }), true, "", IconError, "AlertStopPending"},
		{"last error, off", ok, true, "x", IconError, "AlertLastError"},
		{"no sys, off", backend.Status{}, true, "", IconError, "AlertNoSys"},

		{"last error, live", svcRuns, true, "x", IconWarn, "AlertLastError"},
		{"no sys, live", backend.Status{Service: backend.ServiceRunning}, true, "", IconWarn, "AlertNoSys"},
		{"two engines", with(func(s *backend.Status) { s.BypassRunning, s.BypassExes = true, []string{"winws.exe", "winws2.exe"} }), true, "", IconWarn, "AlertTwoEngines"},
		{"no strategies, live", svcRuns, false, "", IconWarn, "AlertNoStrategies"},
		{"no strategies, off", ok, false, "", IconWarn, "AlertNoStrategies"},
	} {
		got, alert := iconOf(c.st, c.strats, c.lastErr)
		if got != c.want || alert != c.alert {
			t.Errorf("%s: got %v %q, want %v %q", c.name, got, alert, c.want, c.alert)
		}
		if alert != "" && i18n.T(i18n.EN, alert) == alert {
			t.Errorf("%s: no text for %q", c.name, alert)
		}
	}
}

func TestTrayView(t *testing.T) {
	v := trayView(i18n.EN, backend.Status{Service: backend.ServiceStopped, Installed: "a"}, "")
	if !v.StartEnabled || v.StopEnabled || v.Lines[2] != i18n.T(i18n.EN, "StatusStrategy", "a") || v.Alert != "" {
		t.Errorf("stopped = %+v", v)
	}
	if strings.Count(v.Tooltip, "\n") != 2 {
		t.Errorf("tooltip without alert = %q", v.Tooltip)
	}
	v = trayView(i18n.EN, backend.Status{BypassRunning: true}, "")
	if v.StartEnabled || !v.StopEnabled {
		t.Errorf("running = %+v", v)
	}
	if v.Exit != "Exit" || trayView(i18n.RU, backend.Status{}, "").Exit != "Выход" {
		t.Errorf("exit = %q", v.Exit)
	}
	v = trayView(i18n.RU, backend.Status{}, "AlertNoSys")
	if v.Alert != i18n.T(i18n.RU, "AlertNoSys") || !strings.HasPrefix(v.Tooltip, v.Alert+"\n") {
		t.Errorf("alert = %q, tooltip = %q", v.Alert, v.Tooltip)
	}
}

// TestTrayTooltipAlerts checks that each alert is the first line of the
// tooltip and that the tooltip is not longer than the tray permits.
func TestTrayTooltipAlerts(t *testing.T) {
	st := backend.Status{BypassRunning: true, BypassName: "winws2.exe", Service: backend.ServiceStopPending, Running: "general (ALT11 Photon)"} //nolint:gosec // G101: test data, not a credential.
	for _, lang := range []string{i18n.EN, i18n.RU} {
		for _, key := range []string{"AlertStopPending", "AlertLastError", "AlertNoSys", "AlertTwoEngines", "AlertNoStrategies"} {
			v := trayView(lang, st, key)
			if !strings.HasPrefix(v.Tooltip, v.Alert+"\n") {
				t.Errorf("%s %s: tooltip %q", lang, key, v.Tooltip)
			}
			if n := len([]rune(v.Tooltip)); n > 127 {
				t.Errorf("%s %s: tooltip has %d runes", lang, key, n)
			}
		}
	}
}
