package cmdutil

import (
	"time"

	"github.com/papercutsoftware/silver/lib/procmngt"
)

type CommandConfig struct {
	Path        string
	Args        []string
	ExecTimeout time.Duration
}

func Execute(cmdConf CommandConfig) (exitCode int, err error) {
	execConf := procmngt.ExecConfig{Path: cmdConf.Path, Args: cmdConf.Args, ExecTimeout: cmdConf.ExecTimeout}
	executable := procmngt.NewExecutable(execConf)
	return executable.Execute(nil)
}
