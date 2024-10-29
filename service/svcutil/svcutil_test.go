// SILVER - Service Wrapper
//
// Copyright (c) 2016, 2017 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package svcutil_test

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/papercutsoftware/silver/service/svcutil"
)

func Test_ExecuteRandomStartupDelayTask(t *testing.T) {
	// Arrange
	tmpDir, testExe := makeHelloWorldExe(t)
	defer os.RemoveAll(tmpDir)
	delay := time.Duration(1 * time.Second)
	randomDelay := time.Duration(1 * time.Second)
	taskConf := svcutil.TaskConfig{
		Path:               testExe,
		StartupDelay:       delay,
		StartupRandomDelay: randomDelay,
	}
	start := time.Now()

	// Act
	svcutil.ExecuteTask(nil, taskConf)

	// Assert
	elapsed := time.Since(start)
	threshold := time.Duration(300 * time.Millisecond) // the command should take less than threshold to run
	if elapsed <= delay {
		t.Fatalf("The task start up delay is not enforced")
	}

	if elapsed >= delay+randomDelay+threshold {
		t.Fatalf("The task random start up delay is not enforced")
	}
}

func Test_ExecuteTask_ExecTimeout(t *testing.T) {
	// Arrange
	const timeout = 1 * time.Second
	tmpDir, testExe := makeHelloForeverExe(t)
	defer os.RemoveAll(tmpDir)
	taskConf := svcutil.TaskConfig{
		Path:        testExe,
		ExecTimeout: timeout,
	}
	start := time.Now()

	// Act
	svcutil.ExecuteTask(nil, taskConf)

	// Assert
	elapsed := time.Since(start)
	threshold := time.Duration(200 * time.Millisecond)
	if elapsed > timeout+threshold {
		t.Fatalf("Elapse time longer than expected.  Took: %v", elapsed)
	}

	if elapsed < timeout {
		t.Fatalf("Did not expect task to run shorter than our timeout")
	}
}

func Test_ExecuteTask_GracefulShutDown_OK(t *testing.T) {
	// Arrange
	const shutdownIn = 1 * time.Second
	tmpDir, testExe := makeHelloForeverExe(t)
	defer os.RemoveAll(tmpDir)
	taskConf := svcutil.TaskConfig{
		Path:             testExe,
		GracefulShutDown: 10 * time.Second,
	}
	start := time.Now()

	terminate := make(chan struct{})

	// Act
	go func() {
		time.Sleep(shutdownIn)
		close(terminate)
	}()
	svcutil.ExecuteTask(terminate, taskConf)

	// Assert
	elapsed := time.Since(start)
	if elapsed > shutdownIn+taskConf.GracefulShutDown {
		t.Fatalf("Elapse time longer than expected.  Took: %v", elapsed)
	}

	if elapsed < shutdownIn {
		t.Fatalf("Did not expect task to run shorter than our timeout")
	}
}

func Test_ExecuteTask_GracefulShutDown_HardKill(t *testing.T) {
	// Arrange
	const shutdownIn = 1 * time.Second
	const gracefulTime = 5 * time.Second
	tmpDir, testExe := makeHelloForeverNoShutdownExe(t)
	defer os.RemoveAll(tmpDir)
	taskConf := svcutil.TaskConfig{
		Path:             testExe,
		GracefulShutDown: gracefulTime,
	}
	start := time.Now()

	terminate := make(chan struct{})

	// Act
	go func() {
		time.Sleep(shutdownIn)
		close(terminate)
	}()
	svcutil.ExecuteTask(terminate, taskConf)

	// Assert
	elapsed := time.Since(start)
	threshold := time.Duration(500 * time.Millisecond)
	if elapsed > shutdownIn+gracefulTime+threshold {
		t.Fatalf("Elapse time longer than expected.  Took: %v", elapsed)
	}

	if elapsed < shutdownIn+gracefulTime {
		t.Fatalf("Did not expect task to run shorter than our timeout. Took: %v", elapsed)
	}
}

func Test_ExecuteTask_Logger(t *testing.T) {
	// Arrange
	tmpDir, testExe := makeHelloForeverExe(t)
	defer os.RemoveAll(tmpDir)

	var logBuf bytes.Buffer

	taskConf := svcutil.TaskConfig{
		Path:             testExe,
		ExecTimeout:      2 * time.Second,
		GracefulShutDown: 1 * time.Second,
		Logger:           log.New(&logBuf, "", 0),
	}

	// Act
	svcutil.ExecuteTask(nil, taskConf)

	// Assert
	output := logBuf.String()
	if len(output) == 0 {
		t.Fatalf("Expected logging output")
	}

	if !strings.Contains(output, "Hello World") {
		t.Errorf("Expected 'Hello World' in logging output")
	}
}

