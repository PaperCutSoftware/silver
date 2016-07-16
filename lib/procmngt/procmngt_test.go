// SILVER - Service Wrapper
//
// Copyright (c) 2016 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package procmngt_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/papercutsoftware/silver/lib/procmngt"
)

func Test_CustomLoggerForStdout(t *testing.T) {
	// Arrange
	tmpDir, testExe := makeHelloWorld(t)
	defer os.RemoveAll(tmpDir)
	output := &bytes.Buffer{}
	execConf := procmngt.ExecConfig{
		Path:   testExe,
		Args:   []string{"go test"},
		Stdout: output,
	}
	executable := procmngt.NewExecutable(execConf)

	// Act
	exitCode, err := executable.Execute(nil)

	// Assert
	if err != nil {
		t.Fatalf("The command should be executed successfully")
	}
	if exitCode != 0 {
		t.Fatalf("The command should exit with 0")
	}
	if !strings.Contains(output.String(), "go test") {
		t.Fatalf("The command did not use the custom logger for standard out")
	}
}

func Test_CustomLoggerForStderr(t *testing.T) {
	// Arrange
	tmpDir, testExe := makeHelloWorld(t)
	defer os.RemoveAll(tmpDir)
	output := &bytes.Buffer{}
	execConf := procmngt.ExecConfig{
		Path:   testExe,
		Args:   []string{"ERROR:go test"},
		Stderr: output,
	}
	executable := procmngt.NewExecutable(execConf)

	// Act
	exitCode, err := executable.Execute(nil)

	// Assert
	if err != nil {
		t.Fatalf("The command should be executed successfully")
	}
	if exitCode != 1 {
		t.Fatalf("The command should exit with 1")
	}
	if !strings.Contains(output.String(), "ERROR:go test") {
		t.Fatalf("The command did not use the custom logger for standard error")
	}
}

func Test_FixedDelayedStartupCommand(t *testing.T) {
	// Arrange
	tmpDir, testExe := makeHelloWorld(t)
	defer os.RemoveAll(tmpDir)
	delayed := 1 * time.Second
	execConf := procmngt.ExecConfig{
		Path:         testExe,
		StartupDelay: delayed,
	}
	executable := procmngt.NewExecutable(execConf)
	start := time.Now()

	// Act
	executable.Execute(nil)

	// Assert
	elapsed := time.Since(start)
	if elapsed < delayed {
		t.Fatalf("Startup delayed is not enforced")
	}
}

func Test_TimedOutCommand(t *testing.T) {
	// Arrange
	tmpDir, testExe := makeHelloWorldForever(t)
	defer os.RemoveAll(tmpDir)
	timeout := 1 * time.Second
	execConf := procmngt.ExecConfig{
		Path:        testExe,
		ExecTimeout: timeout,
	}
	executable := procmngt.NewExecutable(execConf)
	start := time.Now()

	// Act
	executable.Execute(nil)

	// Assert
	elapsed := time.Since(start)
	if elapsed < timeout {
		t.Fatalf("The command exit before the timeout")
	}
	threshold := 500 * time.Millisecond
	if elapsed > timeout+threshold {
		t.Fatalf("Timeout is not enforced")
	}
}

func Test_StartupDelayedAndTimedOutCommand(t *testing.T) {
	// Arrange
	tmpDir, testExe := makeHelloWorldForever(t)
	defer os.RemoveAll(tmpDir)
	delayed := 1 * time.Second
	timeout := 1 * time.Second
	execConf := procmngt.ExecConfig{
		Path:         testExe,
		ExecTimeout:  timeout,
		StartupDelay: delayed,
	}
	executable := procmngt.NewExecutable(execConf)
	start := time.Now()

	// Act
	executable.Execute(nil)

	// Assert
	elapsed := time.Since(start)
	threshold := 500 * time.Millisecond // the command should take less than threshold to run
	if elapsed >= timeout+delayed+threshold {
		t.Fatalf("The command took longer than expected")
	}
}

func Test_GracefulShutDownCommand(t *testing.T) {
	// Arrange
	tmpDir, testExe := makeHelloWorldForever(t)
	defer os.RemoveAll(tmpDir)
	execConf := procmngt.ExecConfig{
		Path:             testExe,
		GracefulShutDown: 1 * time.Second,
	}
	executable := procmngt.NewExecutable(execConf)
	start := time.Now()
	terminate := make(chan struct{})
	go func() {
		select {
		case <-time.After(2 * time.Second):
			close(terminate)
		}
	}()

	// Act
	executable.Execute(terminate)

	// Assert
	elapsed := time.Since(start)
	threshold := 500 * time.Millisecond // the command should take less than threshold to run
	if elapsed > 3*time.Second+threshold {
		t.Fatalf("The command was not shut down")
	}
}

func makeHelloWorld(t *testing.T) (tmpDir, testExe string) {
	_, thisFile, _, _ := runtime.Caller(0)
	helloWorldGo := path.Dir(thisFile) + "/testexes/helloworld.go"
	return makeTestExe(t, helloWorldGo)
}

func makeHelloWorldForever(t *testing.T) (tmpDir, testExe string) {
	_, thisFile, _, _ := runtime.Caller(0)
	helloWorldGo := path.Dir(thisFile) + "/testexes/helloforever.go"
	return makeTestExe(t, helloWorldGo)
}

func makeTestExe(t *testing.T, testSrc string) (tmpDir, testExe string) {
	tmpDir, err := ioutil.TempDir("", "TestProcmgmt")
	if err != nil {
		t.Fatal("TempDir failed: ", err)
	}

	// compile it
	exe := filepath.Join(tmpDir, path.Base(testSrc)) + ".exe"
	o, err := exec.Command("go", "build", "-o", exe, testSrc).CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to compile: %v\n%v", err, string(o))
	}
	return tmpDir, exe
}
