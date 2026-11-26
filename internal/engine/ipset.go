package engine

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// IPSetFile is the ipset of the IPSet Filter in the user folder.
const IPSetFile = "ipset-all.txt"

// IPSetBackup is the default name of the copy of the loaded ipset, as in
// the PowerShell version.
const IPSetBackup = "ipset-all.txt.backup"

// IPSetMode is the IPSet Filter mode. It is not a setting: it is the
// state of the content of user\ipset-all.txt.
type IPSetMode string

// The IPSet Filter modes. The values are the values of the PowerShell
// version.
const (
	// IPSetNone: the file is missing, or it has only the address that is
	// not used (newIpset). No address matches.
	IPSetNone IPSetMode = "none"
	// IPSetLoaded: the file has a list of addresses.
	IPSetLoaded IPSetMode = "loaded"
	// IPSetAny: the file is empty. All addresses match.
	IPSetAny IPSetMode = "any"
)

// ErrNoIPSetBackup tells that the mode "loaded" has no copy of the list to
// put back.
var ErrNoIPSetBackup = errors.New("no IPSet backup found. Download the ipset list first")

// IPSetPath gives the path of user\ipset-all.txt.
func (l Layout) IPSetPath() string { return filepath.Join(l.User, IPSetFile) }

// IPSetMode gives the mode of user\ipset-all.txt, as
// Get-ZapretIpsetStatus does.
func (l Layout) IPSetMode() (IPSetMode, error) {
	b, err := os.ReadFile(l.IPSetPath())
	if errors.Is(err, fs.ErrNotExist) {
		return IPSetNone, nil
	}
	if err != nil {
		return "", err
	}
	s := strings.TrimSpace(string(bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))))
	switch s {
	case "":
		return IPSetAny, nil
	case strings.TrimSpace(newIpset):
		return IPSetNone, nil
	}
	return IPSetLoaded, nil
}

// backupPath gives the path of the backup name in the user folder.
func (l Layout) backupPath(name string) (string, error) {
	p := filepath.Join(l.User, name)
	if name == "" || filepath.Base(name) != name || !isInside(l.User, p) || strings.EqualFold(name, IPSetFile) {
		return "", fmt.Errorf("%q is not a name for the ipset backup", name)
	}
	return p, nil
}

// SetIPSetMode changes user\ipset-all.txt to the mode, as
// Set-ZapretIpsetMode does. The loaded list goes to the backup file name in
// the user folder (IPSetBackup for the IPSet Filter); the mode "loaded"
// moves it back. When the file has the mode already, nothing changes.
func (l Layout) SetIPSetMode(mode IPSetMode, backup string) error {
	if err := checkIPSetMode(mode); err != nil {
		return err
	}
	bak, err := l.backupPath(backup)
	if err != nil {
		return err
	}
	cur, err := l.IPSetMode()
	if err != nil {
		return err
	}
	if cur == mode {
		return nil
	}
	list := l.IPSetPath()
	if cur == IPSetLoaded {
		if err := os.Remove(bak); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err := os.Rename(list, bak); err != nil {
			return err
		}
	}
	switch mode {
	case IPSetNone:
		return l.writeIPSet(newIpset)
	case IPSetAny:
		return l.writeIPSet("")
	}
	if _, err := os.Stat(bak); errors.Is(err, fs.ErrNotExist) {
		return ErrNoIPSetBackup
	} else if err != nil {
		return err
	}
	if err := os.Remove(list); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.Rename(bak, list)
}

// CopyIPSetMode copies user\ipset-all.txt to the backup file name (an
// empty backup when the file is missing) and then sets the mode, as
// Set-ZapretIpsetMode -CopyBackup does. The mode "loaded" keeps the file;
// it needs a backup. RestoreIPSet puts the copy back. The strategy tests
// use it.
func (l Layout) CopyIPSetMode(mode IPSetMode, backup string) error {
	if err := checkIPSetMode(mode); err != nil {
		return err
	}
	bak, err := l.backupPath(backup)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(l.IPSetPath())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(l.User, 0o755); err != nil {
		return err
	}
	if err := writeIPSetFile(bak, b); err != nil {
		return err
	}
	switch mode {
	case IPSetNone:
		return l.writeIPSet(newIpset)
	case IPSetAny:
		return l.writeIPSet("")
	}
	return nil
}

// RestoreIPSet moves the backup file name back to user\ipset-all.txt. A
// missing backup is not an error.
func (l Layout) RestoreIPSet(backup string) error {
	bak, err := l.backupPath(backup)
	if err != nil {
		return err
	}
	if _, err := os.Stat(bak); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	list := l.IPSetPath()
	if err := os.Remove(list); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.Rename(bak, list)
}

func checkIPSetMode(mode IPSetMode) error {
	switch mode {
	case IPSetNone, IPSetLoaded, IPSetAny:
		return nil
	}
	return fmt.Errorf("%q is not an IPSet Filter mode. Use none, loaded or any", mode)
}

func (l Layout) writeIPSet(s string) error {
	if err := os.MkdirAll(l.User, 0o755); err != nil {
		return err
	}
	return writeIPSetFile(l.IPSetPath(), []byte(s))
}

func writeIPSetFile(p string, b []byte) error {
	return os.WriteFile(p, b, 0o600)
}
