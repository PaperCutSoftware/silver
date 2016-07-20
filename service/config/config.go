// SILVER - Service Wrapper
//
// Copyright (c) 2014-2016 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//
// FUTURE: Parsing structs should be separated from returns structs.  The
//         return structs should have types like time.Duration, etc.
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
	GracefulShutdownTimeoutSecs int
	MaxCrashCountPerHour        int
	RestartDelaySecs            int
	StartupDelaySecs            int
	MonitorPing                 *MonitorPing
}

type MonitorPing struct {
	URL                   string
	IntervalSecs          int
	TimeoutSecs           int
	StartupDelaySecs      int
	RestartOnFailureCount int
}

type Task struct {
	command
	TimeoutSecs            int
	StartupDelaySecs       int
	StartupRandomDelaySecs int
}

type StartupTask struct {
	Task
	Async bool
}

type ScheduledTask struct {
	Task
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
	conf, err = load(path, vars)
	if err != nil {
		return nil, err
	}
	err = validate(conf)
	if err != nil {
		return nil, err
	}
	return conf, nil
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

	replacements := map[string]string{
		"${ServiceName}": jsonEscapeString(vars.ServiceName),
		"${ServiceRoot}": jsonEscapeString(vars.ServiceRoot),
	}
	s = []byte(replaceVars(string(s), replacements))

	err = json.Unmarshal(s, &conf)
	if err != nil {
		return nil, err
	}

	applyDefaults(conf)

	return conf, nil
}

func validate(conf *Config) error {
	if conf.ServiceDescription.DisplayName == "" {
		return fmt.Errorf("ServiceDescription.DisplayName is required configuration")
	}
	return nil
}

func applyDefaults(conf *Config) {
	if conf.ServiceConfig.StopFile == "" {
		conf.ServiceConfig.StopFile = ".stop"
	}
	if conf.ServiceConfig.ReloadFile == "" {
		conf.ServiceConfig.ReloadFile = ".reload"
	}

	if conf.ServiceConfig.LogFileMaxSizeMb == 0 {
		conf.ServiceConfig.LogFileMaxSizeMb = 50
	}

	// Default graceful is 5 seconds
	for i := range conf.Services {
		if conf.Services[i].GracefulShutdownTimeoutSecs == 0 {
			conf.Services[i].GracefulShutdownTimeoutSecs = 5
		}
	}
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
