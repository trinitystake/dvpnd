// SPDX-License-Identifier: Apache-2.0

package common

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// Process is a protocol's child process (a proxy or VPN daemon) with the
// lifecycle every service needs: start, reap on exit, stop politely.
type Process struct {
	cmd    *exec.Cmd
	exited chan error // receives the child's Wait result once it has exited
}

// StartProcess launches name with args; extraEnv is appended to the current
// environment. The child's output goes to the node's stdout and stderr, and a
// goroutine reaps it whenever it exits so a crash leaves no zombie.
func StartProcess(name string, args []string, extraEnv []string) (*Process, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), extraEnv...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	p := &Process{cmd: cmd, exited: make(chan error, 1)}
	go func() { p.exited <- cmd.Wait() }()

	return p, nil
}

// Stop sends SIGTERM and waits for the child, killing it after timeout. An
// exit status reported after the signal is expected and not an error.
func (p *Process) Stop(timeout time.Duration) error {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return errors.New("process was not started")
	}

	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}

	select {
	case <-p.exited:
		return nil
	case <-time.After(timeout):
	}

	if err := p.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	<-p.exited

	return nil
}

// Exited reports whether the child has exited, and how.
func (p *Process) Exited() (bool, *os.ProcessState) {
	if p == nil || p.cmd == nil || p.cmd.ProcessState == nil {
		return false, nil
	}

	return true, p.cmd.ProcessState
}
