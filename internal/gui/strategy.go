package gui

import (
	"context"

	"github.com/NONERUN/zapman/internal/gui/backend"
	"github.com/NONERUN/zapman/internal/spec"
)

// stratDialog is the state of the Strategy dialog.
type stratDialog struct {
	// engineOK tells which engines have their files in bin.
	engineOK map[spec.Engine]bool
	// tests tells that the dialog closed with the Run tests button.
	tests bool
}

// StrategyItem is an item of the strategy list.
type StrategyItem struct {
	Name string
	// Label is the name with the mark of the running or installed strategy.
	Label string
	// Supported tells if the engine can build the strategy. An item that
	// is not supported is gray.
	Supported bool
}

// StrategyView is the state of the Strategy dialog.
type StrategyView struct {
	Engine string
	Items  []StrategyItem
	// Selected is the index of the running or installed strategy, else 0.
	Selected int
	// EngineOK tells if bin has the files of the engine. Run and Install
	// are on when EngineOK and the selected item is supported.
	EngineOK bool
}

// ClickStrategy opens the Strategy dialog.
func (a *App) ClickStrategy() { a.goFlow(a.strategyFlow) }

// strategyFlow shows the Strategy dialog, as Show-StrategyDialog does.
// After the Run tests button it starts the tests.
func (a *App) strategyFlow() {
	a.mu.Lock()
	n := len(a.strategies)
	a.mu.Unlock()
	if n == 0 {
		a.shell.Error(a.t("NoStrategies"))
		return
	}
	ok := map[spec.Engine]bool{}
	for _, e := range []spec.Engine{spec.Winws, spec.Winws2} {
		ok[e] = a.be.CheckEngine(a.ctx, e) == nil
	}
	a.mu.Lock()
	a.stratDlg = stratDialog{engineOK: ok}
	a.mu.Unlock()
	a.showDialog(WinStrategy, "StratTitle")
	a.refresh()
	a.mu.Lock()
	tests := a.stratDlg.tests
	a.stratDlg.tests = false
	a.mu.Unlock()
	if tests {
		a.testsFlow()
	}
}

// StrategyView gives the state of the Strategy dialog for the engine. An
// empty engine is the engine of the settings.
func (a *App) StrategyView(engine string) StrategyView {
	e := spec.Engine(engine)
	if e != spec.Winws && e != spec.Winws2 {
		e = a.engine()
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	v := StrategyView{Engine: string(e), EngineOK: a.stratDlg.engineOK[e]}
	st := a.status
	pick := ""
	if st.BypassRunning {
		pick = st.Running
	}
	if pick == "" {
		pick = st.Installed
	}
	for i, s := range a.strategies {
		it := StrategyItem{Name: s.Name, Label: s.Name, Supported: s.Supports(e)}
		switch {
		case st.BypassRunning && s.Name == st.Running:
			it.Label += a.tLocked("StratMarkRun")
		case s.Name == st.Installed:
			it.Label += a.tLocked("StratMarkService")
		}
		if pick != "" && s.Name == pick {
			v.Selected = i
		}
		v.Items = append(v.Items, it)
	}
	return v
}

// StrategyRun runs the strategy with no service.
func (a *App) StrategyRun(name, engine string) {
	e, ok := a.pickStrategy(name, engine)
	if !ok {
		return
	}
	a.goTask(func() {
		st := a.statusNow()
		if st.Live() && !a.shell.Ask(a.t("ConfirmRunOver"), false) {
			return
		}
		a.action("BtnRunSelected", func(ctx context.Context) error {
			if err := a.saveSettings(func(s *backend.Settings) { s.Engine = e }); err != nil {
				return err
			}
			if err := a.be.Run(ctx, name, e); err != nil {
				return err
			}
			a.reloadSettings()
			a.clearLastError()
			a.logLine(a.t("RunDone", name))
			return nil
		})
		a.emit(EvStrategy)
	})
}

// StrategyInstall installs the service with the strategy. On success the
// dialog closes.
func (a *App) StrategyInstall(name, engine string) {
	e, ok := a.pickStrategy(name, engine)
	if !ok {
		return
	}
	a.goTask(func() {
		done := a.action("BtnInstall", func(ctx context.Context) error {
			if err := a.saveSettings(func(s *backend.Settings) { s.Engine = e }); err != nil {
				return err
			}
			if err := a.be.Install(ctx, name, e); err != nil {
				return err
			}
			a.reloadSettings()
			a.clearLastError()
			a.logLine(a.t("InstallDone", name))
			return nil
		})
		if done {
			a.closeDialog(WinStrategy, nil)
		} else {
			a.emit(EvStrategy)
		}
	})
}

// pickStrategy checks the strategy name and the engine of the dialog. On a
// bad name it shows NoStrategies, as Get-SelectedStrategy does.
func (a *App) pickStrategy(name, engine string) (spec.Engine, bool) {
	e := spec.Engine(engine)
	a.mu.Lock()
	found := false
	for _, s := range a.strategies {
		found = found || s.Name == name
	}
	a.mu.Unlock()
	if !found || (e != spec.Winws && e != spec.Winws2) {
		a.goTask(func() { a.shell.Error(a.t("NoStrategies")) })
		return "", false
	}
	return e, true
}

// StrategyTests saves the engine and closes the dialog; then the tests
// start.
func (a *App) StrategyTests(engine string) {
	e := spec.Engine(engine)
	if e != spec.Winws && e != spec.Winws2 {
		return
	}
	a.goTask(func() {
		if err := a.saveSettings(func(s *backend.Settings) { s.Engine = e }); err != nil {
			a.shell.Error(a.errText(err))
			return
		}
		a.mu.Lock()
		a.stratDlg.tests = true
		a.mu.Unlock()
		a.closeDialog(WinStrategy, nil)
	})
}

// StrategyNone resets the network stack, as the None work? button does.
func (a *App) StrategyNone() {
	a.goTask(func() {
		if !a.shell.Ask(a.t("StratNoneConfirm"), false) {
			return
		}
		var text string
		var resetErr error
		ran := a.action("BtnStratNone", func(ctx context.Context) error {
			text, resetErr = a.be.NetworkReset(ctx)
			return nil
		})
		if !ran {
			return
		}
		if text != "" {
			a.logLine(text)
		}
		if resetErr != nil {
			if text == "" {
				text = a.errText(resetErr)
			}
			a.shell.Error(a.t("StratNoneFail") + "\n\n" + text)
			return
		}
		if a.shell.Ask(a.t("StratNoneDoneReboot"), false) {
			if err := a.be.Reboot(a.ctx); err != nil {
				a.shell.Error(a.errText(err))
			}
			return
		}
		a.shell.Info(a.t("StratNoneRebootLater"))
	})
}