func Test_ExecuteTask_ConsoleLogger(t *testing.T) {
	// Arrange
	tmpDir, testExe := makeHelloForeverExe(t)
	defer os.RemoveAll(tmpDir)

	var logBuf bytes.Buffer
	var errorlogBuf bytes.Buffer

	taskConf := svcutil.TaskConfig{
		Path:             testExe,
		ExecTimeout:      2 * time.Second,
		GracefulShutDown: 1 * time.Second,
		Logger:           log.New(&logBuf, "", 0),
		ErrorLogger:      log.New(&errorlogBuf, "", 0),
	}

	// Act
	svcutil.ExecuteTask(nil, taskConf)

	// Assert
	output := logBuf.String()
	if len(output) == 0 {
		t.Fatalf("Expected some logging output")
	}

	if !strings.Contains(output, "STDOUT|Hello World") {
		t.Errorf("Expected 'STDOUT|Hello World' in logging output: %s", output)
	}
	if strings.Contains(output, "STDERR|Sending an error to the world:") {
		t.Errorf("Did not expect 'STDERR|Sending an error to the world:' in logging output: %s", output)
	}

	erroroutput := errorlogBuf.String()
	if len(erroroutput) == 0 {
		t.Fatalf("Expected some errorlogging output")
	}

	if !strings.Contains(erroroutput, "STDERR|Sending an error to the world:") {
		t.Errorf("Expected 'STDERR|Sending an error to the world:' in error logging output: %s", erroroutput)
	}
	if strings.Contains(erroroutput, "Hello World") {
		t.Errorf("Did not expect 'Hello World' in error logging output: %s", erroroutput)
	}
}

func Test_ExecuteService_CrashConfig_RestartDelay(t *testing.T) {
	// Arrange
	const shutdownIn = 3 * time.Second
	tmpDir, testExe := makeCrashExe(t)
	defer os.RemoveAll(tmpDir)

	var logBuf bytes.Buffer

	serviceConf := svcutil.ServiceConfig{
		Path:   testExe,
		Logger: log.New(&logBuf, "", 0),
		CrashConfig: svcutil.CrashConfig{
			RestartDelay: 2 * time.Second,
		},
	}

	terminate := make(chan struct{})

	// Act
	go func() {
		time.Sleep(shutdownIn)
		close(terminate)
	}()
	svcutil.ExecuteService(terminate, serviceConf)

	// Assert
	output := logBuf.String()
	if len(output) == 0 {
		t.Fatalf("Expected logging output")
	}
	// Find "CRASHED" twice
	crashedTimes := len(regexp.MustCompile("CRASHED").FindAllString(output, -1))
	if crashedTimes != 2 {
		t.Errorf("Expected it to crash twice.  Got: %v", crashedTimes)
	}
}

func Test_ExecuteService_CrashConfig_MaxCount(t *testing.T) {
	// Arrange
	tmpDir, testExe := makeCrashExe(t)
	defer os.RemoveAll(tmpDir)

	var logBuf bytes.Buffer

	serviceConf := svcutil.ServiceConfig{
		Path:   testExe,
		Logger: log.New(&logBuf, "", 0),
		CrashConfig: svcutil.CrashConfig{
			MaxCountPerHour: 5,
		},
	}

	// Act
	err := svcutil.ExecuteService(nil, serviceConf)

	// Assert
	if err == nil {
		t.Errorf("Expected error")
	}

	output := logBuf.String()
	if len(output) == 0 {
		t.Fatalf("Expected logging output")
	}
	// Find "CRASHED" twice
	crashedTimes := len(regexp.MustCompile("CRASHED").FindAllString(output, -1))
	if crashedTimes != 5 {
		t.Errorf("Expected it to crash 5 times.  Got: %v", crashedTimes)
	}
}

func makeHelloWorldExe(t *testing.T) (tmpDir, testExe string) {
	_, thisFile, _, _ := runtime.Caller(0)
	src := path.Dir(thisFile) + "/testexes/helloworld.go"
	return makeTestExe(t, src)
}

func makeHelloForeverExe(t *testing.T) (tmpDir, testExe string) {
	_, thisFile, _, _ := runtime.Caller(0)
	src := path.Dir(thisFile) + "/testexes/helloforever.go"
	return makeTestExe(t, src)
}

func makeHelloForeverNoShutdownExe(t *testing.T) (tmpDir, testExe string) {
	_, thisFile, _, _ := runtime.Caller(0)
	src := path.Dir(thisFile) + "/testexes/helloforever-no-shutdown.go"
	return makeTestExe(t, src)
}

func makeCrashExe(t *testing.T) (tmpDir, testExe string) {
	_, thisFile, _, _ := runtime.Caller(0)
	src := path.Dir(thisFile) + "/testexes/crash.go"
	return makeTestExe(t, src)
}

func makeTestExe(t *testing.T, testSrc string) (tmpDir, testExe string) {
	tmpDir, err := ioutil.TempDir("", "TestSvcutil")
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
