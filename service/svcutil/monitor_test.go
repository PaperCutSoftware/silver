// SILVER - Service Wrapper
//
// Copyright (c) 2016 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package svcutil_test

import (
	"os"
	"path"
	"runtime"
	"testing"
	"time"

	"github.com/papercutsoftware/silver/service/svcutil"
)

func Test_ExecuteService_MonitorConfig_EchoPing(t *testing.T) {
	// Arrange
	tmpDir, testExe := makeEchoPingFailExe(t)
	defer os.RemoveAll(tmpDir)

	serviceConf := svcutil.ServiceConfig{
		Path: testExe,
		Args: []string{"5"},
		//Logger: log.New(os.Stderr, "", 0),
		MonitorConfig: svcutil.MonitorConfig{
			URL:          "echo://localhost:4300",
			StartupDelay: 3 * time.Second,
			Interval:     500 * time.Millisecond,
			Timeout:      1 * time.Second,
		},
	}
	start := time.Now()

	// Act
	svcutil.ExecuteService(nil, serviceConf)

	// Assert
	elapsed := time.Since(start)
	expected := 4500 * time.Millisecond //3500 +  1000 (for go routine to start)
	threshold := 3500 * time.Millisecond //to allow for some variance based on machine speed
	if elapsed > expected+threshold {
		t.Fatalf("Elapse time longer than expected.  Took: %v", elapsed)
	}

	if elapsed < expected-threshold {
		t.Fatalf("Did not expect task to run shorter than our timeout. Took: %v", elapsed)
	}
}

func makeEchoPingFailExe(t *testing.T) (tmpDir, testExe string) {
	_, thisFile, _, _ := runtime.Caller(0)
	src := path.Dir(thisFile) + "/testexes/echo-ping-fail.go"
	return makeTestExe(t, src)
}

/*
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
*/
