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
		t.Errorf("Expect error on missing file")
	}
}

func TestLocadConfig_ValidConfig(t *testing.T) {
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
                "MaxCrashCount" : 999,
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
		t.Errorf("Problem extracting LogFile with variable replaement")
	}

	if strings.Contains(c.ServiceConfig.LogFile, "{ServiceName}") {
		t.Errorf("Variable replaement did not happen")
	}

	if !strings.Contains(c.Include[0], "include.conf") {
		t.Errorf("Expected include")
	}

	if c.Services[0].Path != "test/path/1" {
		t.Errorf("Problem extracting path")
	}

	if c.Services[0].Args[0] != "arg1" {
		t.Errorf("Problem extracting arg")
	}

	if c.Services[0].Args[0] != "arg1" {
		t.Errorf("Problem extracting arg")
	}

	if c.ScheduledTasks[1].Path != "scheduled/task/2" {
		t.Error("Problem extracting schedule task path")
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
		t.Errorf("Expected default StopFile=.stop")
	}

	for _, service := range c.Services {
		if service.GracefulShutdownTimeoutSecs != 5 {
			t.Errorf("Expected default GracefulShutdownTimeoutSecs=5")
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
		t.Errorf("Problem extracting path")
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
                "MaxCrashCount" : 999,
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
        ]
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
		t.Errorf("Problem extracting path")
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
