// +build ignore

// SILVER - Service Wrapper
//
// Copyright (c) 2014-2021 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

// This Go make file builds Silver directly from a code checkout, bypassing
// the need to configure/setup a Go workspace.
//
// Run on the command line with:
//     $ go run make.go
//
// Other options:
//   Run tests:
//     $ go run make.go test
//
// Concepts loosly based on concepts in Camlistore
//     https://github.com/bradfitz/camlistore
//
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	rootNamespace = "github.com/papercutsoftware/silver"
)

var (
	// The project root where this file is located
	projectRoot string
	//
	buildOutputDir string
)

func usage() {
	fmt.Println("Usage: go run make.go [command]")
	fmt.Println("    all")
	fmt.Println("    test")
	os.Exit(1)
}

func main() {
	_ = os.Setenv("GOFLAGS", "-mod=vendor")

	var err error
	projectRoot, err = os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("Failed to get current directory: %v\n", err))
	}
	buildOutputDir = filepath.Join(projectRoot, "build", runtime.GOOS)

	action := "all"
	if len(os.Args) > 1 {
		action = os.Args[1]
	}

	switch action {
	case "all":
		buildAll()
	case "test":
		testAll()
	default:
		usage()
	}
}

func buildAll() {
	makeDir(buildOutputDir)

	fmt.Printf("Building binaries for %s...\n", runtime.GOOS)
	_ = runCmd("go", "build", "-ldflags", "-s -w", "-o", makeOutputPath(buildOutputDir, "updater"), rootNamespace+"/updater")
	_ = runCmd("go", "build", "-ldflags", "-s -w", "-o", makeOutputPath(buildOutputDir, "service"), rootNamespace+"/service")
	_ = runCmd("go", "build", "-tags", "nohttp", "-ldflags", "-s -w", "-o", makeOutputPath(buildOutputDir, "service-no-http"), rootNamespace+"/service")
	if runtime.GOOS == "windows" {
		_ = runCmd("go", "build", "-tags", "nohttp", "-ldflags", "-s -w  -H=windowsgui", "-o", makeOutputPath(buildOutputDir, "service-no-window"), rootNamespace+"/service")
		_ = runCmd("go", "build", "-ldflags", "-s -w -H=windowsgui", "-o", makeOutputPath(buildOutputDir, "updater-no-window"), rootNamespace+"/updater")
	}

	fmt.Printf("\nCOMPLETE. You'll find the files in:\n    '%s'\n", buildOutputDir)

}

func testAll() {
	_ = runCmd("go", "test", rootNamespace+"/...")
}

func runCmd(cmd string, arg ...string) error {
	c := exec.Command(cmd, arg...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("error running command %s: %v", cmd, err)
	}
	return nil
}

func makeDir(dir string) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		panic(err)
	}
}

func makeOutputPath(dir, name string) string {
	if runtime.GOOS == "windows" {
		if !strings.HasSuffix(name, ".exe") {
			name = name + ".exe"
		}
	}
	return filepath.Join(dir, name)
}
