package gui

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/NONERUN/zapman/internal/gui/backend"
	"github.com/NONERUN/zapman/internal/gui/i18n"
)

// The banner colors of the PS GUI, as CSS colors.
const (
	bannerBlueBg   = "rgb(230,240,250)"
	bannerBlueFg   = "rgb(24,95,165)"
	bannerRedBg    = "rgb(253,232,230)"
	bannerRedFg    = "rgb(180,35,24)"
	bannerYellowBg = "rgb(255,244,214)"
	bannerYellowFg = "rgb(154,103,0)"
)

// Banner is the colored line under the title. An empty Text hides it.
type Banner struct {
	Text string
	Bg   string
	Fg   string
}

// StatusLabels are the four lines of the Status group and the banner.
type StatusLabels struct {
	Bypass    string
	Service   string
	Installed string
	Divert    string
	Banner    Banner
}

// statusLabels gives the Status group lines and the banner, as
// Update-Status of the PS GUI does.
func statusLabels(lang string, st backend.Status, hasStrategies bool, lastError string) StatusLabels {
	t := func(key string, args ...any) string { return i18n.T(lang, key, args...) }
	var l StatusLabels
	if st.BypassRunning {
		l.Bypass = t("StatusBypassOn", st.BypassName)
	} else {
		l.Bypass = t("StatusBypassOff")
	}
	svc := st.HasService()
	if svc {
		l.Service = t("StatusServiceOn", string(st.Service))
	} else {
		l.Service = t("StatusServiceOff")
	}
	switch {
	case st.BypassRunning && st.Running != "":
		if st.Service == backend.ServiceRunning {
			l.Installed = t("StatusStrategy", st.Running)
		} else {
			l.Installed = t("StatusStrategyRun", st.Running)
		}
	case st.Installed == "":
		if st.BypassRunning && !svc {
			l.Installed = t("StatusStrategyManual")
		} else {
			l.Installed = t("StatusStrategyNone")
		}
	case !hasStrategies:
		l.Installed = t("StatusStrategyGone", st.Installed)
	default:
		l.Installed = t("StatusStrategy", st.Installed)
	}

	pending := false
	switch st.Divert {
	case backend.ServiceRunning:
		l.Divert = t("StatusDivertOn")
	case backend.ServiceStopPending:
		l.Divert = t("StatusDivertPending")
		pending = true
	case backend.ServiceNone:
		l.Divert = t("StatusDivertNone")
	default:
		l.Divert = t("StatusDivertOther", string(st.Divert))
	}

	b := Banner{Bg: bannerBlueBg, Fg: bannerBlueFg}
	switch {
	case pending:
		b = Banner{Text: t("BannerStopPending"), Bg: bannerRedBg, Fg: bannerRedFg}
	case lastError != "":
		b = Banner{Text: t("BannerLastError"), Bg: bannerRedBg, Fg: bannerRedFg}
	case !hasStrategies:
		b = Banner{Text: t("BannerNoStrategies"), Bg: bannerYellowBg, Fg: bannerYellowFg}
	case svc && !st.BypassRunning:
		b = Banner{Text: t("BannerMismatch", st.BypassName), Bg: bannerYellowBg, Fg: bannerYellowFg}
	case st.BypassRunning && !svc:
		b.Text = t("BannerOrphan")
	case !svc:
		b.Text = t("BannerNoService")
	default:
		b = Banner{}
	}
	l.Banner = b
	return l
}

// Icon is the icon of the windows and the tray.
type Icon int

const (
	// IconIdle is the gray icon: the bypass does not work.
	IconIdle Icon = iota
	// IconLive is the blue icon: the zapret service or a winws runs.
	IconLive
)

// iconOf gives the icon for the status.
func iconOf(st backend.Status) Icon {
	if st.Live() {
		return IconLive
	}
	return IconIdle
}

// TrayView is the state of the tray icon and its menu.
type TrayView struct {
	// Lines are the three status items of the menu.
	Lines   [3]string
	Tooltip string
	Open    string
	Start   string
	Stop    string
	// Exit is the item that ends the program. A winws that runs in the
	// foreground continues to run.
	Exit         string
	StartEnabled bool
	StopEnabled  bool
}

