// SILVER - Service Wrapper
//
// Copyright (c) 2014 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

// +build !windows

package run

import "os/exec"

func setProcAttributes(cmd *exec.Cmd) {
	// None required
}
