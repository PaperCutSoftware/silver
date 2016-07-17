// SILVER - Service Wrapper
//
// Copyright (c) 2016 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package procmngt

import (
	"errors"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/papercutsoftware/silver/lib/osutils"
)

const (
	errorExitCode = 255
)

var (
	errManualTerminate = errors.New("Manually terminated")
)

type Executable interface {
	Execute(terminate <-chan struct{}) (exitCode int, err error)
}

type ExecConfig struct {
	Path             string
	Args             []string
	StartupDelay     time.Duration
	ExecTimeout      time.Duration
	GracefulShutDown time.Duration
	Stdout           io.Writer
	Stderr           io.Writer
	Stdin            io.Reader
	Env              []string
}

type executable struct {
	cmd              *exec.Cmd
	gracefulShutdown time.Duration
}

func (c executable) Execute(terminate <-chan struct{}) (exitCode int, err error) {
	if err := c.cmd.Start(); err != nil {
		return errorExitCode, err
	}
	var done sync.WaitGroup
	done.Add(1)
	complete := make(chan struct{})
	go func() {
		defer done.Done()
		select {
		case <-terminate:
			// FUTURE: log error or return if we find we need to have visibility.
			err = osutils.ProcessKillGracefully(c.cmd.Process.Pid, c.gracefulShutdown)
		case <-complete:
			return
		}
	}()

	if err := c.cmd.Wait(); err != nil {
		// Try to get exit code from the underlining OS
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
			}
		}
	}
	//Have to call these here to avoid race condition
	close(complete)
	done.Wait()
	return exitCode, nil
}

type startupDelayedExecutable struct {
	wrappedExecutable Executable
	startupDelay      time.Duration
}

func (sdc startupDelayedExecutable) Execute(terminate <-chan struct{}) (exitCode int, err error) {
	select {
	case <-terminate:
		return errorExitCode, errManualTerminate
	case <-time.After(sdc.startupDelay):
	}
	return sdc.wrappedExecutable.Execute(terminate)
}

type timeoutExecutable struct {
	wrappedExecutable Executable
	execTimeout       time.Duration
}

func (tc timeoutExecutable) Execute(terminate <-chan struct{}) (exitCode int, err error) {
	t := make(chan struct{})
	go func() {
		select {
		case <-time.After(tc.execTimeout):
		case <-terminate:
		}
		close(t)
	}()
	return tc.wrappedExecutable.Execute(t)
}

func NewExecutable(execConf ExecConfig) Executable {
	var e Executable
	e = executable{
		cmd:              setupCmd(execConf),
		gracefulShutdown: execConf.GracefulShutDown,
	}
	if isStartupDelayedCmd(execConf) {
		e = startupDelayedExecutable{
			wrappedExecutable: e,
			startupDelay:      execConf.StartupDelay,
		}
	}

	if isTimeoutCmd(execConf) {
		e = timeoutExecutable{
			wrappedExecutable: e,
			execTimeout:       execConf.ExecTimeout,
		}
	}
	return e
}

func setupCmd(exeConf ExecConfig) *exec.Cmd {
	cmd := exec.Command(exeConf.Path, exeConf.Args...)
	cmd.SysProcAttr = osutils.ProcessSysProcAttrForQuit()
	cmd.Stdout = exeConf.Stdout
	cmd.Stderr = exeConf.Stderr
	cmd.Stdin = exeConf.Stdin
	cmd.Env = exeConf.Env
	return cmd
}

func isStartupDelayedCmd(cmdConf ExecConfig) bool {
	return cmdConf.StartupDelay > 0
}

func isTimeoutCmd(cmdConf ExecConfig) bool {
	return cmdConf.ExecTimeout > 0
}
