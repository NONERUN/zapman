package stratest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// record is the result of a strategy for the analytics and the file.
type record struct {
	name string
	kind string // Standard, DPI or KindFail
	err  string
	std  []stdResult
	dpi  []dpiResult
}

// runState is the state of a run.
type runState struct {
	sink Sink
}

func (t *Tester) line(s string) {
	if t.out != nil {
		t.out.sink.Line(s)
	}
}

// Run runs the tests of req. The log, the results and the summary go to
// sink. Run gives ErrPreflight when a check before the tests fails,
// ErrNoStrategies when no strategy is selected, and the error of ctx when ctx
// stops (errors.Is(err, context.Canceled)). The engine stops at the end.
// Run is not safe for concurrent use.
func (t *Tester) Run(ctx context.Context, req Request, sink Sink) error {
	if req.Type != Standard && req.Type != DPI {
		return fmt.Errorf("%w %q", ErrUnknownType, req.Type)
	}
	t.out = &runState{sink: sink}
	defer func() { t.out = nil }()

	bad := false
	if t.System.Elevated() {
		t.line("[OK] Administrator rights detected")
	} else {
		t.line("[ERROR] Run as Administrator to execute tests")
		bad = true
	}
	if t.CurlPath == "" {
		t.line("[ERROR] curl.exe not found")
		t.line("Install curl or add it to PATH")
		bad = true
	} else {
		t.line("[OK] curl.exe found")
	}
	if on, err := t.System.ServiceInstalled(); err == nil && on {
		t.line("[ERROR] Windows service 'zapret' is installed")
		t.line("         Remove the service before running tests")
		bad = true
	}
	if bad {
		t.line("")
		t.line("Fix the errors above and rerun.")
		return ErrPreflight
	}

	all, err := t.Engine.Names(req.Engine)
	if err != nil {
		t.line("[ERROR] " + err.Error())
		return err
	}
	t.line(fmt.Sprintf("Engine: %s", req.Engine))
	if len(all) == 0 {
		t.line(fmt.Sprintf("[ERROR] No strategies have %s flags.", req.Engine))
		return ErrNoStrategies
	}
	t.line(fmt.Sprintf("Test type: %s", req.Type))
	names := selectNames(all, req.Strategies)

	var dpiTargets []DPITarget
	var stdTargets []StdTarget
	nameLen := 10
	if req.Type == DPI {
		dpiTargets = t.dpiTargets(ctx)
	} else {
		stdTargets = StdTargets()
		t.line("")
		t.line(fmt.Sprintf("[INFO] Targets: %d", len(stdTargets)))
		for _, tg := range stdTargets {
			nameLen = max(nameLen, len([]rune(tg.Name)))
		}
	}
	if len(names) == 0 {
		t.line(fmt.Sprintf("[ERROR] No strategies to test for %s.", req.Engine))
		return ErrNoStrategies
	}

	const rule = "============================================================"
	t.line("")
	t.line(rule)
	t.line("                 ZAPRET CONFIG TESTS")
	t.line("                 Mode: " + strings.ToUpper(string(req.Type)))
	t.line(fmt.Sprintf("                 Engine: %s", req.Engine))
	t.line(fmt.Sprintf("                 Total configs: %2d", len(names)))
	t.line(rule)
	defer t.stopAll()
	t.line("[WARNING] Tests may take several minutes to complete. Please wait...")

	var recs []record
	for i, name := range names {
		if err := ctx.Err(); err != nil {
			t.line("[INFO] Tests cancelled.")
			return err
		}
		t.line("")
		t.line("------------------------------------------------------------")
		t.line(fmt.Sprintf("  [%d/%d] %s", i+1, len(names), name))
		t.line("------------------------------------------------------------")
		t.stopAll()
		t.line(fmt.Sprintf("  > Starting config (%s)...", req.Engine))
		run, err := t.start(ctx, name, req)
		if err != nil {
			if cerr := ctx.Err(); cerr != nil {
				t.line("[INFO] Tests cancelled.")
				return cerr
			}
			t.line("  > " + err.Error())
			rec := record{name: name, kind: KindFail, err: err.Error()}
			recs = append(recs, rec)
			sink.Strategy(Result{Name: name, Kind: KindFail, Error: rec.err})
			continue
		}
		rec := record{name: name, kind: string(req.Type)}
		res := Result{Name: name, Kind: string(req.Type)}
		if req.Type == Standard {
			rec.std = t.runStd(ctx, stdTargets, nameLen)
			for i := range rec.std {
				res.Targets = append(res.Targets, rec.std[i].Target())
			}
		} else {
			rec.dpi, err = t.runDPI(ctx, dpiTargets)
			for i := range rec.dpi {
				res.Targets = append(res.Targets, rec.dpi[i].Target())
			}
		}
		_ = run.Stop()
		if cerr := ctx.Err(); cerr != nil {
			t.line("[INFO] Tests cancelled.")
			return cerr
		}
		if err != nil {
			return err
		}
		recs = append(recs, rec)
		sink.Strategy(res)
	}

	t.line("")
	t.line("All tests finished.")
	rows := analytics(recs)
	sum := Summary{Type: req.Type}
	if len(rows) == 0 {
		t.line("No completed strategy results.")
		sink.Summary(sum)
		if len(recs) > 0 {
			t.save(recs, rows, "")
		}
		return nil
	}
	t.line("")
	t.line("=== ANALYTICS ===")
	for _, l := range analyticsLines(rows) {
		t.line(l)
	}
	sum.Rows = make([]SumRow, len(rows))
	for i, r := range rows {
		sum.Rows[i] = r.SumRow
	}
	sum.Best = best(rows)
	t.line("")
	t.line("Best config: " + sum.Best)
	t.line("")
	sink.Summary(sum)
	t.save(recs, rows, sum.Best)
	return nil
}

