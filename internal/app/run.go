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
// lists. When the file is not correct, the defaults of the fields that are
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
	off := spec.GameFilter{TCP: config.OffPort, UDP: config.OffPort}
	if _, err := spec.Build(st, nil, e, spec.Paths{}, off); err != nil {
		return nil, backend.Msg("EngineNoFlags", string(e))
	}
	b.cfg.Lock()
	c, _ := b.loadConfig()
	b.cfg.Unlock()
	c.Strategy = name
	c.Engine = e
	bd, err := b.l.Build(cat, &c)
	if err != nil {
		return nil, err
	}
	if err := b.l.EnsureUserFiles(cat, bd.Games); err != nil {
		return nil, fmt.Errorf("make the user lists: %w", err)
	}
	return bd, nil
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
	b.run, b.runName = r, name
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
	return mapErr(b.mgr.Install(service.PlanFor(bd)))
}

// SuspendService implements backend.Backend. It removes the zapret service
// and the WinDivert drivers. A service of an other program (no strategy in
// its record) gives a snapshot with no strategy: RestoreService cannot make
// it again.
func (b *Backend) SuspendService(context.Context) (backend.ServiceSnapshot, error) {
	b.op.Lock()
	defer b.op.Unlock()
	if err := b.elevated(); err != nil {
		return backend.ServiceSnapshot{}, err
	}
	st, err := b.mgr.Status(nil)
	if err != nil || !st.Installed {
		return backend.ServiceSnapshot{}, err
	}
	snap := backend.ServiceSnapshot{
		Strategy: st.Record.Strategy,
		Engine:   st.Engine,
		Running:  st.State == service.Running || st.State == service.StartPending,
	}
	if err := b.stopRun(); err != nil {
		return backend.ServiceSnapshot{}, err
	}
	if err := b.mgr.Remove(); err != nil {
		return backend.ServiceSnapshot{}, mapErr(err)
	}
	return snap, nil
}

// RestoreService implements backend.Backend. It installs the strategy of
// the snapshot with the settings of now. When the service did not run, it
// stops it after the install.
func (b *Backend) RestoreService(ctx context.Context, snap backend.ServiceSnapshot) error {
	if snap.Strategy == "" {
		return nil
	}
	e := snap.Engine
	if e == "" {
		e = spec.Winws
	}
	b.op.Lock()
	defer b.op.Unlock()
	if err := b.install(ctx, snap.Strategy, e); err != nil {
		return err
	}
	if snap.Running {
		return nil
	}
	if err := b.mgr.Stop(); err != nil && !errors.Is(err, service.ErrNotInstalled) {
		return mapErr(err)
	}
	return nil
}
