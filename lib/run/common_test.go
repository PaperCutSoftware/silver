// SILVER - Service Wrapper
//
// Copyright (c) 2014 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//
package run

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"testing"
	"time"
)

func helperArgs(s ...string) (args []string) {
	args = []string{"-test.run=TestHelperProcess", "--"}
	args = append(args, s...)
	return args
}

func sigExitHandler(timeBeforeExit int) {
	c := make(chan os.Signal, 10)
	signal.Notify(c)
	select {
	case s := <-c:
		if s == os.Interrupt {
			time.Sleep(time.Duration(timeBeforeExit) * time.Second)
			os.Exit(0)
		}
	}
}

// TestHelperProcess isn't a real test. It's used as a helper process.
func TestHelperProcess(*testing.T) {
	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		// Not a command
		return
	}

	cmd, args := args[0], args[1:]
	switch cmd {
	case "sleep-for":
		i, err := strconv.Atoi(args[0])
		if err == nil {
			time.Sleep(time.Duration(i) * time.Second)
			os.Exit(0)
		}
		os.Exit(2)
	case "crash-in":
		i, err := strconv.Atoi(args[0])
		if err == nil {
			time.Sleep(time.Duration(i) * time.Second)
			os.Exit(123)
		}
		os.Exit(2)
	case "work-before-exit":
		i, err := strconv.Atoi(args[0])
		if err == nil {
			go sigExitHandler(i)
		} else {
			os.Exit(2)
		}
		time.Sleep(9999 * time.Second)
		os.Exit(123)
	case "echo-ping-fail-after":
		i, _ := strconv.Atoi(args[0])
		failAt := time.Now().Add(time.Duration(i) * time.Second)

		go sigExitHandler(0)
		ln, err := net.Listen("tcp", "127.0.0.1:4300")
		if err != nil {
			panic(err)
		}
		for {
			if time.Now().Before(failAt) {
				conn, err := ln.Accept()
				if err != nil {
					panic(err)
				}
				go func(conn net.Conn) {
					if _, err := io.Copy(conn, conn); err != nil {
						panic(err)
					}
					conn.Close()
				}(conn)
			} else {
				// Die
				time.Sleep(20 * time.Second)
			}
		}
	case "tcp-ping-fail-after":
		i, _ := strconv.Atoi(args[0])
		failAt := time.Now().Add(time.Duration(i) * time.Second)

		go sigExitHandler(0)
		ln, err := net.Listen("tcp", "127.0.0.1:4300")
		if err != nil {
			panic(err)
		}
		for {
			if time.Now().Before(failAt) {
				conn, err := ln.Accept()
				if err != nil {
					panic(err)
				}
				go func(conn net.Conn) {
					conn.Close()
				}(conn)
			} else {
				// Die
				ln.Close()
				break
			}
		}
		time.Sleep(90 * time.Second)
	case "http-ping-fail-after":
		i, _ := strconv.Atoi(args[0])
		failAt := time.Now().Add(time.Duration(i) * time.Second)

		//go sigExitHandler(0)
		http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
			if time.Now().Before(failAt) {
				fmt.Fprintf(w, "Working!")
			} else {
				http.Error(w, "Not working!", http.StatusInternalServerError)
			}
		})
		http.ListenAndServe("127.0.0.1:4300", nil)
	case "file-ping-fail-after":
		pingFile := "status.file"
		i, _ := strconv.Atoi(args[0])
		failAt := time.Now().Add(time.Duration(i) * time.Second)

		for {
			if time.Now().Before(failAt) {
				d := fmt.Sprintf("fake-change-%d", time.Now().Unix())
				ioutil.WriteFile(pingFile, []byte(d), 0644)
				time.Sleep(500 * time.Millisecond)
			} else {
				time.Sleep(60 * time.Second)
			}
		}
	}
}
