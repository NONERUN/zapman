package stratest

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// StdTarget is a target of the standard tests.
type StdTarget struct {
	Name string
	// URL is "" for a target with only a ping.
	URL string
	// PingHost is the host of the ping, or "".
	PingHost string
}

// The targets of the standard tests (Get-ZapmanTestTargets). A value
// "PING:<ip>" is a target with only a ping.
var stdTargets = [][2]string{
	{"DiscordMain", "https://discord.com"},
	{"DiscordGateway", "https://gateway.discord.gg"},
	{"DiscordCDN", "https://cdn.discordapp.com"},
	{"DiscordUpdates", "https://updates.discord.com"},
	{"YouTubeWeb", "https://www.youtube.com"},
	{"YouTubeShort", "https://youtu.be"},
	{"YouTubeImage", "https://i.ytimg.com"},
	{"YouTubeVideoRedirect", "https://redirector.googlevideo.com"},
	{"GoogleMain", "https://www.google.com"},
	{"GoogleGstatic", "https://www.gstatic.com"},
	{"CloudflareWeb", "https://www.cloudflare.com"},
	{"CloudflareCDN", "https://cdnjs.cloudflare.com"},
	{"CloudflareDNS1111", "PING:1.1.1.1"},
	{"CloudflareDNS1001", "PING:1.0.0.1"},
	{"GoogleDNS8888", "PING:8.8.8.8"},
	{"GoogleDNS8844", "PING:8.8.4.4"},
	{"Quad9DNS9999", "PING:9.9.9.9"},
}

var (
	rePingPrefix = regexp.MustCompile(`(?i)^PING:\s*`)
	reScheme     = regexp.MustCompile(`(?i)^https?://`)
	rePath       = regexp.MustCompile(`/.*$`)
)

// ParseTarget is Convert-Target: "PING:<host>" gives a target with only a
// ping; a URL gives the URL and its host as the ping host.
func ParseTarget(name, value string) StdTarget {
	if rePingPrefix.MatchString(value) {
		return StdTarget{Name: name, PingHost: rePingPrefix.ReplaceAllString(value, "")}
	}
	host := rePath.ReplaceAllString(reScheme.ReplaceAllString(value, ""), "")
	return StdTarget{Name: name, URL: value, PingHost: host}
}

// StdTargets gives the targets of the standard tests.
func StdTargets() []StdTarget {
	out := make([]StdTarget, len(stdTargets))
	for i, t := range stdTargets {
		out[i] = ParseTarget(t[0], t[1])
	}
	return out
}

// probe is a curl test: the label and the curl options.
type probe struct {
	label string
	args  []string
}

// The tests of each target, for the two test types.
var probes = []probe{
	{"HTTP", []string{"--http1.1"}},
	{"TLS1.2", []string{"--tlsv1.2", "--tls-max", "1.2"}},
	{"TLS1.3", []string{"--tlsv1.3", "--tls-max", "1.3"}},
}

// The statuses of the standard tests.
const (
	stdOK    = "OK"
	stdSSL   = "SSL"
	stdUnsup = "UNSUP"
	stdError = "ERROR"
)

// The ping results.
const (
	pingTimeout = "Timeout"
	pingNone    = "n/a"
)

var (
	reStdSSL   = regexp.MustCompile(`(?i)Could not resolve host|certificate|SSL certificate problem|self[- ]?signed|certificate verify failed|unable to get local issuer certificate`)
	reStdUnsup = regexp.MustCompile(`(?i)does not support|not supported|protocol\s+'?.+'?\s+not\s+supported|unsupported protocol|TLS.*not supported|Unrecognized option|Unknown option|unsupported option|unsupported feature|schannel`)
	reHTTPCode = regexp.MustCompile(`^[1-5]\d{2}$`)
)

