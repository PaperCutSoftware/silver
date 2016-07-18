// SILVER - Service Wrapper
//
// Copyright (c) 2016 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package svcutil

import (
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

	serviceName := exeName(taskConf.Path)
	execConf := procmngt.ExecConfig{
		Path:             taskConf.Path,
		Args:             taskConf.Args,
		ExecTimeout:      taskConf.ExecTimeout,
		GracefulShutDown: taskConf.GracefulShutDown,
		StartupDelay:     startupDelay,
		Stdout:           &logWriter{prefix: fmt.Sprintf("%s | STDOUT", serviceName), logger: taskConf.Logger},
		Stderr:           &logWriter{prefix: fmt.Sprintf("%s | STDERR", serviceName), logger: taskConf.Logger},
	}

	executable := procmngt.NewExecutable(execConf)
	return executable.Execute(terminate)
}

func exeName(path string) string {
	return filepath.Base(path)
}

type logWriter struct {
	logger *log.Logger
	prefix string
}

func (l *logWriter) Write(p []byte) (int, error) {
	if l.logger != nil {
		// We assume that this operation always succeeds
		l.logger.Printf("%s : %s", l.prefix, string(p))
	}
	return len(p), nil
}

func ExecuteService(terminate chan struct{}, svcConfig ServiceConfig) error {
	serviceName := exeName(svcConfig.Path)
	crashHandlingExec := &crashHandlingExecutable{serviceName: serviceName, svcConfig: svcConfig}
	var t chan struct{}
	if svcConfig.MonitorConfig.URL != "" && svcConfig.MonitorConfig.Interval > 0 {
		// Wrap our terminate channel in a monitor
		monitor := &serviceMonitor{
			serviceName: serviceName,
			config:      svcConfig.MonitorConfig,
			logger:      svcConfig.Logger,
		}
		t = make(chan struct{})
		go func() {
			select {
			case <-terminate:
			case <-monitor.start(terminate):
			}
			close(t)
		}()
	} else {
		// No monitoring setup, just pass in terminate
		t = terminate
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
			Stdout:           &logWriter{prefix: fmt.Sprintf("%s | STDOUT", che.serviceName), logger: che.svcConfig.Logger},
			Stderr:           &logWriter{prefix: fmt.Sprintf("%s | STDERR", che.serviceName), logger: che.svcConfig.Logger},
		}
		executable := procmngt.NewExecutable(execConf)
		exitCode, err = executable.Execute(terminate)

		// Increment resetting every hour
		crashCount++
		if time.Since(start) > 1*time.Hour {
			start = time.Now()
			crashCount = 0
		}
		if max > 1 && crashCount >= max {
			break restartLoop
		}
		select {
		case <-terminate:
			break restartLoop
		case <-time.After(che.svcConfig.CrashConfig.RestartDelay):
		}
	}
	return exitCode, errors.New("Max crash count exceeded")
}
