package gui

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"

	"github.com/NONERUN/zapman/internal/gui/backend"
)

// isCancel tells if err is a cancel of the context.
func isCancel(err error) bool {
	return errors.Is(err, context.Canceled)
}

// downloadState is the state of the download window.
type downloadState struct {
	cancel  context.CancelFunc
	running bool
	view    DownloadView
}

// DownloadView is the state of the download window.
type DownloadView struct {
	Status string
	// Percent is the progress, or -1 when the size is not known.
	Percent int
}

// download opens the download window, runs fn and closes the window, as
// Invoke-GuiDownload does. The Cancel button and the close of the window
// cancel the context of fn.
func (a *App) download(ctx context.Context, titleKey string, fn func(context.Context, backend.Progress) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	a.mu.Lock()
	a.dl = downloadState{cancel: cancel, running: true, view: DownloadView{Status: a.tLocked("DownloadConnecting"), Percent: -1}}
	a.mu.Unlock()
	d := a.openDialog(WinDownload, titleKey)
	err := fn(ctx, func(done, total int64) {
		a.mu.Lock()
		if total > 0 {
			pct := int(min(max(done*100/total, 0), 100))
			a.dl.view = DownloadView{Status: a.tLocked("DownloadProgress", done/1024, total/1024, pct), Percent: pct}
		} else {
			a.dl.view = DownloadView{Status: a.tLocked("DownloadProgressKb", done/1024), Percent: -1}
		}
		a.mu.Unlock()
		a.emit(EvDownload)
	})
	a.mu.Lock()
	a.dl.running = false
	a.mu.Unlock()
	if d != nil {
		a.closeDialog(WinDownload, nil)
	}
	return err
}

// downloadCancel cancels the download when it runs. It gives false when it
// does not run.
func (a *App) downloadCancel() bool {
	a.mu.Lock()
	run, cancel := a.dl.running, a.dl.cancel
	a.mu.Unlock()
	if run {
		cancel()
	}
	return run
}

// DownloadView gives the state of the download window.
func (a *App) DownloadView() DownloadView {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.dl.view
}

// DownloadCancel cancels the download.
func (a *App) DownloadCancel() { a.downloadCancel() }

// fakesDialog is the state of the Fakes dialog.
type fakesDialog struct {
	discord fakeSlot
	game    fakeSlot
}

// fakeChange is a change of the Fakes dialog.
type fakeChange struct {
	slot backend.FakeSlot
	file string
}

// FakesView is the state of the Fakes dialog.
type FakesView struct {
	Discord         []string
	DiscordSelected int
	Game            []string
	GameSelected    int
}

// ClickFakes opens the Fakes dialog and applies the changes.
func (a *App) ClickFakes() { a.goFlow(a.fakesFlow) }

func (a *App) fakesFlow() {
	cat, err := a.be.Fakes(a.ctx)
	if err != nil {
		a.shell.Error(a.errText(err))
		return
	}
	if len(cat.Files) == 0 {
		a.shell.Error(a.t("FakesNone"))
		return
	}
	a.mu.Lock()
	a.fakes = fakesDialog{
		discord: newFakeSlot(a.lang, cat.Files, cat.Discord, cat.DiscordState),
		game:    newFakeSlot(a.lang, cat.Files, cat.Game, cat.GameState),
	}
	a.mu.Unlock()
	changes, _ := a.showDialog(WinFake, "FakesTitle").([]fakeChange)
	if len(changes) == 0 {
		return
	}
	for _, c := range changes {
		if err := a.be.SetFake(a.ctx, c.slot, c.file); err != nil {
			a.fail(err)
			return
		}
		a.logLine(a.t("FakeDone", string(c.slot), c.file))
	}
	if !a.hasService() {
		a.shell.Info(a.t("FakeNeedRerun"))
		return
	}
	if a.shell.Ask(a.t("FakeNeedRestart"), true) {
		a.restartService()
	}
}

// FakesView gives the state of the Fakes dialog.
func (a *App) FakesView() FakesView {
	a.mu.Lock()
	defer a.mu.Unlock()
	f := a.fakes
	return FakesView{
		Discord: slices.Clone(f.discord.Items), DiscordSelected: f.discord.Selected,
		Game: slices.Clone(f.game.Items), GameSelected: f.game.Selected,
	}
}

// FakesApply closes the Fakes dialog with the selected items.
func (a *App) FakesApply(discord, game int) {
	a.mu.Lock()
	var ch []fakeChange
	if f := a.fakes.discord.picked(discord); f != "" {
		ch = append(ch, fakeChange{backend.FakeDiscord, f})
	}
	if f := a.fakes.game.picked(game); f != "" {
		ch = append(ch, fakeChange{backend.FakeGame, f})
	}
	a.mu.Unlock()
	a.closeDialog(WinFake, ch)
}

