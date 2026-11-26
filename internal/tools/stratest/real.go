package stratest

import (
	"context"
	"errors"
	"net/http"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/NONERUN/zapman/internal/config"
	"github.com/NONERUN/zapman/internal/engine"
	"github.com/NONERUN/zapman/internal/service"
	"github.com/NONERUN/zapman/internal/spec"
	"github.com/NONERUN/zapman/internal/tools/command"
)

// LayoutEngine is the Engine of a zapman folder. Each strategy gets a copy
// of Config with the strategy and the engine of the test; the games and the
// Game Filter stay.
type LayoutEngine struct {
	Layout  engine.Layout
	Catalog *spec.Catalog
	Config  config.Config
	Runner  engine.Runner
	// Host stops the engine processes.
	Host service.Host
}

// Names gives an error when the exe of e is missing.
func (l *LayoutEngine) Names(e spec.Engine) ([]string, error) {
	if _, err := l.Layout.Exe(e); err != nil {
		return nil, err
	}
	return l.Catalog.StrategyNames()
}

// Start builds the strategy and runs it. The engine is ready when it runs
// after the start window of the Runner.
func (l *LayoutEngine) Start(ctx context.Context, name string, e spec.Engine) (Running, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cfg := l.Config
	cfg.Strategy, cfg.Engine = name, e
	b, err := l.Layout.Build(l.Catalog, &cfg)
	if err != nil {
		return nil, err
	}
	return l.Runner.Run(b)
}

// StopAll stops all engine processes.
func (l *LayoutEngine) StopAll() error {
	_, err := l.Host.KillEngines()
	return err
}

// HostSystem is the System of a service.Host.
type HostSystem struct {
	Host service.Host
}

// Elevated is Host.Elevated.
func (s HostSystem) Elevated() bool { return s.Host.Elevated() }

// ServiceInstalled tells if the zapret service exists.
func (s HostSystem) ServiceInstalled() (bool, error) {
	_, err := s.Host.Query(service.Name)
	if errors.Is(err, service.ErrNotInstalled) {
		return false, nil
	}
	return err == nil, err
}

// EnableTimestamps is service.Manager.EnableTCPTimestamps.
func (s HostSystem) EnableTimestamps() error {
	m := service.Manager{Host: s.Host}
	return m.EnableTCPTimestamps()
}

// FindCurl gives the path of curl.exe: from PATH, else from System32. It
// gives "" when curl is missing.
func FindCurl() string {
	if p, err := exec.LookPath("curl.exe"); err == nil {
		if abs, err := filepath.Abs(p); err == nil {
			return abs
		}
		return p
	}
	p := command.System32("curl.exe")
	if _, err := exec.LookPath(p); err == nil {
		return p
	}
	return ""
}

// New gives the Tester of this computer for the zapman folder of l, with
// the catalog cat and the user config cfg. The settings come from the
// environment; the results go to <root>\test-results.
func New(l engine.Layout, cat *spec.Catalog, cfg config.Config) *Tester {
	host := service.NewHost()
	s := SettingsFromEnv(nil)
	client := &http.Client{}
	return &Tester{
		Engine:   &LayoutEngine{Layout: l, Catalog: cat, Config: cfg, Host: host},
		System:   HostSystem{Host: host},
		Curl:     command.Run,
		CurlPath: FindCurl(),
		Ping:     Ping,
		Suite: func(ctx context.Context) ([]DPITarget, error) {
			return FetchSuite(ctx, client, SuiteURL, time.Duration(s.DPITimeout)*time.Second)
		},
		Settings:   s,
		ResultsDir: filepath.Join(l.Root, "test-results"),
	}
}