// ClassifyStd gives the status of a curl run of the standard tests from its
// output (stdout and stderr), its exit code and the error of the start.
func ClassifyStd(out string, code int, err error) string {
	switch {
	case err != nil:
		return stdError
	case reStdSSL.MatchString(out):
		return stdSSL
	case code == 35 || reStdUnsup.MatchString(out):
		return stdUnsup
	case code == 0 && reHTTPCode.MatchString(strings.TrimSpace(out)):
		return stdOK
	}
	return stdError
}

// stdResult is the result of a standard target.
type stdResult struct {
	target StdTarget
	// status has the status of each probe; it is empty for a target with
	// only a ping.
	status []string
	ping   string
}

// token is the token of the PowerShell version, for example "HTTP:OK   ".
func token(label, status string) string {
	return fmt.Sprintf("%s:%-5s", label, status)
}

func (r *stdResult) tokens() []string {
	out := make([]string, len(r.status))
	for i, s := range r.status {
		out[i] = token(probes[i].label, s)
	}
	return out
}

func (r *stdResult) Target() Target {
	t := Target{Name: r.target.Name, Ping: r.ping}
	if len(r.status) == len(probes) {
		t.HTTP, t.TLS12, t.TLS13 = r.status[0], r.status[1], r.status[2]
	}
	return t
}

// checkStd checks one target.
func (t *Tester) checkStd(ctx context.Context, tg StdTarget) stdResult {
	res := stdResult{target: tg, ping: pingNone}
	sec := t.Settings.StdTimeout
	if tg.URL != "" {
		base := []string{"-I", "-s", "-m", strconv.Itoa(sec), "--connect-timeout", strconv.Itoa(min(2, sec)),
			"-o", "NUL", "-w", "%{http_code}", "--show-error"}
		for _, p := range probes {
			args := append(append(append([]string{}, base...), p.args...), tg.URL)
			r, err := t.Curl(ctx, t.CurlPath, args...)
			res.status = append(res.status, ClassifyStd(r.Output, r.Code, err))
		}
	}
	if tg.PingHost != "" {
		res.ping = pingTimeout
		if d, err := t.Ping(ctx, tg.PingHost, time.Second); err == nil {
			res.ping = fmt.Sprintf("%d ms", d.Milliseconds())
		}
	}
	return res
}

// runStd runs the standard tests of one strategy and writes the rows.
func (t *Tester) runStd(ctx context.Context, targets []StdTarget, nameLen int) []stdResult {
	t.line("  > Running tests...")
	wait := time.Duration(t.Settings.StdTimeout*3+5) * time.Second
	chans := parallel(ctx, len(targets), t.Settings.StdParallel, wait, func(ctx context.Context, i int) stdResult {
		return t.checkStd(ctx, targets[i])
	})
	out := make([]stdResult, 0, len(targets))
	for _, ch := range chans {
		r := <-ch
		out = append(out, r)
		var b strings.Builder
		b.WriteString("  " + padRight(r.target.Name, nameLen) + "    ")
		if r.target.URL != "" {
			for _, tok := range r.tokens() {
				b.WriteString(" " + tok)
			}
			b.WriteString(" | Ping: " + r.ping)
		} else {
			b.WriteString(" Ping: " + r.ping)
		}
		t.line(b.String())
	}
	return out
}

// parallel runs f for 0..n-1 with at most par calls at the same time. Each
// call gets a context with the time limit wait. The channel i gets the
// result of call i.
func parallel[T any](ctx context.Context, n, par int, wait time.Duration, f func(context.Context, int) T) []chan T {
	if par < 1 {
		par = 1
	}
	sem := make(chan struct{}, par)
	chans := make([]chan T, n)
	for i := range n {
		chans[i] = make(chan T, 1)
		go func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			c, cancel := context.WithTimeout(ctx, wait)
			defer cancel()
			chans[i] <- f(c, i)
		}()
	}
	return chans
}

// padRight is String.PadRight: it counts the characters, not the bytes.
func padRight(s string, n int) string {
	if k := len([]rune(s)); k < n {
		return s + strings.Repeat(" ", n-k)
	}
	return s
}
