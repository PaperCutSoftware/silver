// SILVER - Service Wrapper
//
// Copyright (c) 2016 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

// +build ignore

package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) > 1 {
		if strings.HasPrefix(os.Args[1], "ERROR") {
			fmt.Fprintf(os.Stderr, "Hello %s!\n", os.Args[1])
			os.Exit(1)
		} else {
			fmt.Printf("Hello %s!\n", os.Args[1])
		}
	} else {
		fmt.Println("Hello World!")
	}
}
