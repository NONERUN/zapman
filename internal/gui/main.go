package gui

import (
	"context"
	"slices"

	"github.com/NONERUN/zapman/internal/gui/backend"
	"github.com/NONERUN/zapman/internal/gui/i18n"
)

// MainView is the state of the main window.
type MainView struct {
	Title   string
	Lang    string
	Status  StatusLabels
	Busy    bool
	HasSvc  bool
	HasStrt bool
	// GameFilter and IPSet are the indexes of the combo boxes.
	GameFilter int
	IPSet      int
	AutoUpdate bool
	TrayWatch  bool
}

// Main gives the state of the main window.
func (a *App) Main() MainView {
	a.mu.Lock()
	defer a.mu.Unlock()
	v := MainView{
		Title:      i18n.T(a.lang, "AppTitle", a.be.Version().App),
		Lang:       a.lang,
		Status:     statusLabels(a.lang, a.status, len(a.strategies) > 0, a.lastError),
		Busy:       a.busy,
		HasSvc:     a.status.HasService(),
		HasStrt:    len(a.strategies) > 0,
		GameFilter: max(0, slices.Index(backend.GameFilterModes, a.settings.GameFilter)),
		IPSet:      max(0, slices.Index(backend.IPSetModes, a.ipset)),
		AutoUpdate: a.settings.AutoUpdateCheck,
		TrayWatch:  a.settings.TrayWatch,
	}
	return v
}

// SetLanguage sets the UI language and saves it.
func (a *App) SetLanguage(lang string) {
	if !i18n.Valid(lang) {
		return
	}
	a.goTask(func() {
		if err := a.saveSettings(func(s *backend.Settings) { s.Language = lang }); err != nil {
			a.fail(err)
			return
		}
		a.mu.Lock()
		a.lang = lang
		a.mu.Unlock()
		a.shell.SetTitle(WinMain, a.t("AppTitle", a.be.Version().App))
		a.emit(EvLang)
		a.refresh()
	})
}

// ClickStart starts the installed service.
func (a *App) ClickStart() { a.goFlow(a.startService) }

func (a *App) startService() {
	a.action("BtnStart", func(ctx context.Context) error {
		if err := a.be.StartService(ctx); err != nil {
			return err
		}
		a.clearLastError()
		st := a.statusNow()
		if st.Installed != "" {
			a.logLine(a.t("StartDoneName", st.Installed))
		} else {
			a.logLine(a.t("StartDone"))
		}
		return nil
	})
}

// ClickStop stops the bypass and the service.
func (a *App) ClickStop() { a.goFlow(a.stopBypass) }

func (a *App) stopBypass() {
	a.action("BtnStop", func(ctx context.Context) error {
		if err := a.be.Stop(ctx); err != nil {
			return err
		}
		a.logLine(a.t("StopDone"))
		return nil
	})
}

// ClickRemove removes the services. It does not ask, as in the PS GUI.
func (a *App) ClickRemove() {
	a.goFlow(func() {
		a.action("BtnRemove", func(ctx context.Context) error {
			if err := a.be.RemoveServices(ctx); err != nil {
				return err
			}
			a.logLine(a.t("RemoveDone"))
			return nil
		})
	})
}

// restartService stops the bypass and starts the service again, after a
// change of a filter or a fake.
func (a *App) restartService() {
	a.action("BtnStop", func(ctx context.Context) error {
		if err := a.be.Stop(ctx); err != nil {
			return err
		}
		if err := a.be.StartService(ctx); err != nil {
			return err
		}
		a.clearLastError()
		a.logLine(a.t("StartDone"))
		return nil
	})
}

// ClickStatus opens the Status dialog.
func (a *App) ClickStatus() {
	a.mu.Lock()
	busy := a.busy
	a.mu.Unlock()
	if busy {
		return
	}
	a.goFlow(func() { a.showDialog(WinStatus, "StatusDlgTitle") })
}

