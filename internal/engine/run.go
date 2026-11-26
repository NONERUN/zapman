package engine

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// StatusDLLInitFailed is the exit code STATUS_DLL_INIT_FAILED. winws2.exe
// sometimes stops with it at start; a second start then usually works. An
// antivirus can also cause it.
const StatusDLLInitFailed uint32 = 0xC0000142

// Process is a started engine process.
type Process interface {
	// Wait waits until the process stops and gives its exit code. The
	// error is for a failure of the wait, not for an exit code that is
	// not 0.
	Wait() (uint32, error)
	// Kill stops the process.
	Kill() error
	// Pid gives the process ID.
	Pid() int
}

// StartFunc starts exe with the full command line cmdLine in the folder
// dir. The process writes its output to out. It must not show a window.
type StartFunc func(exe, cmdLine, dir string, out io.Writer) (Process, error)

// Runner runs a Build in the foreground. The zero value uses StartProcess,
// time.Sleep and the default times.
type Runner struct {
	// Start starts the process. Nil is StartProcess.
	Start StartFunc
	// Sleep waits before the second start. Nil is time.Sleep.
	Sleep func(time.Duration)
	// StartWindow is the time in which an exit counts as a failure at
	// start. Zero is DefaultStartWindow.
	StartWindow time.Duration
	// RetryDelay is the time before the second start. Zero is
	// DefaultRetryDelay.
	RetryDelay time.Duration
}

// The default times of Runner.
const (
	DefaultStartWindow = 2 * time.Second
	DefaultRetryDelay  = 1 * time.Second
)

// outputLimit is the number of bytes of output that a Run keeps.
const outputLimit = 64 << 10

// ExitError tells that the engine stopped.
type ExitError struct {
	Exe     string // file name of the exe
	Code    uint32
	Output  string // the last part of the output
	AtStart bool   // the engine stopped in the start window
	Retried bool   // the engine had a second start
}

func (e *ExitError) Error() string {
	var b strings.Builder
	if e.AtStart {
		fmt.Fprintf(&b, "%s stopped at start with code 0x%08X", e.Exe, e.Code)
	} else {
		fmt.Fprintf(&b, "%s stopped with code 0x%08X", e.Exe, e.Code)
	}
	if e.Code == StatusDLLInitFailed {
		b.WriteString(" (STATUS_DLL_INIT_FAILED)")
		if e.Retried {
			b.WriteString(", also after a second start")
		}
		b.WriteString(". A DLL of the engine did not start. An antivirus can cause this: add the bin folder to its exclusions")
	}
	b.WriteString(".")
	if line := lastLine(e.Output); line != "" {
		b.WriteString(" Last output: ")
		b.WriteString(line)
	}
	return b.String()
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// Run is an engine process that runs.
type Run struct {
	b       *Build
	proc    Process
	out     *tail
	done    chan struct{}
	code    uint32
	err     error
	retried bool
}

// Run starts the engine of b and waits for the start window. When the
// engine stops in the start window with StatusDLLInitFailed, Run waits for
// RetryDelay and starts it one more time. Any other stop in the start window
// gives an ExitError. The caller must make sure that the service does not
// run.
func (r *Runner) Run(b *Build) (*Run, error) {
	start := r.Start
	if start == nil {
		start = StartProcess
	}
	sleep := r.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	window := r.StartWindow
	if window <= 0 {
		window = DefaultStartWindow
	}
	delay := r.RetryDelay
	if delay <= 0 {
		delay = DefaultRetryDelay
	}
	cmdLine := b.CommandLine()
	for try := 0; ; try++ {
		out := &tail{limit: outputLimit}
		p, err := start(b.Exe, cmdLine, b.Bin, out)
		if err != nil {
			return nil, fmt.Errorf("start %s: %w", filepath.Base(b.Exe), err)
		}
		run := &Run{b: b, proc: p, out: out, done: make(chan struct{}), retried: try > 0}
		go run.wait()
		timer := time.NewTimer(window)
		select {
		case <-run.done:
			timer.Stop()
			if run.err != nil {
				return nil, run.err
			}
			if run.code == StatusDLLInitFailed && try == 0 {
				sleep(delay)
				continue
			}
			return nil, run.exitError(true)
		case <-timer.C:
			return run, nil
		}
	}
}

func (r *Run) wait() {
	r.code, r.err = r.proc.Wait()
	close(r.done)
}

func (r *Run) exitError(atStart bool) *ExitError {
	return &ExitError{
		Exe:     filepath.Base(r.b.Exe),
		Code:    r.code,
		Output:  r.out.String(),
		AtStart: atStart,
		Retried: r.retried,
	}
}

// Pid gives the process ID.
func (r *Run) Pid() int { return r.proc.Pid() }

// Retried tells if the engine had a second start.
func (r *Run) Retried() bool { return r.retried }

// Done is closed when the process stops.
func (r *Run) Done() <-chan struct{} { return r.done }

// Wait waits until the process stops. It gives an ExitError, or the error
// of the wait.
func (r *Run) Wait() error {
	<-r.done
	if r.err != nil {
		return r.err
	}
	return r.exitError(false)
}

// Stop stops the process and waits for it.
func (r *Run) Stop() error {
	select {
	case <-r.done:
		return nil
	default:
	}
	if err := r.proc.Kill(); err != nil {
		// The process can stop at the same time. Then Kill fails, but
		// the process does not run.
		select {
		case <-r.done:
			return nil
		case <-time.After(time.Second):
			return fmt.Errorf("stop %s: %w", filepath.Base(r.b.Exe), err)
		}
	}
	<-r.done
	return nil
}

// Output gives the last part of the output of the process.
func (r *Run) Output() string { return r.out.String() }

// tail is a writer that keeps the last limit bytes.
type tail struct {
	mu    sync.Mutex
	limit int
	buf   []byte
}

func (t *tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if over := len(t.buf) - t.limit; over > 0 {
		t.buf = append(t.buf[:0], t.buf[over:]...)
	}
	return len(p), nil
}

func (t *tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}
