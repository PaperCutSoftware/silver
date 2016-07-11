package procmngt

import (
	"errors"
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
	StartupRandomDelay time.Duration
	ExecTimeout        time.Duration
	GracefulShutDown   time.Duration
}

type executable struct {
	cmd              *exec.Cmd
	gracefulShutdown time.Duration
}

func (c executable) Execute(terminate chan struct{}) (exitCode int, err error) {
	c.cmd.SysProcAttr = osutils.ProcessSysProcAttrForQuit()
	if err := c.cmd.Start(); err != nil {
		return 255, err
	}
	complete := make(chan struct{})
	defer close(complete)
	go func() {
		select {
		case <-terminate:
			// TODO: log error
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
	wrappedCmd         Executable
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
	return sdc.wrappedCmd.Execute(terminate)
}

type timeoutExecutable struct {
	wrappedCmd  Executable
	execTimeout time.Duration
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
	return tc.wrappedCmd.Execute(t)
}

func NewExecutable(execConf ExecConfig) Executable {
	var silverExec Executable
	silverExec = executable{cmd: setupLogging(execConf), gracefulShutdown: execConf.GracefulShutDown}
	if isStartupDelayedCmd(execConf) {
		silverExec = startupDelayedExecutable{
			wrappedCmd:         silverExec,
			startupDelay:       execConf.StartupDelay,
			startupRandomDelay: execConf.StartupRandomDelay,
		}
	}

	if isTimeoutCmd(execConf) {
		silverExec = timeoutExecutable{wrappedCmd: silverExec, execTimeout: execConf.ExecTimeout}
	}
	return silverExec
}

func setupLogging(cmdConf ExecConfig) *exec.Cmd {
	cmd := exec.Command(cmdConf.Path, cmdConf.Args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func isStartupDelayedCmd(cmdConf ExecConfig) bool {
	return cmdConf.StartupDelay > 0 || cmdConf.StartupRandomDelay > 0
}

func isTimeoutCmd(cmdConf ExecConfig) bool {
	return cmdConf.ExecTimeout > 0
}
