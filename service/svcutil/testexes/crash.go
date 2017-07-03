// SILVER - Service Wrapper
//
// Copyright (c) 2016 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

// +build ignore

package main

import (
	"os"
	"fmt"
	"time"
)

func main() {
	time.Sleep(100 * time.Millisecond)
	fmt.Println("CRASHED!")
	os.Exit(1)
}