// start sets the TCP timestamps on and starts the strategy.
func (t *Tester) start(ctx context.Context, name string, req Request) (Running, error) {
	if err := t.System.EnableTimestamps(); err != nil {
		return nil, err
	}
	return t.Engine.Start(ctx, name, req.Engine)
}

func (t *Tester) stopAll() {
	if err := t.Engine.StopAll(); err != nil {
		t.line("[WARN] Stop the engines: " + err.Error())
	}
}

// selectNames gives the names of all that want selects, in the order of
// all. The compare ignores the case.
func selectNames(all, want []string) []string {
	var w []string
	for _, n := range want {
		if n = strings.ToLower(strings.TrimSpace(n)); n != "" {
			w = append(w, n)
		}
	}
	if len(w) == 0 || w[0] == "all" || w[0] == "*" {
		return all
	}
	var out []string
	for _, n := range all {
		l := strings.ToLower(n)
		if slices.Contains(w, l) || slices.Contains(w, l+".json") {
			out = append(out, n)
		}
	}
	return out
}

// row is a summary row with its type.
type row struct {
	SumRow
	kind string
}

// analytics counts the statuses of each strategy that has results.
func analytics(recs []record) []row {
	var rows []row
	for _, rec := range recs {
		r := row{SumRow: SumRow{Name: rec.name}, kind: rec.kind}
		switch rec.kind {
		case string(Standard):
			if len(rec.std) == 0 {
				continue
			}
			for _, s := range rec.std {
				for _, st := range s.status {
					switch st {
					case stdOK:
						r.OK++
					case stdSSL, stdError:
						r.Err++
					case stdUnsup:
						r.Unsup++
					}
				}
				if s.ping != pingTimeout && s.ping != pingNone {
					r.PingOK++
				} else {
					r.PingFail++
				}
			}
		case string(DPI):
			if len(rec.dpi) == 0 {
				continue
			}
			for _, d := range rec.dpi {
				for _, l := range d.lines {
					switch l.Status {
					case dpiOK:
						r.OK++
					case dpiFail:
						r.Fail++
					case dpiUnsup:
						r.Unsup++
					case dpiBlocked:
						r.Blocked++
					}
				}
			}
		default:
			continue
		}
		rows = append(rows, r)
	}
	return rows
}

