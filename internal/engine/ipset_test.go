package engine

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const testList = "1.2.3.0/24\r\n5.6.7.8/32\r\n"

func ipsetLayout(t *testing.T, list *string) Layout {
	t.Helper()
	l := NewLayout(t.TempDir())
	if list != nil {
		writeFile(t, l.IPSetPath(), *list)
	}
	return l
}

func wantMode(t *testing.T, l Layout, want IPSetMode) {
	t.Helper()
	got, err := l.IPSetMode()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("mode = %q, want %q", got, want)
	}
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func TestIPSetModeOfFile(t *testing.T) {
	s := func(v string) *string { return &v }
	for _, c := range []struct {
		name string
		list *string
		want IPSetMode
	}{
		{"missing", nil, IPSetNone},
		{"empty", s(""), IPSetAny},
		{"spaces", s(" \r\n"), IPSetAny},
		{"bom", s("\xef\xbb\xbf"), IPSetAny},
		{"unused address", s(newIpset), IPSetNone},
		{"unused address without newline", s("203.0.113.113/32"), IPSetNone},
		{"list", s(testList), IPSetLoaded},
	} {
		t.Run(c.name, func(t *testing.T) {
			wantMode(t, ipsetLayout(t, c.list), c.want)
		})
	}
}

func TestSetIPSetModeCycle(t *testing.T) {
	list := testList
	l := ipsetLayout(t, &list)
	bak := filepath.Join(l.User, IPSetBackup)

	if err := l.SetIPSetMode(IPSetAny, IPSetBackup); err != nil {
		t.Fatal(err)
	}
	wantMode(t, l, IPSetAny)
	if got := readFile(t, bak); got != testList {
		t.Fatalf("backup = %q", got)
	}

	// any -> none keeps the backup.
	if err := l.SetIPSetMode(IPSetNone, IPSetBackup); err != nil {
		t.Fatal(err)
	}
	wantMode(t, l, IPSetNone)
	if got := readFile(t, l.IPSetPath()); got != newIpset {
		t.Fatalf("list = %q", got)
	}

	if err := l.SetIPSetMode(IPSetLoaded, IPSetBackup); err != nil {
		t.Fatal(err)
	}
	wantMode(t, l, IPSetLoaded)
	if got := readFile(t, l.IPSetPath()); got != testList {
		t.Fatalf("list = %q", got)
	}
	if exists(bak) {
		t.Fatal("the backup is still there")
	}
}

func TestSetIPSetModeSameDoesNothing(t *testing.T) {
	list := testList
	l := ipsetLayout(t, &list)
	if err := l.SetIPSetMode(IPSetLoaded, IPSetBackup); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(l.User, IPSetBackup)) {
		t.Fatal("a backup is made")
	}
	wantMode(t, l, IPSetLoaded)
}

func TestSetIPSetModeLoadedNeedsBackup(t *testing.T) {
	l := ipsetLayout(t, nil)
	err := l.SetIPSetMode(IPSetLoaded, IPSetBackup)
	if !errors.Is(err, ErrNoIPSetBackup) {
		t.Fatalf("err = %v, want ErrNoIPSetBackup", err)
	}
	wantMode(t, l, IPSetNone)
}

func TestSetIPSetModeReplacesOldBackup(t *testing.T) {
	list := testList
	l := ipsetLayout(t, &list)
	bak := filepath.Join(l.User, IPSetBackup)
	writeFile(t, bak, "9.9.9.9/32\r\n")
	if err := l.SetIPSetMode(IPSetNone, IPSetBackup); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, bak); got != testList {
		t.Fatalf("backup = %q", got)
	}
}

func TestCopyIPSetModeAndRestore(t *testing.T) {
	const name = "ipset-all.test-backup.txt"
	list := testList
	l := ipsetLayout(t, &list)
	if err := l.CopyIPSetMode(IPSetAny, name); err != nil {
		t.Fatal(err)
	}
	wantMode(t, l, IPSetAny)
	if err := l.RestoreIPSet(name); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, l.IPSetPath()); got != testList {
		t.Fatalf("list = %q", got)
	}
	if exists(filepath.Join(l.User, name)) {
		t.Fatal("the backup is still there")
	}
	// A restore without a backup does nothing.
	if err := l.RestoreIPSet(name); err != nil {
		t.Fatal(err)
	}
}

func TestCopyIPSetModeMissingList(t *testing.T) {
	const name = "ipset-all.test-backup.txt"
	l := ipsetLayout(t, nil)
	if err := l.CopyIPSetMode(IPSetNone, name); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(l.User, name)); got != "" {
		t.Fatalf("backup = %q, want empty", got)
	}
	wantMode(t, l, IPSetNone)
}

func TestIPSetBadInput(t *testing.T) {
	l := ipsetLayout(t, nil)
	if err := l.SetIPSetMode("all", IPSetBackup); err == nil {
		t.Fatal("no error for a bad mode")
	}
	for _, name := range []string{"", `..\x.txt`, `a\b.txt`, IPSetFile, "IPSET-ALL.TXT"} {
		if err := l.SetIPSetMode(IPSetAny, name); err == nil {
			t.Fatalf("no error for the backup name %q", name)
		}
	}
}
