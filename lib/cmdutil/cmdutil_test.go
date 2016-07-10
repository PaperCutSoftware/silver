package cmdutil

import (
	"testing"
)

func Test_Execute_ShouldExecuteNormalCommandProperly(t *testing.T) {
	// Act
	exitCode, err := Execute(CommandConfig{Path: `c:\Windows\System32\ping.exe`, Args: []string{"localhost"}})

	// Assert
	if err != nil {
		t.Fatalf("The command should exit normally")
	}

	if exitCode != 0 {
		t.Fatalf("The exit code should be 0")
	}
}
