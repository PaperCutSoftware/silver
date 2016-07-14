package procmngt_test

import (
	"testing"
	"time"
)

func Test_CompleteExample(t *testing.T) {

	_, thisFile, _, _ := runtime.Caller(0)
	helloWorldGo := path.Dir(thisFile) + "/testexes/helloworld.go"

	tmpDir, testExe = makeTestExe(t, helloWorldGo)
	defer os.RemoveAll(tmpDir)

	execConf := procmgmt.ExecConfig{
		Path:         testExe,
		Args:         []string{"go test"},
		StartupDelay: 1 * time.Second,
		Stdin: nil,
		Stdout: os.Stdout,
		Stderr os.Stderr,
	}
	executable := procmgmt.NewExecutable(execConf)

	executable.Stdout = myfile

	executable.Execute(nil)

}

func Test_FixedDelayedStartupCommand(t *testing.T) {
	// Arrange
	delayed := time.Duration(1 * time.Second)
	execConf := procmgmt.ExecConfig{
		Path:         `c:\windows\System32\ping.exe`,
		Args:         []string{"-n", "1", "localhost"},
		StartupDelay: delayed,
	}
	executable := procmgmt.NewExecutable(execConf)
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
	timeout := time.Duration(1 * time.Second)
	execConf := procmgmt.ExecConfig{
		Path:        `c:\windows\System32\ping.exe`,
		Args:        []string{"-n", "10", "localhost"},
		ExecTimeout: timeout,
	}
	executable := procmgmt.NewExecutable(execConf)
	start := time.Now()

	// Act
	executable.Execute(nil)

	// Assert
	elapsed := time.Since(start)
	if elapsed < timeout {
		t.Fatalf("The command exit before the timeout")
	}
	threshold := time.Duration(500 * time.Millisecond)
	if elapsed > timeout+threshold {
		t.Fatalf("Timeout is not enforced")
	}

}

func Test_StartupDelayedAndTimedOutCommand(t *testing.T) {
	// Arrange
	delayed := time.Duration(1 * time.Second)
	randDelayed := time.Duration(2 * time.Second)
	timeout := time.Duration(1 * time.Second)
	execConf := procmgmt.ExecConfig{
		Path:               `c:\windows\System32\ping.exe`,
		Args:               []string{"-n", "1", "localhost"},
		ExecTimeout:        timeout,
		StartupDelay:       delayed,
		StartupRandomDelay: randDelayed,
	}
	executable := procmgmt.NewExecutable(execConf)
	start := time.Now()

	// Act
	executable.Execute(nil)

	// Assert
	elapsed := time.Since(start)
	threshold := time.Duration(500 * time.Millisecond) // the command should take less than threshold to run
	if elapsed >= timeout+delayed+randDelayed+threshold {
		t.Fatalf("The command took longer than expected")
	}
}

// this test is commented out. Wait for Chris to fix the ProcessKillGracefully
//func Test_GracefulShutDownCommand(t *testing.T) {
//	// Arrange
//	execConf := ExecConfig{
//		Path:             `c:\windows\System32\ping.exe`,
//		Args:             []string{"-n", "20", "localhost"},
//		GracefulShutDown: time.Duration(1 * time.Second),
//	}
//	executable := NewExecutable(execConf)
//	start := time.Now()
//	terminate := make(chan struct{})
//
//	// Act
//	executable.Execute(terminate)
//	go func() {
//		select {
//		case <-time.After(time.Duration(2 * time.Second)):
//			close(terminate)
//		}
//	}()
//
//	// Assert
//	elapsed := time.Since(start)
//	threshold := time.Duration(500 * time.Millisecond) // the command should take less than threshold to run
//	if elapsed > time.Duration(3 * time.Second) + threshold {
//		t.Fatalf("The command was not shut down")
//	}
//}


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


