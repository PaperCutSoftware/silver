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
	"io"
	"net"
	"os"
	"strconv"
	"time"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("No fail time on arg 1!")
		os.Exit(1)
	}
	fmt.Println("Starting...")

	i, _ := strconv.Atoi(os.Args[1])
	failAt := time.Now().Add(time.Duration(i) * time.Second)

	ln, err := net.Listen("tcp", ":4300")
	if err != nil {
		panic(err)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		if time.Now().Before(failAt) {
			go func(conn net.Conn) {
				io.Copy(conn, conn)
				conn.Close()
			}(conn)
		} else {
			fmt.Println("Failing!")
			// Hang!
			time.Sleep(1 * time.Hour)
		}
	}
}