// StatusText gives the text of the Status dialog.
func (a *App) StatusText() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	lines := statusLines(a.lang, a.be.Version(), a.status, a.configErr, a.lastError)
	text := ""
	for _, l := range lines {
		text += l + "\n"
	}
	return text + "\n" + i18n.T(a.lang, "JournalHeader") + "\n" + a.journal.text()
}

// SetGameFilter saves the Game Filter mode of the combo box index.
func (a *App) SetGameFilter(idx int) {
	if idx < 0 || idx >= len(backend.GameFilterModes) {
		return
	}
	a.goFlow(func() {
		err := a.saveSettings(func(s *backend.Settings) { s.GameFilter = backend.GameFilterModes[idx] })
		if err != nil {
			a.fail(err)
			return
		}
		a.afterGameChange("GameHintInstall", "GameHintRerun", "FilterNeedInstall")
	})
}

// afterGameChange tells how to apply a change of the game rules. With a
// service, it can open the Strategy dialog to install the service again.
func (a *App) afterGameChange(hintInstall, hintRerun, askKey string) {
	svc := a.hasService()
	if svc {
		a.logLine(a.t(hintInstall))
	} else {
		a.logLine(a.t(hintRerun))
	}
	a.emit(EvMain)
	if !svc {
		a.shell.Info(a.t("FilterNeedRerun"))
		return
	}
	if a.shell.Ask(a.t(askKey), true) {
		a.strategyFlow()
	}
}

// SetIPSet changes user/ipset-all.txt to the IPSet Filter mode of the
// combo box index.
func (a *App) SetIPSet(idx int) {
	if idx < 0 || idx >= len(backend.IPSetModes) {
		return
	}
	mode := backend.IPSetModes[idx]
	a.goFlow(func() {
		if err := a.be.SetIPSet(a.ctx, mode); err != nil {
			a.fail(err)
			return
		}
		a.mu.Lock()
		a.ipset = mode
		a.mu.Unlock()
		svc := a.hasService()
		if svc {
			a.logLine(a.t("IpsetHintRestart"))
		} else {
			a.logLine(a.t("IpsetHintRerun"))
		}
		a.emit(EvMain)
		if !svc {
			a.shell.Info(a.t("FilterNeedRerun"))
			return
		}
		if a.shell.Ask(a.t("FilterNeedRestart"), true) {
			a.restartService()
		}
	})
}

// SetAutoUpdate saves the Auto-Update Check box.
func (a *App) SetAutoUpdate(on bool) {
	a.goTask(func() {
		if err := a.saveSettings(func(s *backend.Settings) { s.AutoUpdateCheck = on }); err != nil {
			a.fail(err)
			return
		}
		if on {
			a.logLine(a.t("AutoOn"))
		} else {
			a.logLine(a.t("AutoOff"))
		}
		a.emit(EvMain)
	})
}

// SetTrayWatch makes or removes the logon task of the tray, saves the Tray
// icon box and shows or removes the icon. As Set-ZapmanTrayWatchEnabled,
// it changes the task first; when the task fails, the box does not change.
func (a *App) SetTrayWatch(on bool) {
	a.goTask(func() {
		if err := a.be.SetTrayAtLogon(a.ctx, on); err != nil {
			a.fail(backend.Msg("TrayTaskFail", a.errText(err)))
			a.emit(EvMain)
			return
		}
		if err := a.saveSettings(func(s *backend.Settings) { s.TrayWatch = on }); err != nil {
			a.fail(err)
			return
		}
		if on {
			a.logLine(a.t("TrayOn"))
		} else {
			a.logLine(a.t("TrayOff"))
		}
		a.refresh()
	})
}

// ClickIPSet downloads the ipset list.
func (a *App) ClickIPSet() {
	a.goFlow(func() {
		a.action("BtnIpset", func(ctx context.Context) error {
			err := a.download(ctx, "IpsetTitle", func(ctx context.Context, p backend.Progress) error {
				return a.be.DownloadIPSet(ctx, p)
			})
			if isCancel(err) {
				a.logLine(a.t("IpsetCancel"))
				return nil
			}
			if err != nil {
				return err
			}
			a.logLine(a.t("IpsetOk"))
			return nil
		})
	})
}

