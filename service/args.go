package main

import (
	"errors"
	"strings"
)

func parse(args []string) (action string, actionArgs []string, err error) {
	args = normalizeArgs(args)

	if !isArgsValid(args) {
		return "", nil, errors.New("Invalid arguments")
	}

	if len(args) >= 2 {
		action = args[1]
	}
	if len(args) >= 3 {
		actionArgs = args[2:]
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

func normalizeArgs(args []string) []string {
	normalized := make([]string, len(args))
	copy(normalized, args)
	if len(out) <= 1 {
		return args
	}

	// Strip off any off the standard prefixes on first arg
	normalized[1] = strings.TrimLeft(normalized[1], "-/")

	if alias, ok := aliases[normalized[1]]; ok {
		normalized[1] = alias
	}
	return normalized
}
