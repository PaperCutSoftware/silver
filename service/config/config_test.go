// SILVER - Service Wrapper
//
// Copyright (c) 2014-2016 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//
package config_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/papercutsoftware/silver/service/config"
)

func TestLoadConfig_MissingFileShouldRaiseError(t *testing.T) {
	_, err := config.LoadConfig("invalid.conf", config.ReplacementVars{})
	if err == nil {
		t.Error("Expect error on missing file")
	}
}

func TestLocalConfig_ValidConfig(t *testing.T) {
	// Arrange
	testConfig := `
    {
        "ServiceDescription" : {
            "DisplayName" : "My Service",
            "Description" : "My Service Desc"
        },
        "ServiceConfig" : {
            "StopFile" : ".stop",
            "ReloadFile" : ".reload",
            "LogFile" : "${ServiceName}.log",
            "PidFile" : "${ServiceName}.pid"
        },
		"Include" : [
			"${ServiceRoot}/v*/include.conf",
			"${ServiceRoot}/other-v*/other.conf"
		],
        "Services" : [
            {
                "Path" : "test/path/1",
                "Args" : ["arg1", "arg2"],
                "GracefulShutdownTimeoutSecs" : 12,
                "MaxCrashCountPerHour" : 999,
                "RestartDelaySecs" : 1,
                "MonitorPing" : {
                    "URL" : "http://localhost:80/login",
                    "IntervalSecs" : 30,
                    "TimeoutSecs" : 10,
                    "RestartOnFailureCount" : 3
                }
            },
            {
                "Path" : "test/path/2"
            }
        ],
        "StartupTasks" : [
            {
                "Path" : "mypath",
                "Args" : ["arg1", "arg2"],
                "Async" : false,
                "TimeoutSecs" : 1,
                "StartupDelaySecs" : 2,
                "StartupRandomDelaySecs" : 0
            }
        ],
		"EnvironmentVars" : {
			"MY_VAR1" : "MyValue1", 
			"MY_VAR2" : "MyValue2" 
		},
        "ScheduledTasks" : [
            {
                "Schedule" : "0 30 * * * *",
                "Path" : "mypath",
                "Args" : ["arg1", "arg2"],
                "TimeoutSecs" : 1,
                "StartupDelaySecs" : 2,
                "StartupRandomDelaySecs" : 0
            },
            {
                "Schedule" : "0 30 * * * *",
                "Path" : "scheduled/task/2",
                "Args" : ["arg1", "arg2"],
                "TimeoutSecs" : 999,
                "StartupDelaySecs" : 2
            }
        ],
        "Commands" : [
            {
                "Name" : "mycmd",
                "Path" : "mypath",
                "Args" : ["${ServiceRoot}/arg1", "arg2"],
                "TimeoutSecs" : 1
            },
            {
                "Name" : "mycmd2",
                "Path" : "mypath2",
                "TimeoutSecs" : 1
            }
        ]
    }`
	tmpFile := writeTestConfig(t, testConfig)
	defer os.Remove(tmpFile)

	vars := config.ReplacementVars{
		ServiceName: "MyServiceName",
		ServiceRoot: `C:\ProgramFiles\MyService`,
	}

	// Act
	c, err := config.LoadConfig(tmpFile, vars)

	// Assert
	if err != nil {
		t.Errorf("Error loading config: %v", err)
	}

	if !strings.Contains(c.ServiceConfig.LogFile, ".log") {
		t.Error("Problem extracting LogFile with variable replaement")
	}

	if strings.Contains(c.ServiceConfig.LogFile, "{ServiceName}") {
		t.Error("Variable replaement did not happen")
	}

	if !strings.Contains(c.Include[0], "include.conf") {
		t.Error("Expected include")
	}

	if c.Services[0].Path != "test/path/1" {
		t.Error("Problem extracting path")
	}

	if c.Services[0].Args[0] != "arg1" {
		t.Error("Problem extracting arg")
	}

	if c.Services[0].Args[0] != "arg1" {
		t.Error("Problem extracting arg")
	}

	if c.Services[0].MaxCrashCountPerHour != 999 {
		t.Error("MaxCrashCountPerHour field not parsed correctly")
	}

	if c.ScheduledTasks[1].Path != "scheduled/task/2" {
		t.Error("Problem extracting schedule task path")
	}

	if len(c.EnvironmentVars) != 2 {
		t.Errorf("Expected 2 EnvironmentVars, got %d", len(c.EnvironmentVars))
	}

	if c.EnvironmentVars["MY_VAR1"] != "MyValue1" {
		t.Error("Expected EnvironmentVars")
	}

	cmdArg := c.Commands[0].Args[0]
	if strings.Contains(cmdArg, "ServiceRoot") {
		t.Error(fmt.Sprintf("${ServiceRoot} replacement did not work. Got %s", cmdArg))
	}

}