// ClickHosts compares the hosts file with the template.
func (a *App) ClickHosts() {
	a.goFlow(func() {
		a.action("BtnHosts", func(ctx context.Context) error {
			var hc backend.HostsCheck
			err := a.download(ctx, "HostsTitle", func(ctx context.Context, p backend.Progress) error {
				var err error
				hc, err = a.be.CheckHosts(ctx, p)
				return err
			})
			if isCancel(err) {
				a.logLine(a.t("HostsCancel"))
				return nil
			}
			if err != nil {
				return backend.Msg("HostsFail")
			}
			if !hc.NeedsUpdate {
				a.shell.Info(a.t("HostsOk"))
				a.logLine(a.t("HostsOk"))
				return nil
			}
			a.mu.Lock()
			a.hosts = hc
			a.mu.Unlock()
			a.showDialog(WinHosts, "HostsTitle")
			a.logLine(a.t("HostsNeed"))
			return nil
		})
	})
}

// HostsView gives the template of the Hosts dialog.
func (a *App) HostsView() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.hosts.Template
}

// HostsOpen opens the system hosts file.
func (a *App) HostsOpen() {
	a.goTask(func() {
		if err := a.be.OpenHosts(a.ctx); err != nil {
			a.shell.Error(a.errText(err))
		}
	})
}

// ClickVersion checks the latest version and tells the result.
func (a *App) ClickVersion() {
	a.mu.Lock()
	run := a.versionRun
	a.mu.Unlock()
	if run {
		return
	}
	a.logLine(a.t("VersionChecking"))
	a.goTask(func() { a.versionCheck(true) })
}

// versionCheck compares the build tag with the latest tag, as
// Complete-GuiVersionCheck does. With report false, it tells only about a
// new version.
func (a *App) versionCheck(report bool) {
	a.mu.Lock()
	if a.versionRun {
		a.mu.Unlock()
		return
	}
	a.versionRun = true
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.versionRun = false
		a.mu.Unlock()
	}()
	ctx, cancel := context.WithTimeout(a.ctx, a.opt.VersionTimeout)
	remote, err := a.be.LatestVersion(ctx)
	cancel()
	if a.ctx.Err() != nil {
		return
	}
	ver := a.be.Version()
	switch {
	case err != nil:
		if report {
			a.shell.Info(a.t("VersionFail"))
			a.logLine(a.t("VersionFail"))
		} else {
			a.logLine(a.t("AutoFail"))
		}
	case remote == ver.App:
		if report {
			a.shell.Info(a.t("VersionLatest", ver.App))
			a.logLine(a.t("VersionLatest", ver.App))
		}
	case remote == "":
		if report {
			a.shell.Info(a.t("VersionFail"))
			a.logLine(a.t("VersionFail"))
		}
	default:
		if a.shell.Ask(a.t("VersionNew", remote), true) {
			if err := a.shell.OpenURL(ver.ReleasesURL); err != nil {
				a.shell.Error(err.Error())
			}
		}
		a.logLine(a.t("VersionNew", remote))
	}
}

// Tray gives the state of the tray icon.
func (a *App) Tray() TrayView {
	a.mu.Lock()
	defer a.mu.Unlock()
	return trayView(a.lang, a.status)
}

// TrayOpen shows the main window.
func (a *App) TrayOpen() {
	a.shell.OpenWindow(WinMain, a.t("AppTitle", a.be.Version().App))
}

// TrayStart starts the service from the tray menu.
func (a *App) TrayStart() { a.goTask(a.startService) }

// TrayStop stops the bypass from the tray menu.
func (a *App) TrayStop() { a.goTask(a.stopBypass) }
