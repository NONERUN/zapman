package netreset

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/NONERUN/zapman/internal/tools/command"
)

type call struct {
	exe  string
	args []string
}

// fake gives the results in order and records the calls.
type fake struct {
	calls   []call
	results []command.Result
	errs    []error
	cancel  func()
}

func (f *fake) run(_ context.Context, exe string, args ...string) (command.Result, error) {
	i := len(f.calls)
	f.calls = append(f.calls, call{filepath.Base(exe), args})
	if f.cancel != nil && i == 1 {
		f.cancel()
		return command.Result{}, context.Canceled
	}
	var r command.Result
	var err error
	if i < len(f.results) {
		r = f.results[i]
	}
	if i < len(f.errs) {
		err = f.errs[i]
	}
	return r, err
}

func TestResetOK(t *testing.T) {
	f := &fake{results: []command.Result{{Output: "Successfully reset.\r\n\r\n"}, {}, {Output: "Direct access"}, {Output: "Flushed."}}}
	text, err := Reset(t.Context(), f.run)
	if err != nil {
		t.Fatal(err)
	}
	want := []call{
		{"netsh.exe", []string{"winsock", "reset"}},
		{"netsh.exe", []string{"int", "ip", "reset", "all"}},
		{"netsh.exe", []string{"winhttp", "reset", "proxy"}},
		{"ipconfig.exe", []string{"/flushdns"}},
	}
	if !slices.EqualFunc(f.calls, want, func(a, b call) bool { return a.exe == b.exe && slices.Equal(a.args, b.args) }) {
		t.Errorf("calls = %v", f.calls)
	}
	wantText := "> netsh winsock reset\nSuccessfully reset.\n> netsh int ip reset all\n> netsh winhttp reset proxy\nDirect access\n> ipconfig /flushdns\nFlushed."
	if text != wantText {
		t.Errorf("text =\n%s", text)
	}
}

func TestResetFailContinues(t *testing.T) {
	f := &fake{
		results: []command.Result{{Output: "Access is denied.", Code: 1}},
		errs:    []error{nil, nil, errors.New("exec: not found")},
	}
	text, err := Reset(t.Context(), f.run)
	if !errors.Is(err, ErrFailed) {
		t.Fatalf("err = %v", err)
	}
	if len(f.calls) != 4 {
		t.Errorf("%d calls, want 4", len(f.calls))
	}
	for _, s := range []string{"Access is denied.\nexit 1\n", "exec: not found\n> ipconfig"} {
		if !strings.Contains(text, s) {
			t.Errorf("text has no %q:\n%s", s, text)
		}
	}
}

func TestResetCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	f := &fake{cancel: cancel}
	text, err := Reset(ctx, f.run)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if len(f.calls) != 2 || text == "" {
		t.Errorf("%d calls, text %q", len(f.calls), text)
	}
}

func TestReboot(t *testing.T) {
	f := &fake{}
	if err := Reboot(t.Context(), f.run); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 1 || f.calls[0].exe != "shutdown.exe" || !slices.Equal(f.calls[0].args, []string{"/r", "/f", "/t", "0"}) {
		t.Errorf("calls = %v", f.calls)
	}
	f = &fake{results: []command.Result{{Output: "A required privilege is not held.", Code: 1314}}}
	if err := Reboot(t.Context(), f.run); err == nil || !strings.Contains(err.Error(), "1314") {
		t.Errorf("err = %v", err)
	}
}