// analyticsLines gives the lines of "=== ANALYTICS ===".
func analyticsLines(rows []row) []string {
	n := 0
	for _, r := range rows {
		n = max(n, len([]rune(r.Name)))
	}
	out := make([]string, len(rows))
	for i, r := range rows {
		name := padRight(r.Name, n)
		if r.kind == string(Standard) {
			out[i] = fmt.Sprintf("%s : HTTP OK: %3d, ERR: %3d, UNSUP: %3d, Ping OK: %3d, Fail: %3d", name, r.OK, r.Err, r.Unsup, r.PingOK, r.PingFail)
		} else {
			out[i] = fmt.Sprintf("%s : OK: %3d, FAIL: %3d, UNSUP: %3d, BLOCKED: %3d", name, r.OK, r.Fail, r.Unsup, r.Blocked)
		}
	}
	return out
}

// best is the rule of the PowerShell version: the most OK; on a tie, the
// most PingOK (the DPI tests have 0). A first strategy with 0 OK is also
// the best when no other strategy has more.
func best(rows []row) string {
	name, maxOK, maxPing := "", 0, -1
	for _, r := range rows {
		ping := 0
		if r.kind == string(Standard) {
			ping = r.PingOK
		}
		if r.OK > maxOK || (r.OK == maxOK && ping > maxPing) {
			name, maxOK, maxPing = r.Name, r.OK, ping
		}
	}
	return name
}

// resultText is the text of the result file (Save-ZapmanTestResultLog).
func resultText(recs []record, rows []row, bestName string) string {
	var lines []string
	add := func(s string) { lines = append(lines, s) }
	for _, rec := range recs {
		add(fmt.Sprintf("Config: %s (Type: %s)", rec.name, rec.kind))
		switch rec.kind {
		case KindFail:
			for l := range strings.Lines(rec.err) {
				add("  " + strings.TrimRight(l, "\r\n"))
			}
		case string(Standard):
			for i := range rec.std {
				s := &rec.std[i]
				add(fmt.Sprintf("  %s : %s | Ping: %s", s.target.Name, strings.Join(s.tokens(), " "), s.ping))
			}
		case string(DPI):
			for _, d := range rec.dpi {
				if d.target.Country != "" {
					add(fmt.Sprintf("  Target: [%s] %s (%s)", d.target.Country, d.target.ID, d.target.Provider))
				} else {
					add(fmt.Sprintf("  Target: %s (%s)", d.target.ID, d.target.Provider))
				}
				for _, l := range d.lines {
					add(fmt.Sprintf("    %s: code=%s  up=%s KB  down=%s KB  time=%ss  status=%s",
						l.Label, l.Code, kb(l.Up), kb(l.Down), num(l.Time), l.Status))
				}
			}
		}
		add("")
	}
	if len(rows) > 0 {
		add("=== ANALYTICS ===")
		lines = append(lines, analyticsLines(rows)...)
		if strings.TrimSpace(bestName) != "" {
			add("Best strategy: " + bestName)
		}
	}
	return strings.Join(lines, "\r\n") + "\r\n"
}

// save writes the result file and writes its path to the log.
func (t *Tester) save(recs []record, rows []row, bestName string) {
	if t.ResultsDir == "" {
		return
	}
	now := time.Now
	if t.Now != nil {
		now = t.Now
	}
	path := filepath.Join(t.ResultsDir, "test_results_"+now().Format("2006-01-02_15-04-05")+".txt")
	data := append([]byte("\uFEFF"), resultText(recs, rows, bestName)...)
	err := os.MkdirAll(t.ResultsDir, 0o755)
	if err == nil {
		err = os.WriteFile(path, data, 0o644)
	}
	if err != nil {
		t.line("[WARN] Save the results failed: " + err.Error())
		return
	}
	t.line("Results saved to " + path)
}