func TestLocalConfig_Defaults_OK(t *testing.T) {
	// Arrange
	testConfig := `
    {
        "ServiceDescription" : {
            "DisplayName" : "My Service",
            "Description" : "My Service Desc"
        },
        "Services" : [
            {
                "Path" : "test/path/1"
            },
            {
                "Path" : "test/path/2"
            }
        ]
    }`
	tmpFile := writeTestConfig(t, testConfig)
	defer os.Remove(tmpFile)

	vars := config.ReplacementVars{
		ServiceName: "MyServiceName",
		ServiceRoot: `C:\ProgramFiles\MyService`,
	}

	// Act
	c, err := config.LoadConfig(tmpFile, vars)

	// Assert
	if err != nil {
		t.Errorf("Error loading config: %v", err)
	}

	if !strings.Contains(c.ServiceConfig.StopFile, ".stop") {
		t.Error("Expected default StopFile=.stop")
	}

	for _, service := range c.Services {
		if service.GracefulShutdownTimeoutSecs != 5 {
			t.Error("Expected default GracefulShutdownTimeoutSecs=5")
		}
	}
}

func TestLoadConfig_MinimalConfig(t *testing.T) {
	// Arrange
	testConfig := `
    {
        "ServiceDescription" : {
            "DisplayName" : "My Service",
            "Description" : "My Service Desc"
        },
        "Services" : [
            {
                "Path" : "test/path/1"
            }
        ]
    }`
	tmpFile := writeTestConfig(t, testConfig)
	defer os.Remove(tmpFile)

	vars := config.ReplacementVars{
		ServiceName: "MyServiceName",
		ServiceRoot: `C:\ProgramFiles\MyService`,
	}

	// Act
	c, err := config.LoadConfig(tmpFile, vars)

	// Assert
	if err != nil {
		t.Errorf("Error loading config: %v", err)
	}

	if c.Services[0].Path != "test/path/1" {
		t.Error("Problem extracting path")
	}

	if len(c.EnvironmentVars) != 0 {
		t.Error("Expected no environment")
	}
}

func TestLoadConfig_IncompleteConfig_ShouldError(t *testing.T) {
	// Arrange
	testConfig := `
    {
        "ServiceDescription" : {
            "DisplayName" : "My Service",
            "Description" : "My Service Desc"
        },
        "ServiceConfig" : {
            "StopFile" : ".stop",
            "ReloadFile" : ".reload",
            "LogFile" : "${ServiceName}.log",
            "PidFile" : "${ServiceName}.pid"
        },
        "Services" : [
            {
                "Path" : "test/path/1",
                "Args" : ["arg1", "arg2"],
                "GracefulShutdownTimeoutSecs" : 12,
                "MaxCrashCountPerHour" : 999,
                "RestartDelaySecs" : 1,
                "MonitorPing" : {
                    "URL" : "http://localhost:80/login",
                    "IntervalSecs" : 30,
                    "TimeoutSecs" : 10,
                    "RestartOnFailureCount" : 3
                }
            },
            {
                "Path" : "test/path/2"
            }
        ]
    }`
	tmpFile := writeTestConfig(t, testConfig)
	defer os.Remove(tmpFile)

	// Act
	c, err := config.LoadConfig(tmpFile, config.ReplacementVars{})

	// Assert
	if err != nil {
		t.Errorf("Error loading config: %v", err)
	}

	if len(c.Commands) != 0 {
		t.Error("Expected zero commands")
	}
}

func TestMergeInclude_ValidInclude(t *testing.T) {
	// Arrange
	baseConfig := `
    {
        "ServiceDescription" : {
            "DisplayName" : "My Service",
            "Description" : "My Service Desc"
        },
		"Include" : ["${ServiceRoot}/v*/service.conf"]
    }`
	baseFile := writeTestConfig(t, baseConfig)
	defer os.Remove(baseFile)

	vars := config.ReplacementVars{
		ServiceName: "MyServiceName",
		ServiceRoot: `C:\ProgramFiles\MyService`,
	}
	baseConf, err := config.LoadConfig(baseFile, vars)

	includeConfig := `
    {
        "Services" : [
            {
                "Path" : "test/path/from-include"
            }
        ],
		"EnvironmentVars" : {
			"MY_VAR1" : "MyValue1"
		}
    }`
	incFile := writeTestConfig(t, includeConfig)
	defer os.Remove(incFile)

	// Act
	baseConf, err = config.MergeInclude(*baseConf, incFile, vars)

	// Assert
	if err != nil {
		t.Errorf("Error loading config: %v", err)
	}

	if baseConf.Services[0].Path != "test/path/from-include" {
		t.Error("Problem extracting path")
	}

	if baseConf.EnvironmentVars["MY_VAR1"] != "MyValue1" {
		t.Error("Expected environment var")
	}
}

