// SILVER - Service Wrapper
//
// Copyright (c) 2016 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package cmdutil

import (
	"os"
	"time"

	"github.com/papercutsoftware/silver/lib/procmngt"
)

type CommandConfig struct {
	Path        string
	Args        []string
	ExecTimeout time.Duration
}

func Execute(cmdConf CommandConfig) (exitCode int, err error) {
	execConf := procmngt.ExecConfig{
		Path:        cmdConf.Path,
		Args:        cmdConf.Args,
		ExecTimeout: cmdConf.ExecTimeout,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		Stdin:       os.Stdin,
	}
	executable := procmngt.NewExecutable(execConf)
	return executable.Execute(nil)
}
