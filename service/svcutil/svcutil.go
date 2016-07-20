// SILVER - Service Wrapper
//
// Copyright (c) 2016 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package svcutil

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/papercutsoftware/silver/lib/procmngt"
)

var (
	random *rand.Rand
)

func init() {
	random = rand.New(rand.NewSource(time.Now().UTC().UnixNano() + int64(os.Getpid())))
}

type TaskConfig struct {
	Path               string
	Args               []string
	StartupDelay       time.Duration
	StartupRandomDelay time.Duration
	ExecTimeout        time.Duration
	GracefulShutDown   time.Duration
	Logger             *log.Logger
}

type ScheduleTaskConfig struct {
	TaskConfig
	Schedule string
}

type ServiceConfig struct {
	Path             string
	Args             []string
	StartupDelay     time.Duration
	GracefulShutDown time.Duration
	Logger           *log.Logger
	CrashConfig      CrashConfig
	MonitorConfig    MonitorConfig
}

type CrashConfig struct {
	MaxCountPerHour int
	RestartDelay    time.Duration
}

func ExecuteTask(terminate chan struct{}, taskConf TaskConfig) (exitCode int, err error) {
	startupDelay := taskConf.StartupDelay
	if taskConf.StartupRandomDelay > 0 {
		startupDelay = startupDelay + time.Duration(random.Int63n(taskConf.StartupRandomDelay.Nanoseconds()))
	}

	taskName := exeName(taskConf.Path)
	execConf := procmngt.ExecConfig{
		Path:             taskConf.Path,
		Args:             taskConf.Args,
		ExecTimeout:      taskConf.ExecTimeout,
		GracefulShutDown: taskConf.GracefulShutDown,
		StartupDelay:     startupDelay,
		Stdout:           &logWriter{prefix: fmt.Sprintf("%s: STDOUT|", taskName), logger: taskConf.Logger},
		Stderr:           &logWriter{prefix: fmt.Sprintf("%s: STDERR|", taskName), logger: taskConf.Logger},
	}

	go func() {
		<-terminate
		logf(taskConf.Logger, taskName, "Stopping task...")
	}()

	executable := procmngt.NewExecutable(execConf)
	logf(taskConf.Logger, taskName, "Starting task...")
	return executable.Execute(terminate)
}

func exeName(path string) string {
	return filepath.Base(path)
}

func logf(l *log.Logger, exeName string, format string, v ...interface{}) {
	if l != nil {
		l.Printf("%s: %s", exeName, fmt.Sprintf(format, v...))
	}
}

type logWriter struct {
	logger *log.Logger
	prefix string
	buf    bytes.Buffer
}

func (l *logWriter) Write(p []byte) (int, error) {
	if l.logger == nil {
		return len(p), nil
	}
	// Write lines that we can find, otherwise leave in buffer
	l.buf.Write(p)

	scanner := bufio.NewScanner(&l.buf)
	for scanner.Scan() {
		l.logger.Printf("%s%s", l.prefix, scanner.Text())
	}
	return len(p), nil
}

func ExecuteService(terminate chan struct{}, svcConfig ServiceConfig) error {
	serviceName := exeName(svcConfig.Path)
	crashHandlingExec := &crashHandlingExecutable{serviceName: serviceName, svcConfig: svcConfig}
	go func() {
		<-terminate
		logf(svcConfig.Logger, serviceName, "Stopping service...")
	}()
	t := terminate
	if svcConfig.MonitorConfig.URL != "" && svcConfig.MonitorConfig.Interval > 0 {
		t = make(chan struct{})
		// Wrap our terminate channel in a monitor
		logf(svcConfig.Logger, serviceName, "Starting service with monitor %s", svcConfig.MonitorConfig.URL)
		monitor := &serviceMonitor{
			serviceName: serviceName,
			config:      svcConfig.MonitorConfig,
			logger:      svcConfig.Logger,
		}
		go func() {
			select {
			case <-terminate:
			case <-monitor.start(terminate):
			}
			close(t)
		}()
	}
	_, err := crashHandlingExec.Executable(t)
	return err
}

type crashHandlingExecutable struct {
	svcConfig   ServiceConfig
	serviceName string
}

func (che *crashHandlingExecutable) Executable(terminate chan struct{}) (exitCode int, err error) {
	crashCount := 0
	max := che.svcConfig.CrashConfig.MaxCountPerHour
	restartDelay := che.svcConfig.CrashConfig.RestartDelay
	if restartDelay == 0 {
		restartDelay = time.Millisecond
	}
	start := time.Now()
restartLoop:
	for {
		execConf := procmngt.ExecConfig{
			Path:             che.svcConfig.Path,
			Args:             che.svcConfig.Args,
			GracefulShutDown: che.svcConfig.GracefulShutDown,
			StartupDelay:     che.svcConfig.StartupDelay,
			Stdout:           &logWriter{prefix: fmt.Sprintf("%s: STDOUT|", che.serviceName), logger: che.svcConfig.Logger},
			Stderr:           &logWriter{prefix: fmt.Sprintf("%s: STDERR|", che.serviceName), logger: che.svcConfig.Logger},
		}
		executable := procmngt.NewExecutable(execConf)
		logf(che.svcConfig.Logger, che.serviceName, "Starting service...")
		exitCode, err = executable.Execute(terminate)
		if err != nil {
			logf(che.svcConfig.Logger, che.serviceName, "Service returned error: %v", err)
		} else {
			logf(che.svcConfig.Logger, che.serviceName, "Service stopped with exit code %d", exitCode)
		}

		// Increment resetting every hour
		crashCount++
		if time.Since(start) > 1*time.Hour {
			start = time.Now()
			crashCount = 0
		}
		if max > 1 && crashCount >= max {
			err = errors.New("Max crash count exceeded.")
			break restartLoop
		}
		select {
		case <-terminate:
			break restartLoop
		case <-time.After(che.svcConfig.CrashConfig.RestartDelay):
		}
		logf(che.svcConfig.Logger, che.serviceName, "Restarting service (crash count: %d)", crashCount)
	}
	return exitCode, err
}
