// +build ignore

// SILVER - Service Wrapper
//
// Copyright (c) 2014 PaperCut Software http://www.papercut.com/
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
//   Setup environment (install godep):
//     $ go run make.go setup
//
// Alternatively you may use "go get" to load the code into your default
// workspace and play from there.
//
// Concepts loosly based on concepts in Camlistore
//     https://github.com/bradfitz/camlistore
//
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	rootNamespace = "github.com/papercutsoftware/silver"
)

var (
	// The project root where this file is located
	projectRoot string
	//
	buildGoPath    string
	buildOutputDir string
)

func usage() {
	fmt.Println("Usage: go run make.go [command]")
	fmt.Println("    all")
	fmt.Println("    test")
	fmt.Println("    setup\n")
	os.Exit(1)
}

func main() {

	var err error
	projectRoot, err = os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("Failed to get current directory: %v\n", err))
	}
	buildGoPath = filepath.Join(projectRoot, ".gopath")
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
	case "setup":
		setupEnv()
	default:
		usage()
	}
}

func verifyEnv() {
	if _, err := exec.LookPath("godep"); err != nil {
		fmt.Println("Please install godep. Run:")
		fmt.Println("   go make.go setup")
	}
}

func setupEnv() {
	fmt.Println("Installing godep...")
	runCmd("go", "get", "github.com/tools/godep")
	if _, err := exec.LookPath("godep"); err != nil {
		fmt.Println("ERROR: godep does note seem to be on the path")
	}
}

func buildAll() {
	verifyEnv()
	setupBuildGoPath()
	makeDir(buildOutputDir)

	fmt.Printf("Building binaries for %s...\n", runtime.GOOS)
	runCmd("godep", "go", "build", "-o", makeOutputPath(buildOutputDir, "updater"), rootNamespace+"/updater")
	runCmd("godep", "go", "build", "-o", makeOutputPath(buildOutputDir, "service"), rootNamespace+"/service")
	runCmd("godep", "go", "build", "-tags", "nohttp", "-o", makeOutputPath(buildOutputDir, "service-no-http"), rootNamespace+"/service")
	if runtime.GOOS == "windows" {
		runCmd("godep", "go", "build", "-tags", "nohttp", "-ldflags", "-H=windowsgui", "-o", makeOutputPath(buildOutputDir, "service-no-window"), rootNamespace+"/service")
		runCmd("godep", "go", "build", "-ldflags", "-H=windowsgui", "-o", makeOutputPath(buildOutputDir, "updater-no-window"), rootNamespace+"/updater")
	}

	fmt.Printf("\nCOMPLETE. You'll find the files in:\n    '%s'\n", buildOutputDir)

}

func testAll() {
	verifyEnv()
	setupBuildGoPath()
	runCmd("godep", "go", "test", rootNamespace+"/...")
}

func runCmd(cmd string, arg ...string) error {
	c := exec.Command(cmd, arg...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("Error running command %s: %v", cmd, err)
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

func setupBuildGoPath() {

	// .gopath/src/github.com/papercut/silver
	goPathSrcDir := filepath.Join(buildGoPath, "src", rootNamespace)
	makeDir(goPathSrcDir)

	goDirs := []string{
		"lib",
		"service",
		"updater",
	}

	for _, dir := range goDirs {
		orig := filepath.Join(projectRoot, filepath.FromSlash(dir))
		dest := filepath.Join(goPathSrcDir, filepath.FromSlash(dir))
		if _, err := mirrorDir(orig, dest); err != nil {
			panic(fmt.Sprintf("Error while mirroring %s to %s: %v", orig, dest, err))
		}
	}

	os.Setenv("GOPATH", buildGoPath)
}

func mirrorDir(src, dst string) (maxMod time.Time, err error) {
	err = filepath.Walk(src, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		suffix, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("Failed to find Rel(%q, %q): %v", src, path, err)
		}
		if t := fi.ModTime(); t.After(maxMod) {
			maxMod = t
		}
		return mirrorFile(path, filepath.Join(dst, suffix))
	})
	return
}

func isExecMode(mode os.FileMode) bool {
	return (mode & 0111) != 0
}

func mirrorFile(src, dst string) error {
	sfi, err := os.Stat(src)
	if err != nil {
		return err
	}
	if sfi.Mode()&os.ModeType != 0 {
		panic(fmt.Sprintf("mirrorFile can't deal with non-regular file %s", src))
	}
	dfi, err := os.Stat(dst)
	if err == nil &&
		isExecMode(sfi.Mode()) == isExecMode(dfi.Mode()) &&
		(dfi.Mode()&os.ModeType == 0) &&
		dfi.Size() == sfi.Size() &&
		dfi.ModTime().Unix() == sfi.ModTime().Unix() {
		// Seems to not be modified.
		return nil
	}

	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	df, err := os.Create(dst)
	if err != nil {
		return err
	}
	sf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sf.Close()

	n, err := io.Copy(df, sf)
	if err == nil && n != sfi.Size() {
		err = fmt.Errorf("copied wrong size for %s -> %s: copied %d; want %d", src, dst, n, sfi.Size())
	}
	cerr := df.Close()
	if err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(dst, sfi.Mode())
	}
	if err == nil {
		err = os.Chtimes(dst, sfi.ModTime(), sfi.ModTime())
	}
	return err
}
