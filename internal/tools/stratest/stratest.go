// Package stratest is the strategy test runner of zapman. It is the port of
// Invoke-ZapmanStrategyTestsCore of Tests.ps1: the standard tests (curl to
// HTTP targets and a ping) and the DPI tests (the TCP 16-20 KB freeze checks
// of hyperion-cs/dpi-checkers), with the same log lines, the same analytics
// and the same rule for the best strategy.
//
// The Tester gets all parts of the system through its fields: the engine,
// the system checks, curl, the ping and the DPI suite. Thus the tests do not
// start processes and do not use the network. New gives the real parts.
//
// The runner does not change the IPSet mode and does not restore the
// engines that ran before the tests. The caller does these steps.
package stratest

import (
	"context"
	"errors"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/NONERUN/zapman/internal/spec"
	"github.com/NONERUN/zapman/internal/tools/command"
)

// Type is the type of the tests.
type Type string

// The test types.
const (
	Standard Type = "standard"
	DPI      Type = "dpi"
)

// KindFail is the Kind of a Result when the strategy did not start.
const KindFail = "fail"

// Request selects the tests.
type Request struct {
	Type Type
	// Strategies are the names of the strategies. An empty list, "all" or
	// "*" as the first name selects all strategies. The names that are not
	// in the catalog are ignored. The order is the order of the catalog.
	Strategies []string
	Engine     spec.Engine
}

// Target is the result of a target. For the standard tests, HTTP, TLS12 and
// TLS13 are "OK", "SSL", "UNSUP" or "ERROR", and they are "" for a target
// with only a ping. For the DPI tests, they are "OK", "FAIL", "UNSUPPORTED"
// or "LIKELY_BLOCKED", and Ping is "".
type Target struct {
	Name  string
	HTTP  string
	TLS12 string
	TLS13 string
	// Ping is "N ms", "Timeout" or "n/a".
	Ping string
}

// Result is the result of a strategy.
type Result struct {
	Name string
	// Kind is the Type, or KindFail when the strategy did not start.
	Kind string
	// Error is the text of the start failure.
	Error   string
	Targets []Target
}

// SumRow is the result of a strategy in the summary. The standard tests use
// OK, Err, Unsup, PingOK and PingFail. The DPI tests use OK, Fail, Unsup and
// Blocked.
type SumRow struct {
	Name     string
	OK       int
	Err      int
	Unsup    int
	PingOK   int
	PingFail int
	Fail     int
	Blocked  int
}

// Summary is the result of all strategies.
type Summary struct {
	Type Type
	// Best is the best strategy, or "" when no strategy has results.
	Best string
	Rows []SumRow
}

// Sink gets the output of the tests.
type Sink interface {
	// Line is a line of the test log.
	Line(text string)
	// Strategy is the result of a strategy.
	Strategy(r Result)
	// Summary is the result of all strategies.
	Summary(s Summary)
}

// Running is an engine that runs.
type Running interface {
	// Stop stops the engine and waits for it.
	Stop() error
}

// Engine starts the strategies.
type Engine interface {
	// Names gives the strategies of the catalog that the engine e can run,
	// in the order of the catalog. It gives an error when the files of e
	// are missing.
	Names(e spec.Engine) ([]string, error)
	// Start starts the strategy name with the engine e and gives the
	// engine when it is ready.
	Start(ctx context.Context, name string, e spec.Engine) (Running, error)
	// StopAll stops all winws.exe and winws2.exe processes.
	StopAll() error
}

// System does the checks before the tests.
type System interface {
	// Elevated tells if the process has administrator rights.
	Elevated() bool
	// ServiceInstalled tells if the zapret service is installed.
	ServiceInstalled() (bool, error)
	// EnableTimestamps sets the TCP timestamps on.
	EnableTimestamps() error
}

// PingFunc sends one ICMP echo to host and gives the round trip time. An
// error is a timeout.
type PingFunc func(ctx context.Context, host string, timeout time.Duration) (time.Duration, error)

// SuiteFunc gives the targets of the DPI tests.
type SuiteFunc func(ctx context.Context) ([]DPITarget, error)

// Settings are the times and limits of the tests.
type Settings struct {
	// StdTimeout is the curl time limit of the standard tests, in seconds.
	StdTimeout int
	// StdParallel is the number of targets that the standard tests check
	// at the same time.
	StdParallel int
	// DPITimeout is the curl time limit of the DPI tests, in seconds.
	DPITimeout int
	// DPIRange is the number of bytes of the DPI payload.
	DPIRange int
	// DPIParallel is the number of DPI targets at the same time.
	DPIParallel int
	// CustomHost, when it is not "", is the only DPI target.
	CustomHost string
}

// DefaultParallel is min(16, max(8, 2 * number of CPUs)).
func DefaultParallel() int {
	return min(16, max(8, runtime.NumCPU()*2))
}

// DefaultSettings gives the defaults of the PowerShell version.
func DefaultSettings() Settings {
	p := DefaultParallel()
	return Settings{StdTimeout: 4, StdParallel: p, DPITimeout: 5, DPIRange: 65536, DPIParallel: p}
}

// SettingsFromEnv gives the defaults with the changes of the environment
// variables of the PowerShell version: TEST_CURL_TIMEOUT,
// TEST_MAX_PARALLEL, MONITOR_TIMEOUT, MONITOR_RANGE, MONITOR_MAX_PARALLEL
// and MONITOR_HOST. A value that is not a positive number is ignored.
// getenv nil is os.Getenv.
func SettingsFromEnv(getenv func(string) string) Settings {
	if getenv == nil {
		getenv = os.Getenv
	}
	s := DefaultSettings()
	envInt := func(name string, v *int) {
		if n, err := strconv.Atoi(strings.TrimSpace(getenv(name))); err == nil && n > 0 {
			*v = n
		}
	}
	envInt("TEST_CURL_TIMEOUT", &s.StdTimeout)
	envInt("TEST_MAX_PARALLEL", &s.StdParallel)
	envInt("MONITOR_TIMEOUT", &s.DPITimeout)
	envInt("MONITOR_RANGE", &s.DPIRange)
	envInt("MONITOR_MAX_PARALLEL", &s.DPIParallel)
	s.CustomHost = strings.TrimSpace(getenv("MONITOR_HOST"))
	return s
}

// The errors of Run. The log has the details.
var (
	ErrPreflight    = errors.New("the strategy tests cannot start: see the test log")
	ErrNoStrategies = errors.New("no strategies to test")
	ErrUnknownType  = errors.New("unknown test type")
)

// Tester runs the tests.
type Tester struct {
	Engine Engine
	System System
	// Curl runs curl.exe; CurlPath is its path, "" when curl is missing.
	Curl     command.Func
	CurlPath string
	Ping     PingFunc
	Suite    SuiteFunc
	Settings Settings
	// ResultsDir is the folder of the result files (test-results in the
	// zapman folder). "" writes no file.
	ResultsDir string
	// TempDir is the folder of the DPI payload file. "" is os.TempDir.
	TempDir string
	// Now is the clock for the name of the result file. Nil is time.Now.
	Now func() time.Time

	out *runState
}
