// SILVER - Service Wrapper
//
// Copyright (c) 2014-2016 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//
package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

type Config struct {
	ServiceDescription ServiceDescription
	ServiceConfig      ServiceConfig
	Include            []string
	Services           []Service
	StartupTasks       []StartupTask
	ScheduledTasks     []ScheduledTask
	Commands           []Command
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

type ReplacementVars struct {
	ServiceName string
	ServiceRoot string
}

// LoadConfig parses config.
func LoadConfig(path string, vars ReplacementVars) (conf *Config, err error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("The conf file does not exist. Please configuration here: %s", path)
	}
	return load(path, vars)
}

// Merge in an include file.  Include files can contain services, tasks and commands
func MergeInclude(conf Config, path string, vars ReplacementVars) (*Config, error) {
	include, err := load(path, vars)
	if err != nil {
		return &conf, err
	}

	conf.Services = append(conf.Services, include.Services...)
	conf.StartupTasks = append(conf.StartupTasks, include.StartupTasks...)
	conf.ScheduledTasks = append(conf.ScheduledTasks, include.ScheduledTasks...)
	conf.Commands = append(conf.Commands, include.Commands...)
	return &conf, nil
}

func load(path string, vars ReplacementVars) (conf *Config, err error) {
	s, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(s, &conf)
	if err != nil {
		return nil, err
	}

	replacments := map[string]string{
		"${ServiceName}": jsonEscapeString(vars.ServiceName),
		"${ServiceRoot}": jsonEscapeString(vars.ServiceRoot),
	}
	s = []byte(replaceVars(string(s), replacments))

	err = json.Unmarshal(s, &conf)
	if err != nil {
		return nil, err
	}

	return conf, nil
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
