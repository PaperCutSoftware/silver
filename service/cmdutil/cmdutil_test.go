// SILVER - Service Wrapper
//
// Copyright (c) 2016, 2017 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package cmdutil

import (
	"runtime"
	"testing"
)

func Test_Execute_ShouldExecuteNormalCommandProperly(t *testing.T) {
	// Arrange
	exec := "ping"
	args := []string{"-c", "1", "localhost"}
	if runtime.GOOS == "windows" {
		args = []string{"-n", "1", "localhost"}
	}

	// Act
	exitCode, err := Execute(CommandConfig{Path: exec, Args: args})

	// Assert
	if err != nil {
		t.Fatalf("The command should exit normally")
	}

	if exitCode != 0 {
		t.Fatalf("The exit code should be 0")
	}
}
