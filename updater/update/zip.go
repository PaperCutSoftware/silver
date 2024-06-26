// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2014-2021 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package update

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// ExtractZip extracts a zip from zipfile to destination specified dby dest
func ExtractZip(zipfile, dest string) error {
	r, err := zip.OpenReader(zipfile)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		if err := extractZipItem(f, dest); err != nil {
			return err
		}
	}
	return nil
}

func extractZipItem(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	path := filepath.Join(dest, f.Name)
	if f.FileInfo().IsDir() {
		if runtime.GOOS == "windows" {
			err = os.MkdirAll(path, f.Mode())
		} else {
			err = os.MkdirAll(path, f.Mode()|0111)
		}
		if err != nil {
			return err
		}
	} else {
		f, err := os.OpenFile(
			path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(f, rc)
		if err != nil {
			return err
		}
	}
	return nil
}
