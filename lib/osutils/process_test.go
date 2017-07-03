// SILVER - Service Wrapper
//
// Copyright (c) 2016, 2017 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package osutils_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/papercutsoftware/silver/lib/osutils"
)

func Test_ProcessKillGracefully_ConsoleProgram(t *testing.T) {
	tmpDir, testExe := makeTestCommand(t)
	defer cleanupTestCommand(t, tmpDir)
	testProcessKillGracefully(testExe, nil, t)
}

func Test_ProcessKillGracefully_GUIProgram(t *testing.T) {
	var testCmd string
	var args []string

	if runtime.GOOS == "windows" {
		testCmd = `c:\Windows\notepad.exe`
	} else {
		testCmd = "ping"
		args = []string{"localhost"}
	}
	testProcessKillGracefully(testCmd, args, t)
}

func testProcessKillGracefully(command string, args []string, t *testing.T) {
	t.Logf("Starting %v", command)
	cmd := exec.Command(command, args...)
	cmd.SysProcAttr = osutils.ProcessSysProcAttrForQuit()
	err := cmd.Start()
	// Give time to open
	time.Sleep(1 * time.Second)
	if err != nil {
		t.Fatalf("Error starting test cmd: %v", cmd)
	}
	start := time.Now()
	go func() {
		err := cmd.Wait()
		t.Logf("Cmd complete in %v : %v", time.Now().Sub(start), err)
	}()

	// Act
	err = osutils.ProcessKillGracefully(cmd.Process.Pid, 5*time.Second)

	// Assert no error
	if err != nil {
		t.Errorf("Cmd did not exit cleanly: %v", err)
	}

	duration := time.Now().Sub(start)
	t.Logf("Exit happened in %v", duration)

	// Assert time killed within 1 second
	if duration > 1*time.Second {
		t.Errorf("Expected kill to return quicker!")
	}
	// Assert time killed larger than 500 ms
	if duration < 500*time.Millisecond {
		t.Errorf("Expected it to take > 500 ms due to check!")
	}
}

func TestProcessIsRunning_DetectsRunning(t *testing.T) {
	// Arrange
	tmpDir, testExe := makeTestCommand(t)
	defer cleanupTestCommand(t, tmpDir)
	cmd := exec.Command(testExe)

	err := cmd.Start()
	if err != nil {
		t.Fatalf("Error starting test cmd: %v", cmd)
	}
	time.Sleep(500 * time.Millisecond)

	pid := cmd.Process.Pid

	// Act
	running, err := osutils.ProcessIsRunning(pid)

	// Assert
	if err != nil {
		t.Errorf("ProcessIsRunning returned error: %v", err)
	}
	if !running {
		t.Errorf("Expected process at pid %d to be running", pid)
	}

	cmd.Process.Kill()
	cmd.Wait()
}

func TestProcessIsRunning_DetectsNotRunning(t *testing.T) {
	// Arrange
	tmpDir, testExe := makeTestCommand(t)
	defer cleanupTestCommand(t, tmpDir)

	cmd := exec.Command(testExe)
	err := cmd.Start()
	if err != nil {
		t.Fatalf("Error starting test cmd: %v", cmd)
	}
	pid := cmd.Process.Pid
	go func() {
		time.Sleep(500 * time.Millisecond)
		cmd.Process.Kill()
	}()
	cmd.Wait()

	// Act
	running, err := osutils.ProcessIsRunning(pid)

	// Assert
	if err != nil {
		t.Errorf("ProcessIsRunning returned error: %v", err)
	}
	if running {
		t.Errorf("Expected process at pid %d NOT to be running", pid)
	}
}

func makeTestCommand(t *testing.T) (tmpDir, testExe string) {
	// create source file
	const testSource = `
package main

import (
	"log"
	"os"
	"os/signal"
	"time"
)

func main() {
	c := make(chan os.Signal, 10)
	signal.Notify(c)
	select {
	case s := <-c:
		if s != os.Interrupt {
			log.Fatalf("Wrong signal received: got %q, want %q\n", s, os.Interrupt)
		}
	case <-time.After(3 * time.Second):
		log.Fatalf("Timeout waiting for signal (e.g. Ctrl-Break)\n")
	}
}
`
	tmpDir, err := ioutil.TempDir("", "TestSigProcess")
	if err != nil {
		t.Fatal("TempDir failed: ", err)
	}

	// write sigprocess.go
	name := filepath.Join(tmpDir, "sigprocess")
	src := name + ".go"
	f, err := os.Create(src)
	if err != nil {
		t.Fatalf("Failed to create %v: %v", src, err)
	}
	f.Write([]byte(testSource))
	f.Close()

	// compile it
	exe := name + ".exe"
	o, err := exec.Command("go", "build", "-o", exe, src).CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to compile: %v\n%v", err, string(o))
	}
	return tmpDir, exe
}

func cleanupTestCommand(t *testing.T, tmpDir string) {
	err := os.RemoveAll(tmpDir)
	if err != nil {
		t.Errorf("Unable to cleanup: %v", err)
	}
}
