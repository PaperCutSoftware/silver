package procmngt

import (
	"errors"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/papercutsoftware/silver/lib/osutils"
)

var (
	ErrManualTerminate = errors.New("Manually terminated")
)

var (
	random *rand.Rand
)

func init() {
	random = rand.New(rand.NewSource(time.Now().UTC().UnixNano() + int64(os.Getpid())))
}

type Executable interface {
	Execute(terminate chan struct{}) (exitCode int, err error)
}

type ExecConfig struct {
	Path               string
	Args               []string
	StartupDelay       time.Duration
	StartupRandomDelay time.Duration // FIXME: Remove and move up
	ExecTimeout        time.Duration
	GracefulShutDown   time.Duration
	Stdout             io.Writer
	Stderr             io.Writer
	Stdin              io.Reader
	// FUTURE: Maybe Env?
}

type executable struct {
	cmd              *exec.Cmd
	gracefulShutdown time.Duration
}

func (c executable) Execute(terminate chan struct{}) (exitCode int, err error) {
	if err := c.cmd.Start(); err != nil {
		return 255, err
	}
	complete := make(chan struct{})
	defer close(complete)
	go func() {
		select {
		case <-terminate:
			// FUTURE: log error or return if we find we need to have visability.
			err = osutils.ProcessKillGracefully(c.cmd.Process.Pid, c.gracefulShutdown)
		case <-complete:
			return
		}
	}()

	if err := c.cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
			}
		}
	}
	return exitCode, nil
}

type startupDelayedExecutable struct {
	wrappedExecutable  Executable
	startupDelay       time.Duration
	startupRandomDelay time.Duration
}

func (sdc startupDelayedExecutable) Execute(terminate chan struct{}) (exitCode int, err error) {
	var randDelay = time.Duration(0)
	if sdc.startupRandomDelay > 0 {
		randDelay = time.Duration(random.Int63n(sdc.startupRandomDelay.Nanoseconds()))
	}
	startupDelay := sdc.startupDelay + randDelay
	select {
	case <-terminate:
		return 255, ErrManualTerminate
	case <-time.After(startupDelay):
	}
	return sdc.wrappedExecutable.Execute(terminate)
}

type timeoutExecutable struct {
	wrappedExecutable Executable
	execTimeout       time.Duration
}

func (tc timeoutExecutable) Execute(terminate chan struct{}) (exitCode int, err error) {
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
	e = executable{cmd: setupCmd(execConf), gracefulShutdown: execConf.GracefulShutDown}
	if isStartupDelayedCmd(execConf) {
		e = startupDelayedExecutable{
			wrappedExecutable:  e,
			startupDelay:       execConf.StartupDelay,
			startupRandomDelay: execConf.StartupRandomDelay,
		}
	}

	if isTimeoutCmd(execConf) {
		e = timeoutExecutable{wrappedExecutable: e, execTimeout: execConf.ExecTimeout}
	}
	return e
}

func setupCmd(cmdConf ExecConfig) *exec.Cmd {
	cmd := exec.Command(cmdConf.Path, cmdConf.Args...)
	cmd.SysProcAttr = osutils.ProcessSysProcAttrForQuit()
	cmd.Stdout = cmdConf.Stdout
	cmd.Stderr = cmdConf.Stderr
	cmd.Stdin = cmdConf.Stdin
	return cmd
}

func isStartupDelayedCmd(cmdConf ExecConfig) bool {
	return cmdConf.StartupDelay > 0 || cmdConf.StartupRandomDelay > 0
}

func isTimeoutCmd(cmdConf ExecConfig) bool {
	return cmdConf.ExecTimeout > 0
}
