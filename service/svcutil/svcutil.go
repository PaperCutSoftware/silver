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
	"time"
	"path/filepath"

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
	MaxCount     int
	RestartDelay time.Duration
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
	l.logger.Printf("%s : %s", l.prefix, string(p)) // We assume that this operation always succeeds
	return len(p), nil
}

func ExecuteService(terminate chan struct{}, svcConfig ServiceConfig) error {
	serviceName := exeName(svcConfig.Path)
	crashHandlingExec := &crashHandlingExecutable{serviceName: serviceName, svcConfig: svcConfig}
	monitor := &serviceMonitor{serviceName: serviceName, config: svcConfig.MonitorConfig, logger: svcConfig.Logger}
	t := make(chan struct{})
	go func() {
		select {
		case <-terminate:
		case <-monitor.start(terminate):
		}
		close(t)
	}()
	_, err := crashHandlingExec.Executable(t)
	return err
}

type crashHandlingExecutable struct {
	svcConfig   ServiceConfig
	serviceName string
}

func (che *crashHandlingExecutable) Executable(terminate chan struct{}) (exitCode int, err error) {
	crashCount := 0
	var crashStart time.Time
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
		exitCode, err := executable.Execute(terminate)
		if err != nil || exitCode != 0 {
			if crashCount == 0 {
				crashStart = time.Now()
			}
			crashCount++
		}
		if crashCount > che.svcConfig.CrashConfig.MaxCount && time.Since(crashStart) > 1*time.Hour {
			break
		}
		time.Sleep(che.svcConfig.CrashConfig.RestartDelay)
	}
	return 1, errors.New("Max restart exceeded")
}
