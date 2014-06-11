// SILVER - Service Wrapper
//
// Copyright (c) 2014 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//
package pathutils

import (
	"path/filepath"
	"sort"
)

func FindLastFile(pattern string) (match string) {

	var matches []string
	var err error
	matches, err = filepath.Glob(pattern)
	if err != nil {
		return pattern
	}
	if len(matches) < 1 {
		return pattern
	}
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))
	match = matches[0]
	return
}