// trayView gives the tray state, as Tray.ps1 does.
func trayView(lang string, st backend.Status) TrayView {
	t := func(key string, args ...any) string { return i18n.T(lang, key, args...) }
	var v TrayView
	if st.BypassRunning {
		v.Lines[0] = t("StatusBypassOn", st.BypassName)
	} else {
		v.Lines[0] = t("StatusBypassOff")
	}
	if st.HasService() {
		v.Lines[1] = t("StatusServiceOn", string(st.Service))
	} else {
		v.Lines[1] = t("StatusServiceOff")
	}
	name := st.Running
	if name == "" {
		name = st.Installed
	}
	if name != "" {
		v.Lines[2] = t("StatusStrategy", name)
	} else {
		v.Lines[2] = t("StatusStrategyNone")
	}
	v.Tooltip = trayTooltip(strings.Join(v.Lines[:], "\n"))
	v.Open, v.Start, v.Stop, v.Exit = t("TrayOpen"), t("TrayStart"), t("TrayStop"), t("TrayExit")
	v.StartEnabled = st.Service == backend.ServiceStopped
	v.StopEnabled = st.BypassRunning || (st.HasService() && st.Service != backend.ServiceStopped)
	return v
}

// trayTooltip cuts the text to the 127 characters of a NotifyIcon tooltip.
func trayTooltip(s string) string {
	r := []rune(s)
	if len(r) <= 127 {
		return s
	}
	return string(r[:124]) + "..."
}

// statusLines gives the text of the Status dialog before the journal, as
// Get-ZapmanStatusLines does.
func statusLines(lang string, v backend.Version, st backend.Status, configErr, lastError string) []string {
	t := func(key string, args ...any) string { return i18n.T(lang, key, args...) }
	lines := []string{"Zapret Manager " + v.App, "ZapretSpec " + v.Spec}
	if configErr != "" {
		lines = append(lines, configErr)
	}
	if st.Running != "" {
		lines = append(lines, t("StatusLineRunning", st.Running))
	}
	if st.Installed != "" {
		lines = append(lines, t("StatusLineService", st.Installed))
	}
	for _, s := range []struct {
		name  string
		state backend.ServiceState
	}{{"zapret", st.Service}, {"WinDivert", st.Divert}} {
		switch s.state {
		case backend.ServiceRunning:
			lines = append(lines, t("StatusLineSvcRun", s.name))
		case backend.ServiceStopPending:
			lines = append(lines, t("StatusLineSvcPending", s.name))
		default:
			lines = append(lines, t("StatusLineSvcOff", s.name))
		}
	}
	if !st.DivertSys {
		lines = append(lines, t("StatusLineSysMissing"))
	}
	if len(st.BypassExes) == 0 {
		lines = append(lines, t("StatusLineBypassOff"))
	}
	for _, exe := range st.BypassExes {
		lines = append(lines, t("StatusLineBypassOn", exe))
	}
	if lastError != "" {
		lines = append(lines, "", t("StatusLineLastError"))
		lines = append(lines, strings.Split(strings.ReplaceAll(lastError, "\r\n", "\n"), "\n")...)
	}
	return lines
}

// The kinds of a test table cell. They select the colors.
const (
	cellInfo = "info"
	cellOK   = "ok"
	cellWarn = "warn"
	cellErr  = "err"
)

var (
	reCellErr  = regexp.MustCompile(`ERROR|FAIL|SSL`)
	reCellWarn = regexp.MustCompile(`UNSUP|TIMEOUT|BLOCK|NA`)
	reCellOK   = regexp.MustCompile(`OK|^\d`)
)

// cellKind gives the kind of a test status, as Get-GuiTestCellKind does.
func cellKind(text string) string {
	s := strings.TrimSpace(text)
	if s == "" || s == "-" || strings.EqualFold(s, "n/a") {
		return cellInfo
	}
	u := strings.ToUpper(s)
	switch {
	case reCellErr.MatchString(u):
		return cellErr
	case reCellWarn.MatchString(u):
		return cellWarn
	case reCellOK.MatchString(u):
		return cellOK
	}
	return cellInfo
}

// Cell is a cell of a test table.
type Cell struct {
	Text string
	Kind string
}

// Table is a test table. When Error is not empty, the frontend shows it in
// place of the rows.
type Table struct {
	Headers []string
	Rows    [][]Cell
	Error   string
}

// StrategyTab is the tab of a tested strategy.
type StrategyTab struct {
	Name     string
	HasError bool
	Table    Table
}

