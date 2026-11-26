package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/NONERUN/zapman/internal/engine"
	"github.com/NONERUN/zapman/internal/gui/backend"
	"github.com/NONERUN/zapman/internal/spec"
)

// fakesDir is the folder of the stock fake files in the catalog.
const fakesDir = "fakes"

// slotNames are the active fake names of the slots.
var slotNames = map[backend.FakeSlot]string{
	backend.FakeDiscord: "DISCORD_UDP",
	backend.FakeGame:    "GAME_UDP",
}

// activePath gives user\fakes\ACTIVE_<name>.bin.
func (b *Backend) activePath(name string) string {
	return filepath.Join(b.l.User, spec.ActiveFakesDir, "ACTIVE_"+name+".bin")
}

// fakeFiles gives the names of the .bin files of strategies\fakes, sorted.
func (b *Backend) fakeFiles() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(b.l.Strategies, fakesDir))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.Type().IsRegular() && strings.EqualFold(filepath.Ext(e.Name()), ".bin") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// Fakes implements backend.Backend, as Get-ZapretFakeCatalog does: an
// active fake is "matched" when its SHA256 is the SHA256 of a file of the
// list (the first one), else "unlisted".
func (b *Backend) Fakes(context.Context) (backend.FakeCatalog, error) {
	var c backend.FakeCatalog
	files, err := b.fakeFiles()
	if err != nil {
		return c, err
	}
	c.Files = files
	hashes := make([]string, len(files))
	for i, f := range files {
		if hashes[i], err = engine.FileSHA256(filepath.Join(b.l.Strategies, fakesDir, f)); err != nil {
			return c, err
		}
	}
	for _, slot := range []backend.FakeSlot{backend.FakeDiscord, backend.FakeGame} {
		name, state, err := b.activeFake(slotNames[slot], files, hashes)
		if err != nil {
			return c, err
		}
		if slot == backend.FakeDiscord {
			c.Discord, c.DiscordState = name, state
		} else {
			c.Game, c.GameState = name, state
		}
	}
	return c, nil
}

func (b *Backend) activeFake(name string, files, hashes []string) (string, backend.FakeState, error) {
	p := b.activePath(name)
	ok, err := fileExists(p)
	if err != nil || !ok {
		return "", backend.FakeMissing, err
	}
	h, err := engine.FileSHA256(p)
	if err != nil {
		return "", backend.FakeMissing, err
	}
	for i, fh := range hashes {
		if fh == h {
			return files[i], backend.FakeMatched, nil
		}
	}
	return "", backend.FakeUnlisted, nil
}

// SetFake implements backend.Backend. It copies the file of
// strategies\fakes to user\fakes\ACTIVE_<name>.bin. The new active fake
// applies at the next start of the engine.
func (b *Backend) SetFake(_ context.Context, slot backend.FakeSlot, file string) error {
	name, ok := slotNames[slot]
	if !ok {
		return fmt.Errorf("unknown fake slot %q", slot)
	}
	files, err := b.fakeFiles()
	if err != nil {
		return err
	}
	found := false
	for _, f := range files {
		found = found || f == file
	}
	if !found {
		return fmt.Errorf("fake file %q is not in %s", file, filepath.Join(b.l.Strategies, fakesDir))
	}
	data, err := os.ReadFile(filepath.Join(b.l.Strategies, fakesDir, file))
	if err != nil {
		return err
	}
	dst := b.activePath(name)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".fake-*.tmp")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}
