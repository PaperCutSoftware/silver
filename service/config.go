// SILVER - Service Wrapper
//
// Copyright (c) 2014 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	ServiceDescription *ServiceDescription
	ServiceConfig      *ServiceConfig
	Services           []*Service
	StartupTasks       []*StartupTask
	ScheduledTasks     []*ScheduledTask
	Commands           []*Command
}

type ServiceDescription struct {
	Name        string
	DisplayName string
	Description string
}

type ServiceConfig struct {
	StopFile         string
	ReloadFile       string
	LogFile          string
	LogFileMaxSizeMb int
	PidFile          string
}

type command struct {
	Path string
	Args []string
}

type Service struct {
	command
	GracefulShutdownTimeout int
	MaxCrashCount           int
	RestartDelaySecs        int
	StartupDelaySecs        int
	MonitorPing             *MonitorPing
}

type MonitorPing struct {
	URL                   string
	IntervalSecs          int
	TimeoutSecs           int
	StartupDelaySecs      int
	RestartOnFailureCount int
}

type task struct {
	command
	TimeoutSecs            int
	StartupDelaySecs       int
	StartupRandomDelaySecs int
}

type StartupTask struct {
	task
	Async bool
}

type ScheduledTask struct {
	task
	Schedule string
}

type Command struct {
	command
	Name        string
	TimeoutSecs int
}

func LoadConfig() (config *Config, err error) {
	f := getConfigFilePath()
	if _, err := os.Stat(f); os.IsNotExist(err) {
		msg := fmt.Sprintf("The conf file does not exist. "+
			"Place configuration here: %s", f)
		return nil, errors.New(msg)
	}
	return loadConfigFromFile(f)
}

func getConfigFilePath() string {
	exePath := exePath()
	extension := filepath.Ext(exePath)
	if strings.ToLower(extension) == ".exe" {
		return exePath[0:len(exePath)-4] + ".conf"
	} else {
		return exePath + ".conf"
	}
}

func loadConfigFromFile(configFile string) (config *Config, err error) {

	s, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(s, &config)
	if err != nil {
		return nil, err
	}

	// We've parsed once to extract 'ServiceName'.  Now replace and
	// parse again.
	serviceName := serviceName()
	serviceRoot := exeFolder()

	replacments := map[string]string{
		"${ServiceName}": jsonEscapeString(serviceName),
		"${ServiceRoot}": jsonEscapeString(serviceRoot),
	}
	s = []byte(replaceVars(string(s), replacments))

	err = json.Unmarshal(s, &config)
	if err != nil {
		return nil, err
	}

	// Validate
	if config.ServiceConfig == nil {
		config.ServiceConfig = new(ServiceConfig)
	}
	if config.ServiceDescription == nil {
		config.ServiceDescription = new(ServiceDescription)
	}

	return config, nil
}

func replaceVars(in string, replacements map[string]string) (out string) {
	out = in
	for key, value := range replacements {
		out = strings.Replace(out, key, value, -1)
	}
	return out
}

func jsonEscapeString(in string) (out string) {
	// FIXME: We should be a bit smarter
	r := strings.NewReplacer("\\", "\\\\", "\"", "\\\"")
	return r.Replace(in)
}