// strategyTab gives the tab of a test result, as New-GuiTestStrategyTable
// does.
func strategyTab(lang string, r backend.TestResult) StrategyTab {
	tab := StrategyTab{Name: r.Name}
	if tab.Name == "" {
		tab.Name = "?"
	}
	if r.Kind == "fail" || r.Error != "" {
		tab.HasError = true
		tab.Table.Error = r.Error
		return tab
	}
	dpi := r.Kind == string(backend.TestDPI)
	tab.Table.Headers = []string{i18n.T(lang, "TestsColTarget"), "HTTP", "TLS1.2", "TLS1.3"}
	if !dpi {
		tab.Table.Headers = append(tab.Table.Headers, i18n.T(lang, "TestsColPing"))
	}
	for _, tg := range r.Targets {
		row := []Cell{{Text: tg.Name, Kind: cellInfo}}
		vals := []string{tg.HTTP, tg.TLS12, tg.TLS13}
		if !dpi {
			vals = append(vals, tg.Ping)
		}
		for i, v := range vals {
			if v == "" && i < 3 {
				v = "-"
			}
			c := Cell{Text: v, Kind: cellKind(v)}
			if c.Kind == cellErr {
				tab.HasError = true
			}
			row = append(row, c)
		}
		tab.Table.Rows = append(tab.Table.Rows, row)
	}
	return tab
}

// summaryTable gives the summary table, as Add-GuiTestSummary does.
func summaryTable(lang string, s backend.TestSummary) Table {
	t := func(key string) string { return i18n.T(lang, key) }
	var tb Table
	if len(s.Rows) == 0 {
		return tb
	}
	std := s.Type == backend.TestStandard
	if std {
		tb.Headers = []string{t("TestsGrpStrats"), t("TestsSumOk"), t("TestsSumErr"), t("TestsSumUnsup"), t("TestsSumPingOk"), t("TestsSumPingFail")}
	} else {
		tb.Headers = []string{t("TestsGrpStrats"), t("TestsSumOk"), t("TestsSumFail"), t("TestsSumUnsup"), t("TestsSumBlocked")}
	}
	n := func(v int, bad string) Cell {
		c := Cell{Text: strconv.Itoa(v), Kind: cellOK}
		if v > 0 {
			c.Kind = bad
		}
		return c
	}
	for _, r := range s.Rows {
		row := []Cell{{Text: r.Name, Kind: cellInfo}, {Text: strconv.Itoa(r.OK), Kind: cellOK}}
		if std {
			row = append(row, n(r.Err, cellErr), n(r.Unsup, cellWarn), Cell{Text: strconv.Itoa(r.PingOK), Kind: cellOK}, n(r.PingFail, cellWarn))
		} else {
			row = append(row, n(r.Fail, cellErr), n(r.Unsup, cellWarn), n(r.Blocked, cellWarn))
		}
		tb.Rows = append(tb.Rows, row)
	}
	return tb
}

var reTestStep = regexp.MustCompile(`\[(\d+)/(\d+)\]`)

// testStep finds "[n/m]" in a test line.
func testStep(line string) (n, m int, ok bool) {
	sm := reTestStep.FindStringSubmatch(line)
	if sm == nil {
		return 0, 0, false
	}
	n, err1 := strconv.Atoi(sm[1])
	m, err2 := strconv.Atoi(sm[2])
	if err1 != nil || err2 != nil || m <= 0 {
		return 0, 0, false
	}
	return n, m, true
}

// fakeSlot is the combo box of a fake slot.
type fakeSlot struct {
	Items    []string
	Selected int
	offset   int
	current  string
	state    backend.FakeState
}

// newFakeSlot fills the combo box of a slot, as Add-FakeSlotComboItems
// does: when the active fake is not a file of the list, the first item is
// the text of the active fake.
func newFakeSlot(lang string, files []string, current string, state backend.FakeState) fakeSlot {
	s := fakeSlot{current: current, state: state}
	if state != backend.FakeMatched {
		text := i18n.T(lang, "FakesMissing")
		if state == backend.FakeUnlisted {
			text = i18n.T(lang, "FakesUnlisted")
		}
		s.Items = append(s.Items, text)
		s.offset = 1
	}
	for _, f := range files {
		s.Items = append(s.Items, f)
		if state == backend.FakeMatched && f == current {
			s.Selected = len(s.Items) - 1
		}
	}
	return s
}

// picked gives the file of the selected item, or "" when the slot does not
// change, as Get-FakeSlotPickedFile does.
func (s fakeSlot) picked(idx int) string {
	if idx < s.offset || idx >= len(s.Items) {
		return ""
	}
	f := s.Items[idx]
	if s.state == backend.FakeMatched && f == s.current {
		return ""
	}
	return f
}