// GameItem is a game module of the Games dialog.
type GameItem struct {
	ID    string
	Title string
	// About is the text about the game in the UI language, or in the
	// other language when the UI language has no text. It can be empty.
	About   string
	Checked bool
}

// ClickGames opens the Games dialog. The PS GUI has no game modules.
func (a *App) ClickGames() {
	a.goFlow(func() {
		a.mu.Lock()
		n := len(a.games)
		a.mu.Unlock()
		if n == 0 {
			a.shell.Info(a.t("GamesNone"))
			return
		}
		ids, ok := a.showDialog(WinGames, "GamesTitle").([]string)
		if !ok {
			return
		}
		if err := a.saveSettings(func(s *backend.Settings) { s.Games = ids }); err != nil {
			a.fail(err)
			return
		}
		a.afterGameChange("GamesHintInstall", "GamesHintRerun", "GamesNeedInstall")
	})
}

// GamesView gives the game modules of the catalog with the selected ones.
func (a *App) GamesView() []GameItem {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]GameItem, 0, len(a.games))
	for _, g := range a.games {
		out = append(out, GameItem{ID: g.ID, Title: g.Title, About: pickAbout(g.About, a.lang), Checked: slices.Contains(a.settings.Games, g.ID)})
	}
	return out
}

// pickAbout gives the text of about in the language lang. When lang has no
// text, it gives the en text, then the text of the first language in the
// sort order.
func pickAbout(about map[string]string, lang string) string {
	if s := about[lang]; s != "" {
		return s
	}
	if s := about["en"]; s != "" {
		return s
	}
	for _, k := range slices.Sorted(maps.Keys(about)) {
		if about[k] != "" {
			return about[k]
		}
	}
	return ""
}

// GamesApply closes the Games dialog with the checked game IDs.
func (a *App) GamesApply(ids []string) {
	a.mu.Lock()
	keep := []string{}
	for _, g := range a.games {
		if slices.Contains(ids, g.ID) {
			keep = append(keep, g.ID)
		}
	}
	a.mu.Unlock()
	a.closeDialog(WinGames, keep)
}

// diagDialog is the state of the Diagnostics dialog.
type diagDialog struct {
	items []backend.DiagItem
	busy  bool
}

// DiagRow is a row of the Diagnostics dialog.
type DiagRow struct {
	Text string
	// Status is "ok", "warn" or "fail".
	Status    string
	HasAction bool
}

// DiagView is the state of the Diagnostics dialog.
type DiagView struct {
	Rows []DiagRow
	Busy bool
}

// ClickDiag opens the Diagnostics dialog.
func (a *App) ClickDiag() {
	a.goFlow(func() {
		a.action("BtnDiag", func(ctx context.Context) error {
			items, err := a.be.Diagnostics(ctx)
			if err != nil {
				return err
			}
			a.mu.Lock()
			a.diag = diagDialog{items: items}
			a.mu.Unlock()
			a.showDialog(WinDiag, "DiagTitle")
			a.logLine(a.t("DiagDone"))
			return nil
		})
	})
}

// DiagView gives the state of the Diagnostics dialog.
func (a *App) DiagView() DiagView {
	a.mu.Lock()
	defer a.mu.Unlock()
	v := DiagView{Busy: a.diag.busy}
	for _, it := range a.diag.items {
		v.Rows = append(v.Rows, DiagRow{Text: it.Text, Status: string(it.Status), HasAction: strings.TrimSpace(it.Action) != ""})
	}
	return v
}

// DiagRun does the action of the row and builds the list again. It does
// not ask, as in the PS GUI.
func (a *App) DiagRun(index int) {
	a.mu.Lock()
	if a.diag.busy || index < 0 || index >= len(a.diag.items) || a.diag.items[index].Action == "" {
		a.mu.Unlock()
		return
	}
	a.diag.busy = true
	item := a.diag.items[index]
	a.mu.Unlock()
	a.emit(EvDiag)
	a.goTask(func() {
		defer func() {
			a.mu.Lock()
			a.diag.busy = false
			a.mu.Unlock()
			a.emit(EvDiag)
		}()
		if err := a.be.RunDiagAction(a.ctx, item); err != nil {
			a.shell.Error(a.errText(err))
		}
		items, err := a.be.Diagnostics(a.ctx)
		if err != nil {
			a.shell.Error(a.errText(err))
			return
		}
		a.mu.Lock()
		a.diag.items = items
		a.mu.Unlock()
	})
}
