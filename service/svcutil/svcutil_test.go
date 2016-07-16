// SILVER - Service Wrapper
//
// Copyright (c) 2016 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package svcutil_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/papercutsoftware/silver/service/svcutil"
)

func Test_ExecuteRandomStartupDelayTask(t *testing.T) {
	// Arrange
	tmpDir, testExe := makeHelloWorld(t)
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
	if elapsed <= delay+threshold {
		t.Fatalf("The task start up delay is not enforced")
	}

	if elapsed >= delay+randomDelay+threshold {
		t.Fatalf("The task random start up delay is not enforced")
	}
}

func makeHelloWorld(t *testing.T) (tmpDir, testExe string) {
	_, thisFile, _, _ := runtime.Caller(0)
	helloWorldGo := path.Dir(thisFile) + "/testexes/helloworld.go"
	return makeTestExe(t, helloWorldGo)
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
