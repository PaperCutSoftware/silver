package main

import (
	"errors"
	"strings"
)


func parse(args []string) (action string, actionArgs []string, err error) {
	normalizeArgs(args)

	if !isArgsValid(args) {
		return "", nil, errors.New("Invalid arguments")
	}

	if len(args) >= 2 {
		action = args[2]
	}
	if len(args) >= 3 {
		actionArgs = args[3:]
	}
	return action, actionArgs, nil
}

var validArgs = []string{
	"install",
	"uninstall",
	"start",
	"stop",
	"validate",
	"run",
	"command",
}

func isArgsValid(args []string) bool {
	if len(args) < 2 {
		// No command to validate
		return true
	}

	for _, arg := range validArgs {
		if arg == args[1] {
			return true
		}
	}
	return false
}

var aliases = map[string]string{
	"setup":  "install",
	"remove": "uninstall",
	"delete": "uninstall",
	"check":  "validate",
	"test":   "validate",
}

func normalizeArgs(args []string) {
	if len(args) <= 1 {
		return
	}

	// Strip off any off the standard prefixes on first arg
	args[1] = strings.TrimLeft(args[1], "-/")

	if alias, ok := aliases[args[1]]; ok {
		args[1] = alias
	}
}
