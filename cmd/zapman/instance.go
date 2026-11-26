package main

import (
	"os"
	"os/exec"
	"slices"
	"strconv"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/NONERUN/zapman/internal/gui/i18n"
)

// The single instance of zapman.
//
// Only one zapman process can run. A second start of the same exe file
// opens the main window of the first process. A second start of an other
// exe file (a new build at the same path, or an other zapman folder) takes
// over: the first process starts the new exe again with the flag
// -after-pid and quits. The new process waits until the first process
// ends, because the lock of Wails stays until then.
//
// The takeover does not change winws or the service: they are not parts of
// the process. The stale check of the new process tells about a service of
// an old catalog.

const (
	// instanceID is the UniqueID of the Wails single instance lock.
	instanceID = "com.github.nonerun.zapman"
	// dataExe is the key of the exe path in SecondInstanceData.AdditionalData.
	dataExe = "exe"
	// afterPIDFlag is the flag of a start by a takeover.
	afterPIDFlag = "-after-pid"
	// afterPIDTimeout is the time limit of the wait for the first process.
	// It is more than quitTimeout.
	afterPIDTimeout = quitTimeout + 5*time.Second
)

// instance is the identity of the exe of this process.
type instance struct {
	exe string
	id  string // exeID of exe at the start; empty when it is not known
}

// newInstance gives the identity of the exe of this process. Call it at
// the start, before a build can replace the file.
func newInstance() instance {
	exe, err := os.Executable()
	if err != nil {
		return instance{}
	}
	id, _ := exeID(exe)
	return instance{exe: exe, id: id}
}

// options gives the Wails options of the single instance lock.
// onSame is called when the second start is of the same exe file.
func (in instance) options(sh *shell, onSame func(application.SecondInstanceData)) *application.SingleInstanceOptions {
	return &application.SingleInstanceOptions{
		UniqueID:       instanceID,
		AdditionalData: map[string]string{dataExe: in.exe},
		OnSecondInstanceLaunch: func(d application.SecondInstanceData) {
			exe := d.AdditionalData[dataExe]
			if in.same(exe) {
				onSame(d)
				return
			}
			if err := startAfter(exe, d); err != nil {
				onSame(d)
				sh.Error(i18n.T(sh.ui.Lang(), "TakeoverFail", exe, err.Error()))
				return
			}
			sh.quit()
		},
	}
}

// same tells if exe is the exe file of this process. When an identity is
// not known, it tells true: then the first process stays, as before.
func (in instance) same(exe string) bool {
	if exe == "" || in.id == "" {
		return true
	}
	id, err := exeID(exe)
	return err != nil || id == in.id
}

// startAfter starts exe with the args of the second start and the flag
// -after-pid with the PID of this process.
func startAfter(exe string, d application.SecondInstanceData) error {
	var args []string
	if len(d.Args) > 1 {
		args = slices.Clone(d.Args[1:])
	}
	args = append(args, afterPIDFlag+"="+strconv.Itoa(os.Getpid()))
	cmd := exec.Command(exe, args...) //nolint:gosec // G204: exe is the exe of a second start of zapman.
	cmd.Dir = d.WorkingDir
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
