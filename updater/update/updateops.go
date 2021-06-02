// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2014-2021 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package update

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/papercutsoftware/silver/lib/osutils"
	"github.com/papercutsoftware/silver/lib/pathutils"
)

func ExecOp(args []string) (err error) {
	if len(args) < 1 {
		return errors.New("Invalid exec operation format - arg expected.")
	}
	cmd := args[0]
	fmt.Printf("Running install command: %s\n", strings.Join(args, " "))
	_ = os.Chmod(cmd, 0755)
	c := exec.Command(cmd, args[1:]...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	err = c.Run()
	return err
}

func BatchRenameOp(args []string) error {
	if len(args) != 3 {
		return errors.New("Invalid rename operation format - three args expected.")
	}
	root := args[0]
	find := args[1]
	replacement := args[2]
	fmt.Printf("Running batch rename operation on root %s ('%s' => '%s')\n", root, find, replacement)
	return batchRename(root, find, replacement)
}

func batchRename(root, find, replacement string) error {
	matches, err := filepath.Glob(root)
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		return nil
	}

	re, err := regexp.Compile(find)
	if err != nil {
		return err
	}

	renameCnt := 0
	visitFn := func(path string, fi os.FileInfo, errin error) error {
		name := fi.Name()
		newName := re.ReplaceAllString(name, replacement)
		if name == newName {
			return nil
		}
		if !osutils.FileExists(path) {
			return nil
		}

		newPath := filepath.Join(filepath.Dir(path), newName)
		fmt.Printf("Renaming '%s' to '%s' ...\n", path, newPath)
		err := os.Rename(path, newPath)
		if err != nil {
			return err
		}
		renameCnt++
		return nil
	}

	for _, match := range matches {
		err := filepath.Walk(match, visitFn)
		if err != nil {
			return err
		}
	}
	fmt.Printf("Renamed %d files.\n", renameCnt)
	return nil
}

func MoveOp(args []string) error {
	if len(args) != 2 {
		return errors.New("Invalid copy operation format - two args expected.")
	}
	src := pathutils.FindLastFile(args[0])
	fmt.Printf("Moving '%s' to '%s'...\n", src, args[1])
	return os.Rename(src, args[1])
}

func RemoveOp(args []string) error {
	if len(args) != 1 {
		return errors.New("Invalid remove operation format - one arg file expected.")
	}
	path := args[0]
	matches, err := filepath.Glob(path)
	if err != nil {
		return err
	}
	removeCnt := 0
	for _, match := range matches {
		fmt.Printf("Removing '%s' ...\n", match)
		if err := os.RemoveAll(match); err != nil {
			// Don't exit... best effort
			fmt.Printf("Problem removing %s: %v\n", match, err)
		}
		removeCnt++
	}
	fmt.Printf("Removed %d files.\n", removeCnt)
	return nil
}

func CopyOp(args []string) error {
	if len(args) != 2 {
		return errors.New("Invalid copy operation format - two args expected.")
	}
	src := pathutils.FindLastFile(args[0])
	fmt.Printf("Copying '%s' to '%s'...\n", src, args[1])
	return osutils.CopyFile(src, args[1])
}
