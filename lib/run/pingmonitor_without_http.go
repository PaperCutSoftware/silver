// SILVER - Service Wrapper
//
// Copyright (c) 2014 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//
// +build nohttp

package run

import (
	"errors"
	"time"
)

func pingHTTP(pingUrl string, timeout time.Duration) (ok bool, err error) {
	return true, errors.New("HTTP monitoring is not supported in this version. Use the full version.")
}
