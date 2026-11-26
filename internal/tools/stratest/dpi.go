package stratest

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// SuiteURL is the DPI suite of hyperion-cs/dpi-checkers (Apache-2.0).
const SuiteURL = "https://hyperion-cs.github.io/dpi-checkers/ru/tcp-16-20/suite.v2.json"

// suiteLimit is the maximum size of the suite in bytes.
const suiteLimit = 4 << 20

// DPITarget is a target of the DPI tests.
type DPITarget struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	Country  string `json:"country"`
	Host     string `json:"host"`
}

// customTarget is the target of Settings.CustomHost.
func customTarget(host string) DPITarget {
	return DPITarget{ID: "CUSTOM", Provider: "Custom", Country: "💡", Host: host}
}

// FetchSuite gets the DPI suite from url with the time limit timeout.
// client nil is http.DefaultClient.
func FetchSuite(ctx context.Context, client *http.Client, url string, timeout time.Duration) ([]DPITarget, error) {
	if client == nil {
		client = http.DefaultClient
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "zapman")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get %s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, suiteLimit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > suiteLimit {
		return nil, fmt.Errorf("the DPI suite is larger than %d bytes", suiteLimit)
	}
	var out []DPITarget
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("read the DPI suite: %w", err)
	}
	return out, nil
}

// The statuses of the DPI tests.
const (
	dpiOK      = "OK"
	dpiFail    = "FAIL"
	dpiUnsup   = "UNSUPPORTED"
	dpiBlocked = "LIKELY_BLOCKED"
)

var (
	reDPIOut   = regexp.MustCompile(`^(\d{3})\s+(\d+)\s+(\d+)\s+([\d.]+)$`)
	reDPIUnsup = regexp.MustCompile(`(?i)not supported|does not support|protocol\s+'.+'\s+not\s+supported|protocol\s+.+\s+not\s+supported|unsupported protocol|TLS.not supported|Unrecognized option|Unknown option|unsupported option|unsupported feature|schannel|SSL`)
)

// DPILine is the result of one curl run of the DPI tests.
type DPILine struct {
	Label    string
	Code     string // the HTTP code, "UNSUP", "ERR" or "NA"
	Up, Down int64  // bytes
	Time     float64
	Status   string
}

// ClassifyDPI gives the line of a curl run from its output, its exit code
// and the time limit in seconds. A start error must come as code -1 and an
// empty output.
func ClassifyDPI(label, out string, code, timeout int) DPILine {
	l := DPILine{Label: label, Code: "NA", Time: -1, Status: dpiOK}
	out = strings.TrimSpace(out)
	if m := reDPIOut.FindStringSubmatch(out); m != nil {
		l.Code = m[1]
		l.Up, _ = strconv.ParseInt(m[2], 10, 64)
		l.Down, _ = strconv.ParseInt(m[3], 10, 64)
		l.Time, _ = strconv.ParseFloat(m[4], 64)
	} else if code == 35 || reDPIUnsup.MatchString(out) {
		l.Code = "UNSUP"
	} else if out != "" {
		l.Code = "ERR"
	}
	switch {
	case l.Code == "UNSUP":
		l.Status = dpiUnsup
	case code != 0 || l.Code == "ERR" || l.Code == "NA":
		l.Status = dpiFail
	}
	if l.Up > 0 && l.Down == 0 && l.Time >= float64(timeout) && code != 0 {
		l.Status = dpiBlocked
	}
	return l
}

// kb is [math]::Round(bytes / 1024, 1).
func kb(n int64) string {
	return num(math.RoundToEven(float64(n)/1024*10) / 10)
}

