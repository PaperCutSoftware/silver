package procmngt

import (
	"testing"
	"time"
)

func Test_FixedDelayedStartupCommand(t *testing.T) {
	// Arrange
	delayed := time.Duration(1 * time.Second)
	execConf := ExecConfig{
		Path:         `c:\windows\System32\ping.exe`,
		Args:         []string{"-n", "1", "localhost"},
		StartupDelay: delayed,
	}
	executable := NewExecutable(execConf)
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
	execConf := ExecConfig{
		Path:        `c:\windows\System32\ping.exe`,
		Args:        []string{"-n", "10", "localhost"},
		ExecTimeout: timeout,
	}
	executable := NewExecutable(execConf)
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
	execConf := ExecConfig{
		Path:               `c:\windows\System32\ping.exe`,
		Args:               []string{"-n", "1", "localhost"},
		ExecTimeout:        timeout,
		StartupDelay:       delayed,
		StartupRandomDelay: randDelayed,
	}
	executable := NewExecutable(execConf)
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
