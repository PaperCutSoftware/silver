// SILVER - Service Wrapper
//
// Copyright (c) 2014-2021 PaperCut Software http://www.papercut.com/
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
	"strings"

	"github.com/papercutsoftware/silver/lib/osutils"
)

const stopFileName = ".stop"
const ReloadFileName = ".reload"

type Config struct {
	ServiceDescription ServiceDescription
	ServiceConfig      ServiceConfig
	Include            []string
	EnvironmentVars    map[string]string
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
	StopFile               string
	ReloadFile             string
	LogFile                string
	LogFileMaxSizeMb       int64
	LogFileMaxBackupFiles  int
	PidFile                string
	UserLevel              bool
	UserName               string
	LogFileTimestampFormat string
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
	if !osutils.FileExists(path) {
		return nil, fmt.Errorf("The conf file does not exist. Please put the configuration file here: %s", path)
	}
	conf, err = load(path, vars)
	if err != nil {
		return nil, err
	}
	err = conf.validate()
	if err != nil {
		return nil, err
	}
	return conf, nil
}

// MergeInclude merges in an include file.  Include files can contain services, tasks and commands
func MergeInclude(conf Config, path string, vars ReplacementVars) (*Config, error) {
	include, err := load(path, vars)
	if err != nil {
		return &conf, err
	}

	conf.Services = append(conf.Services, include.Services...)
	conf.StartupTasks = append(conf.StartupTasks, include.StartupTasks...)
	conf.ScheduledTasks = append(conf.ScheduledTasks, include.ScheduledTasks...)
	conf.Commands = append(conf.Commands, include.Commands...)
	for k, v := range include.EnvironmentVars {
		conf.EnvironmentVars[k] = v
	}
	return &conf, nil
}

// LoadConfigNoReplacements parse config similar to LoadConfig but retains any variables found without replacing them.
func LoadConfigNoReplacements(filePath string) (*Config, error) {
	conf, err := LoadConfig(filePath, ReplacementVars{
		ServiceName: "${ServiceName}",
		ServiceRoot: "${ServiceRoot}",
	})
	return conf, err
}

func load(path string, vars ReplacementVars) (conf *Config, err error) {
	s, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Special case for an empty file (empty file will raise error with JSON parser)
	if string(s) == "" {
		conf = &Config{}
		conf.applyDefaults()
		return conf, nil
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

	conf.applyDefaults()

	return conf, nil
}

func (conf *Config) FindCommand(cmdName string) *Command {
	for _, c := range conf.Commands {
		if c.Name == cmdName {
			return &c
		}
	}
	return nil
}

func (conf *Config) validate() error {
	if conf.ServiceDescription.DisplayName == "" {
		return fmt.Errorf("ServiceDescription.DisplayName is required configuration")
	}
	return nil
}

func (conf *Config) applyDefaults() {
	if conf.ServiceConfig.StopFile == "" {
		conf.ServiceConfig.StopFile = stopFileName
	}
	if conf.ServiceConfig.ReloadFile == "" {
		conf.ServiceConfig.ReloadFile = ReloadFileName
	}

	if conf.ServiceConfig.LogFileMaxSizeMb == 0 {
		conf.ServiceConfig.LogFileMaxSizeMb = 50
	}
	if conf.ServiceConfig.LogFileMaxBackupFiles == 0 {
		conf.ServiceConfig.LogFileMaxBackupFiles = 1
	}

	if conf.EnvironmentVars == nil {
		conf.EnvironmentVars = make(map[string]string)
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
		out = strings.ReplaceAll(out, key, value)
	}
	return out
}

func jsonEscapeString(in string) (out string) {
	// FIXME: We should be a bit smarter
	r := strings.NewReplacer("\\", "\\\\", "\"", "\\\"")
	return r.Replace(in)
}