// num formats a number as PowerShell does: no zeros at the end.
func num(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

// String is the log line of the PowerShell version.
func (l *DPILine) String() string {
	return fmt.Sprintf("[%s] code=%s buf_up=%d bytes (%s KB) buf_down=%d bytes (%s KB) time=%ss status=%s",
		l.Label, l.Code, l.Up, kb(l.Up), l.Down, kb(l.Down), num(l.Time), l.Status)
}

// dpiResult is the result of a DPI target.
type dpiResult struct {
	target DPITarget
	lines  []DPILine
	warned bool
}

func (r *dpiResult) Target() Target {
	t := Target{Name: r.target.ID}
	for _, l := range r.lines {
		switch l.Label {
		case "HTTP":
			t.HTTP = l.Status
		case "TLS1.2":
			t.TLS12 = l.Status
		case "TLS1.3":
			t.TLS13 = l.Status
		}
	}
	return t
}

// checkDPI checks one target.
func (t *Tester) checkDPI(ctx context.Context, tg DPITarget, payload string) dpiResult {
	sec := t.Settings.DPITimeout
	res := dpiResult{target: tg}
	for _, p := range probes {
		args := []string{
			"--range", fmt.Sprintf("0-%d", t.Settings.DPIRange-1),
			"-m", strconv.Itoa(sec),
			"--connect-timeout", strconv.Itoa(min(3, sec)),
			"-w", "%{http_code} %{size_upload} %{size_download} %{time_total}",
			"-o", "NUL",
			"-X", "POST",
			"--data-binary", "@" + payload,
			"-s",
		}
		args = append(append(args, p.args...), "https://"+tg.Host)
		r, err := t.Curl(ctx, t.CurlPath, args...)
		if err != nil {
			r.Output, r.Code = "", -1
		}
		l := ClassifyDPI(p.label, r.Output, r.Code, sec)
		if l.Status == dpiBlocked {
			res.warned = true
		}
		res.lines = append(res.lines, l)
	}
	return res
}

// writePayload makes a file with DPIRange random bytes.
func (t *Tester) writePayload() (string, error) {
	f, err := os.CreateTemp(t.TempDir, "zapman-dpi-*.bin")
	if err != nil {
		return "", err
	}
	buf := make([]byte, t.Settings.DPIRange)
	_, _ = rand.Read(buf) // crypto/rand.Read does not fail
	_, err = f.Write(buf)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// dpiTargets gives the targets of the DPI tests.
func (t *Tester) dpiTargets(ctx context.Context) []DPITarget {
	if t.Settings.CustomHost != "" {
		return []DPITarget{customTarget(t.Settings.CustomHost)}
	}
	ts, err := t.Suite(ctx)
	if err != nil {
		t.line("[WARN] Fetch dpi suite failed.")
		return nil
	}
	return ts
}

// runDPI is Invoke-DpiSuite: it runs the DPI tests of one strategy and
// writes the log.
func (t *Tester) runDPI(ctx context.Context, targets []DPITarget) ([]dpiResult, error) {
	s := t.Settings
	t.line("  > Running DPI checkers...")
	t.line(fmt.Sprintf("[INFO] Targets: %d (custom URL overrides suite). Range: 0-%d bytes; Timeout: %ds", len(targets), s.DPIRange-1, s.DPITimeout))
	t.line(fmt.Sprintf("[INFO] Starting DPI TCP 16-20 checks (parallel: %d)...", s.DPIParallel))
	payload, err := t.writePayload()
	if err != nil {
		return nil, fmt.Errorf("make the DPI payload: %w", err)
	}
	defer os.Remove(payload) //nolint:errcheck // a temp file; a failure leaves only a small file.
	wait := time.Duration(s.DPITimeout*3+5) * time.Second
	chans := parallel(ctx, len(targets), s.DPIParallel, wait, func(ctx context.Context, i int) dpiResult {
		return t.checkDPI(ctx, targets[i], payload)
	})
	warn := false
	out := make([]dpiResult, 0, len(targets))
	for _, ch := range chans {
		r := <-ch
		out = append(out, r)
		t.line("")
		t.line(fmt.Sprintf("=== [%s][%s] %s ===", r.target.Country, r.target.Provider, r.target.ID))
		for i := range r.lines {
			l := &r.lines[i]
			t.line(l.String())
			if l.Status == dpiBlocked {
				t.line("  Pattern matches 16-20KB freeze; censor likely cutting this strategy.")
			}
		}
		if r.warned {
			warn = true
		} else {
			t.line("  No 16-20KB freeze pattern for this target.")
		}
	}
	t.line("")
	if warn {
		t.line("[WARNING] Detected possible DPI TCP 16-20 blocking on one or more targets. Consider changing strategy/SNI/IP.")
	} else {
		t.line("[OK] No 16-20KB freeze pattern detected across targets.")
	}
	return out, nil
}
