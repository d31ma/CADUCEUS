package entry

import (
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// child is one supervised subprocess with inherited stdio.
type child struct {
	name     string
	cmd      *exec.Cmd
	done     chan struct{}
	mu       sync.Mutex
	exited   bool
	code     int
	signaled bool
	signal   string
	startErr error
}

func spawn(stdout, stderr io.Writer, name string, args ...string) *child {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = os.Environ()
	process := &child{name: name, cmd: cmd, done: make(chan struct{})}
	if err := cmd.Start(); err != nil {
		process.startErr = err
		process.exited = true
		process.code = 1
		close(process.done)
		return process
	}
	go func() {
		err := cmd.Wait()
		process.mu.Lock()
		process.exited = true
		state := cmd.ProcessState
		if state != nil {
			process.code = state.ExitCode()
			if status, ok := state.Sys().(syscall.WaitStatus); ok && status.Signaled() {
				process.signaled = true
				process.signal = signalName(status.Signal())
			}
		} else if err != nil {
			process.code = 1
		}
		process.mu.Unlock()
		close(process.done)
	}()
	return process
}

// result answers the exit code as Node's `code ?? (signal ? 1 : 0)`.
func (process *child) result() int {
	<-process.done
	process.mu.Lock()
	defer process.mu.Unlock()
	if process.startErr != nil || process.signaled {
		return 1
	}
	return process.code
}

func (process *child) running() bool {
	process.mu.Lock()
	defer process.mu.Unlock()
	return !process.exited
}

func (process *child) kill(signal os.Signal) {
	if process.cmd.Process == nil || !process.running() {
		return
	}
	_ = process.cmd.Process.Signal(signal)
}

// stop sends SIGTERM, waits up to ten seconds, then SIGKILL, as the
// JavaScript stopChild did.
func (process *child) stop() {
	if !process.running() {
		return
	}
	process.kill(syscall.SIGTERM)
	select {
	case <-process.done:
	case <-time.After(10 * time.Second):
		process.kill(os.Kill)
		<-process.done
	}
}

func signalName(signal os.Signal) string {
	switch signal {
	case syscall.SIGTERM:
		return "SIGTERM"
	case syscall.SIGINT:
		return "SIGINT"
	case syscall.SIGKILL:
		return "SIGKILL"
	case syscall.SIGHUP:
		return "SIGHUP"
	}
	if signal == nil {
		return ""
	}
	return signal.String()
}
