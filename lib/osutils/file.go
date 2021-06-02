// SILVER - Service Wrapper
//
// Copyright (c) 2021 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package osutils

import (
	"io"
	"io/ioutil"
	"os"
	"strings"
)

// FileExists check if a file denoted by path exists, returning true or false.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// CopyFile copies a file from src to dst
func CopyFile(src, dest string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = s.Close() }()
	d, err := os.Create(dest)
	if err != nil {
		return err
	}
	if _, err := io.Copy(d, s); err != nil {
		_ = d.Close()
		return err
	}
	return d.Close()
}

// WriteFileString writes a string to file denoted by filename with specified permissions.
func WriteFileString(filename string, data string, perm os.FileMode) error {
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	_, err = f.WriteString(data)
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return err
}

// ReadStringFromFile reads a text file and returns contents as string or fall back to default as per def on error
func ReadStringFromFile(file string, def string) string {
	if dat, err := ioutil.ReadFile(file); err == nil {
		def = strings.TrimSpace(string(dat))
	}
	return def
}
