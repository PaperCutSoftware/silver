// SILVER - Service Wrapper
//
// Copyright (c) 2016 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

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