func TestMergeInclude_EmptyInclude(t *testing.T) {
	// Arrange
	baseConfig := `
    {
        "ServiceDescription" : {
            "DisplayName" : "My Service",
            "Description" : "My Service Desc"
        },
		"Include" : ["${ServiceRoot}/v*/service.conf"]
    }`
	baseFile := writeTestConfig(t, baseConfig)
	defer os.Remove(baseFile)

	vars := config.ReplacementVars{
		ServiceName: "MyServiceName",
		ServiceRoot: `C:\ProgramFiles\MyService`,
	}
	baseConf, err := config.LoadConfig(baseFile, vars)

	emptyIncludeConfig := ""
	incFile := writeTestConfig(t, emptyIncludeConfig)
	defer os.Remove(incFile)

	// Act
	baseConf, err = config.MergeInclude(*baseConf, incFile, vars)

	// Assert no error
	if err != nil {
		t.Errorf("Error loading config: %v", err)
	}
}

func writeTestConfig(t *testing.T, config string) string {
	tmpFile, err := ioutil.TempFile("", "test-config")
	if err != nil {
		t.Fatalf("Unable to write test config: %v", err)
	}
	defer tmpFile.Close()
	_, err = tmpFile.WriteString(config)
	if err != nil {
		t.Fatalf("Unable to write test config: %v", err)
	}
	return tmpFile.Name()
}

func TestLoadConfig_CrashLogFile(t *testing.T) {
	vars := config.ReplacementVars{
		ServiceName: "MyServiceName",
		ServiceRoot: `C:\ProgramFiles\MyService`,
	}
	cases := []struct {
		name  string
		value string // JSON value for CrashLogFile; "" means the key is omitted
		want  string
	}{
		{"plain path", `"logs/crash.log"`, "logs/crash.log"},
		{"ServiceRoot replaced", `"${ServiceRoot}/crash.log"`, `C:\ProgramFiles\MyService/crash.log`},
		{"ServiceName replaced", `"${ServiceName}-crash.log"`, "MyServiceName-crash.log"},
		{"both replaced", `"${ServiceRoot}/${ServiceName}-crash.log"`, `C:\ProgramFiles\MyService/MyServiceName-crash.log`},
		{"explicit empty", `""`, ""},
		{"omitted defaults to empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpFile := writeTestConfig(t, crashLogTestConfig(tc.value))
			defer os.Remove(tmpFile)

			c, err := config.LoadConfig(tmpFile, vars)
			if err != nil {
				t.Fatalf("Error loading config: %v", err)
			}
			if c.ServiceConfig.CrashLogFile != tc.want {
				t.Errorf("CrashLogFile = %q, want %q", c.ServiceConfig.CrashLogFile, tc.want)
			}
		})
	}
}

func TestLoadConfigNoReplacements_CrashLogFileKeepsPlaceholders(t *testing.T) {
	tmpFile := writeTestConfig(t, crashLogTestConfig(`"${ServiceRoot}/${ServiceName}-crash.log"`))
	defer os.Remove(tmpFile)

	c, err := config.LoadConfigNoReplacements(tmpFile)
	if err != nil {
		t.Fatalf("Error loading config: %v", err)
	}
	want := "${ServiceRoot}/${ServiceName}-crash.log"
	if c.ServiceConfig.CrashLogFile != want {
		t.Errorf("CrashLogFile = %q, want %q", c.ServiceConfig.CrashLogFile, want)
	}
}

// crashLogTestConfig builds a minimal config; jsonValue is the raw JSON value
// for ServiceConfig.CrashLogFile, or "" to omit the key entirely.
func crashLogTestConfig(jsonValue string) string {
	crash := ""
	if jsonValue != "" {
		crash = `, "CrashLogFile" : ` + jsonValue
	}
	return `
    {
        "ServiceDescription" : {
            "DisplayName" : "My Service",
            "Description" : "My Service Desc"
        },
        "ServiceConfig" : {
            "LogFile" : "${ServiceName}.log"` + crash + `
        },
        "Services" : [
            {
                "Path" : "test/path/1"
            }
        ]
    }`
}
