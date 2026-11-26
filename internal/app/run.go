package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/NONERUN/zapman/internal/config"
	"github.com/NONERUN/zapman/internal/engine"
	"github.com/NONERUN/zapman/internal/gui/backend"
	"github.com/NONERUN/zapman/internal/service"
	"github.com/NONERUN/zapman/internal/spec"
)

// build builds the strategy name for the engine e with the other settings
// of user\config.json (games, Game Filter). It makes the missing user
// lists and active fakes. When the file is not correct, the defaults of the fields that are
// not correct apply.
func (b *Backend) build(ctx context.Context, name string, e spec.Engine) (*engine.Build, error) {
	if err := b.CheckEngine(ctx, e); err != nil {
		return nil, err
	}
	cat, err := b.l.LoadCatalog()
	if err != nil {
		return nil, err
	}
	st, err := cat.Strategy(name)
	if err != nil {
		return nil, err
	}
	if !supports(st, e) {
		return nil, backend.Msg("EngineNoFlags", string(e))
	}
	b.cfg.Lock()
	c, _ := b.loadConfig()
	b.cfg.Unlock()
	c.Strategy = name
	c.Engine = e
	// Make the user files first: the argv then has the new active fakes.
	if err := b.l.EnsureUserFiles(cat, gamesOf(cat, &c)); err != nil {
		return nil, fmt.Errorf("make the user files: %w", err)
	}
	return b.l.Build(cat, &c)
}

// supports tells if the strategy has flags for the engine e: spec.Build
// with no games works. backend.LoadCatalogInfo uses the same rule.
func supports(st *spec.Strategy, e spec.Engine) bool {
	off := spec.GameFilter{TCP: config.OffPort, UDP: config.OffPort}
	_, err := spec.Build(st, nil, e, spec.Paths{}, off)
	return err == nil
}

// elevated gives AdminRequired when the process has no administrator
// rights.
func (b *Backend) elevated() error {
	if !b.mgr.Host.Elevated() {
		return backend.Msg("AdminRequired")
	}
	return nil
}

// stopEngines stops the process of Run, the zapret service and all other
// engine processes.
func (b *Backend) stopEngines() error {
	if err := b.stopRun(); err != nil {
		return err
	}
	if err := b.mgr.Stop(); err != nil {
		return mapErr(err)
	}
	if _, err := b.mgr.Host.KillEngines(); err != nil {
		return fmt.Errorf("stop the engine processes: %w", err)
	}
	return nil
}

// StartService implements backend.Backend. It stops the process of Run
// first: two engines on the same traffic do not work.
func (b *Backend) StartService(context.Context) error {
	b.op.Lock()
	defer b.op.Unlock()
	if err := b.elevated(); err != nil {
		return err
	}
	if err := b.stopRun(); err != nil {
		return err
	}
	return mapErr(b.mgr.Start())
}

// Stop implements backend.Backend.
func (b *Backend) Stop(context.Context) error {
	b.op.Lock()
	defer b.op.Unlock()
	if err := b.elevated(); err != nil {
		return err
	}
	return b.stopEngines()
}

// RemoveServices implements backend.Backend.
func (b *Backend) RemoveServices(context.Context) error {
	b.op.Lock()
	defer b.op.Unlock()
	if err := b.elevated(); err != nil {
		return err
	}
	if err := b.stopRun(); err != nil {
		return err
	}
	return mapErr(b.mgr.Remove())
}

// Run implements backend.Backend. It stops the service and all engine
// processes, turns on the TCP timestamps and starts the engine. The Backend
// keeps the process: Stop stops it.
func (b *Backend) Run(ctx context.Context, name string, e spec.Engine) error {
	b.op.Lock()
	defer b.op.Unlock()
	if err := b.elevated(); err != nil {
		return err
	}
	bd, err := b.build(ctx, name, e)
	if err != nil {
		return err
	}
	if err := b.stopEngines(); err != nil {
		return err
	}
	if err := b.mgr.EnableTCPTimestamps(); err != nil {
		return mapErr(err)
	}
	r, err := b.runner.Run(bd)
	if err != nil {
		return backend.Msg("EngineStartFailText", filepath.Base(bd.Exe), err.Error())
	}
	b.mu.Lock()
	b.run, b.runName, b.runEngine = r, name, e
	b.mu.Unlock()
	return b.keepSelection(name, e)
}

// Install implements backend.Backend. service.Manager.Install stops the
// engine processes and turns on the TCP timestamps.
func (b *Backend) Install(ctx context.Context, name string, e spec.Engine) error {
	b.op.Lock()
	defer b.op.Unlock()
	if err := b.install(ctx, name, e); err != nil {
		return err
	}
	return b.keepSelection(name, e)
}

func (b *Backend) install(ctx context.Context, name string, e spec.Engine) error {
	if err := b.elevated(); err != nil {
		return err
	}
	bd, err := b.build(ctx, name, e)
	if err != nil {
		return err
	}
	if err := b.stopRun(); err != nil {
		return err
	}
	return mapErrKey(b.mgr.Install(service.PlanFor(bd)), "ServiceInstallFail")
}

// SuspendService implements backend.Backend. It keeps an exact copy of the
// zapret service (also of a service of an other program) and then removes
// the service and the WinDivert drivers.
func (b *Backend) SuspendService(context.Context) (backend.ServiceSnapshot, error) {
	b.op.Lock()
	defer b.op.Unlock()
	return b.suspend()
}

// suspend is SuspendService. The caller holds b.op.
func (b *Backend) suspend() (backend.ServiceSnapshot, error) {
	if err := b.elevated(); err != nil {
		return backend.ServiceSnapshot{}, err
	}
	s, err := b.mgr.Snapshot()
	if err != nil || !s.Installed {
		return backend.ServiceSnapshot{}, err
	}
	snap := backend.ServiceSnapshot{
		Installed: true,
		Strategy:  s.Record.Strategy,
		Engine:    service.EngineOf(s.Record.ImagePath),
		Running:   s.Running,
		Data:      s,
	}
	if err := b.stopRun(); err != nil {
		return backend.ServiceSnapshot{}, err
	}
	if err := b.mgr.Remove(); err != nil {
		return backend.ServiceSnapshot{}, mapErr(err)
	}
	return snap, nil
}

// RestoreService implements backend.Backend. It makes the service of the
// snapshot again with the same ImagePath, configuration and registry
// values, and starts it when it ran. A snapshot that is not from
// SuspendService of this package gives an error.
func (b *Backend) RestoreService(_ context.Context, snap backend.ServiceSnapshot) error {
	if !snap.Installed {
		return nil
	}
	s, ok := snap.Data.(service.Snapshot)
	if !ok {
		return errors.New("the service snapshot has no data of the zapret service")
	}
	b.op.Lock()
	defer b.op.Unlock()
	if err := b.elevated(); err != nil {
		return err
	}
	if err := b.stopRun(); err != nil {
		return err
	}
	return b.restore(&s)
}

// restore makes the service of s again. It does not stop the process of
// Run. The caller holds b.op.
func (b *Backend) restore(s *service.Snapshot) error {
	return mapErrKey(b.mgr.Restore(s), "ServiceRestoreFail")
}
