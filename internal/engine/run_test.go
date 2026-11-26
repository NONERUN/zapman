package engine

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NONERUN/zapman/internal/spec"
)

// fakeProc is a Process that stops when exit gets a code.
type fakeProc struct {
	exit   chan uint32
	once   sync.Once
	killed bool
}

func newFakeProc() *fakeProc { return &fakeProc{exit: make(chan uint32, 1)} }

func (p *fakeProc) stop(code uint32) { p.once.Do(func() { p.exit <- code }) }

func (p *fakeProc) Wait() (uint32, error) { return <-p.exit, nil }
func (p *fakeProc) Pid() int              { return 42 }
func (p *fakeProc) Kill() error {
	p.killed = true
	p.stop(1)
	return nil
}

// fakeStarter gives the processes of codes in turn. A code that is not
// running makes the process stop at once with that code and output.
type fakeStarter struct {
	codes  []uint32
	output string
	calls  []string
	procs  []*fakeProc
}

const running uint32 = 0xFFFFFFFF

func (s *fakeStarter) start(exe, cmdLine, dir string, out io.Writer) (Process, error) {
	s.calls = append(s.calls, exe+"|"+cmdLine+"|"+dir)
	p := newFakeProc()
	s.procs = append(s.procs, p)
	code := s.codes[len(s.calls)-1]
	if code != running {
		_, _ = io.WriteString(out, s.output)
		p.stop(code)
	}
	return p, nil
}

func testBuild() *Build {
	return &Build{Engine: spec.Winws2, Exe: `C:\z\bin\winws2.exe`, Bin: `C:\z\bin`, Lines: []string{"--a=1"}}
}

func testRunner(s *fakeStarter, slept *[]time.Duration) *Runner {
	return &Runner{
		Start:       s.start,
		Sleep:       func(d time.Duration) { *slept = append(*slept, d) },
		StartWindow: 50 * time.Millisecond,
	}
}

func TestRunRetry(t *testing.T) {
	s := &fakeStarter{codes: []uint32{StatusDLLInitFailed, running}}
	var slept []time.Duration
	r, err := testRunner(s, &slept).Run(testBuild())
	if err != nil {
		t.Fatal(err)
	}
	if len(s.calls) != 2 || s.calls[0] != s.calls[1] || s.calls[0] != `C:\z\bin\winws2.exe|"C:\z\bin\winws2.exe" --chdir="C:\z\bin" --a=1|C:\z\bin` {
		t.Fatalf("calls: %q", s.calls)
	}
	if len(slept) != 1 || slept[0] != DefaultRetryDelay {
		t.Fatalf("slept %v", slept)
	}
	if !r.Retried() || r.Pid() != 42 {
		t.Fatal("Retried or Pid")
	}
	select {
	case <-r.Done():
		t.Fatal("done before stop")
	default:
	}
	if err := r.Stop(); err != nil {
		t.Fatal(err)
	}
	if !s.procs[1].killed {
		t.Fatal("not killed")
	}
	<-r.Done()
}

func TestRunRetryFails(t *testing.T) {
	s := &fakeStarter{codes: []uint32{StatusDLLInitFailed, StatusDLLInitFailed, running}, output: "line 1\nlast line\n"}
	var slept []time.Duration
	_, err := testRunner(s, &slept).Run(testBuild())
	var ee *ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("got %v", err)
	}
	if len(s.calls) != 2 || len(slept) != 1 {
		t.Fatalf("starts %d, sleeps %d: only one retry", len(s.calls), len(slept))
	}
	if ee.Code != StatusDLLInitFailed || !ee.Retried || !ee.AtStart || ee.Exe != "winws2.exe" {
		t.Fatalf("got %+v", ee)
	}
	msg := err.Error()
	for _, w := range []string{"winws2.exe", "0xC0000142", "STATUS_DLL_INIT_FAILED", "second start", "antivirus", "last line"} {
		if !strings.Contains(msg, w) {
			t.Errorf("no %q in %q", w, msg)
		}
	}
}

func TestRunEarlyExit(t *testing.T) {
	for _, code := range []uint32{1, 0} {
		s := &fakeStarter{codes: []uint32{code, running}, output: "error: bad flag\n"}
		var slept []time.Duration
		_, err := testRunner(s, &slept).Run(testBuild())
		var ee *ExitError
		if !errors.As(err, &ee) || ee.Code != code || ee.Retried {
			t.Fatalf("code %d: got %v", code, err)
		}
		if len(s.calls) != 1 || len(slept) != 0 {
			t.Fatalf("code %d: retry for a code that is not 0xC0000142", code)
		}
		if !strings.Contains(err.Error(), "error: bad flag") || strings.Contains(err.Error(), "antivirus") {
			t.Errorf("got %q", err)
		}
	}
}

func TestRunStartError(t *testing.T) {
	r := &Runner{Start: func(string, string, string, io.Writer) (Process, error) { return nil, os.ErrPermission }}
	if _, err := r.Run(testBuild()); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("got %v", err)
	}
}

func TestRunWait(t *testing.T) {
	s := &fakeStarter{codes: []uint32{running}}
	var slept []time.Duration
	r, err := testRunner(s, &slept).Run(testBuild())
	if err != nil {
		t.Fatal(err)
	}
	s.procs[0].stop(3)
	var ee *ExitError
	if err := r.Wait(); !errors.As(err, &ee) || ee.Code != 3 || ee.AtStart {
		t.Fatalf("got %v", err)
	}
	if err := r.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestTail(t *testing.T) {
	tl := &tail{limit: 4}
	_, _ = tl.Write([]byte("abc"))
	_, _ = tl.Write([]byte("defg"))
	if tl.String() != "defg" {
		t.Fatalf("got %q", tl.String())
	}
}

// TestStartProcess starts cmd.exe, which stops with 0xC0000142.
func TestStartProcess(t *testing.T) {
	cmd := filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
	if _, err := os.Stat(cmd); err != nil {
		t.Skip("no cmd.exe")
	}
	var out tail
	out.limit = 1024
	p, err := StartProcess(cmd, `cmd.exe /c echo hi & exit -1073741502`, t.TempDir(), &out)
	if err != nil {
		t.Fatal(err)
	}
	code, err := p.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if code != StatusDLLInitFailed || strings.TrimSpace(out.String()) != "hi" {
		t.Fatalf("code 0x%08X, output %q", code, out.String())
	}
	if err := p.Kill(); err != nil {
		t.Fatalf("Kill after the stop: %v", err)
	}
}
