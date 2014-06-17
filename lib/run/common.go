// SILVER - Service Wrapper
//
// Copyright (c) 2014 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package run

import (
	"bufio"
	"io"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	// FIXME: Get rid of this dependency - use a message channel
	"github.com/papercutsoftware/silver/lib/logging"
)

const (
	defaultGracefulShutdownTimeoutSecs = 10
)

var (
	RAND *rand.Rand
)

func init() {
	RAND = rand.New(rand.NewSource(time.Now().UTC().UnixNano() + int64(os.Getpid())))
}

type RunConfig struct {
	Path                        string
	Args                        []string
	Stdout                      io.Writer
	Stderr                      io.Writer
	GracefulShutdownTimeoutSecs int
	Logger                      *log.Logger
}

func (c *RunConfig) name() string {
	return filepath.Base(c.Path)
}

type procInfo struct {
	process *os.Process
	status  chan struct{}
}

func RunWithMonitor(c *RunConfig, terminate chan struct{}) (exitCode int, err error) {

	setupRunConfigDefaults(c)

	procInfo := procInfo{}

	c.Logger.Printf("%s: Starting '%s' %v", c.name(), c.Path, c.Args)
	cmd := exec.Command(c.Path, c.Args...)

	var pOut io.ReadCloser
	if c.Stdout != nil {
		cmd.Stdout = c.Stdout
	} else {
		p, err := cmd.StdoutPipe()
		pOut = p
		if err != nil {
			return 255, err
		}
	}

	var pErr io.ReadCloser
	if c.Stderr != nil {
		cmd.Stderr = c.Stderr
	} else {
		p, err := cmd.StderrPipe()
		pErr = p
		if err != nil {
			return 255, err
		}
	}

	// Set up a process status channel - closed when process exits
	procInfo.status = make(chan struct{})
	defer close(procInfo.status)

	// This is required to control brake works on Windows
	setProcAttributes(cmd)

	if err := cmd.Start(); err != nil {
		return 255, err
	}
	procInfo.process = cmd.Process

	c.Logger.Printf("%s: Started (pid: %d)", c.name(), cmd.Process.Pid)

	if pOut != nil {
		go func() {
			pOutReader := bufio.NewReader(pOut)
			for {
				if line, _, err := pOutReader.ReadLine(); err == nil {
					c.Logger.Printf("%s: STDOUT -> %s", c.name(), line)
				} else {
					break
				}
			}
		}()
	}

	if pErr != nil {
		go func() {
			pErrReader := bufio.NewReader(pErr)
			for {
				if line, _, err := pErrReader.ReadLine(); err == nil {
					c.Logger.Printf("%s: STDERR -> %s", c.name(), line)
				} else {
					break
				}
			}
		}()
	}

	// Terminate managament
	if terminate != nil {
		go terminateOnRequest(c, &procInfo, terminate)
	}

	exitCode = 0
	if err := cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
			}
		}
	}
	c.Logger.Printf("%s: Stopped (exit code: %d)", c.name(), exitCode)
	return exitCode, nil
}

func setupRunConfigDefaults(c *RunConfig) {
	if c.GracefulShutdownTimeoutSecs == 0 {
		c.GracefulShutdownTimeoutSecs = defaultGracefulShutdownTimeoutSecs
	}
	if c.Logger == nil {
		c.Logger = logging.NewNilLogger()
	}
}

func terminateOnRequest(c *RunConfig, p *procInfo, terminate chan struct{}) {
	select {
	case <-terminate:
		c.Logger.Printf("%s: Stopping...", c.name())
		if !terminateProcess(p.process, c.GracefulShutdownTimeoutSecs) {
			c.Logger.Printf("%s: WARNING: Process not gracefully exit", c.name())
		}
	case <-p.status:
		break
	}
}

func sleep(secs int, terminate chan struct{}) {
	if secs > 0 {
		select {
		case <-terminate:
			return
		case <-time.After(time.Second * time.Duration(secs)):
			return
		}
	}
}

func sleepRandom(maxSecs int, terminate chan struct{}) {
	if maxSecs > 0 {
		secs := RAND.Intn(maxSecs)
		sleep(secs, terminate)
	}
}

func isTerminated(terminate chan struct{}) bool {
	if terminate == nil {
		return false
	}
	select {
	case _, ok := <-terminate:
		if ok {
			return false
		}
		return true
	default:
		return false
	}
}
