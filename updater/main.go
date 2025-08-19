// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2014-2025 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/papercutsoftware/silver/updater/update"
)

var (
	versionFile     = flag.String("f", ".version", "Set version file")
	showVersion     = flag.Bool("v", false, "Display current installed version and exit")
	overrideVersion = flag.String("c", "", "Override current installed version")
	httpProxy       = flag.String("p", "", "Set HTTP proxy in format http://server:port")
	allowHTTP       = flag.Bool("http", false, "Debug only: Support non-https for update checking")
	publicKey       = flag.String("public-key", "", "(Optional) Base64 encoded Ed25519 public key for update manifest verification.")
	allowInsecure   = flag.Bool("insecure", false, "Support insecure & self-signed certificates for update checking")
)

func usage() {
	exeName := filepath.Base(os.Args[0])
	_, _ = fmt.Fprintf(os.Stdout, "usage: %s [flags] [update url]\n", exeName)
	flag.PrintDefaults()
	_, _ = fmt.Fprintf(os.Stdout, "\nNote on manifest signing with --public-key:\n")
	_, _ = fmt.Fprintf(os.Stdout, "  The public key is used by the client to verify the update manifest signature.\n")
	_, _ = fmt.Fprintf(os.Stdout, "  The manifest is signed on the server with the corresponding private key.\n\n")
	_, _ = fmt.Fprintf(os.Stdout, "To generate or modify profile\n")
	_, _ = fmt.Fprintf(os.Stdout, "  profile-set-random-id\n")
	_, _ = fmt.Fprintf(os.Stdout, "\tGenerate a unique random id for this installation.\n")
	_, _ = fmt.Fprintf(os.Stdout, "  profile-set-id <id-string>\n")
	_, _ = fmt.Fprintf(os.Stdout, "\tUse the id-string as the unique identity.\n")
	_, _ = fmt.Fprintf(os.Stdout, "  profile-set-channel <channel-string>\n")
	_, _ = fmt.Fprintf(os.Stdout, "\tUse the channel-string as the distribution channel.\n")
	os.Exit(2)
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "profile-set-random-id":
			if len(os.Args) == 2 {
				os.Exit(update.SetRandomProfileID())
			} else {
				usage()
			}
		case "profile-set-id":
			if len(os.Args) == 3 {
				os.Exit(update.SetProfileID(os.Args[2]))
			} else {
				usage()
			}
		case "profile-set-channel":
			if len(os.Args) == 3 {
				os.Exit(update.SetProfileChannel(os.Args[2]))
			} else {
				usage()
			}
		}
	}

	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		fmt.Printf("Current version: %s\n", update.ReadCurrentVersion(*versionFile))
		return
	}

	if flag.NArg() == 0 {
		usage()
	}
	checkURL := flag.Arg(0)

	if !*allowHTTP && !strings.HasPrefix(strings.ToLower(checkURL), "https") {
		_, _ = fmt.Fprintf(os.Stderr, "ERROR: The update URL must be HTTPS for security reasons!\n")
		os.Exit(1)
	}

	if *allowInsecure { // Overwrite default HTTP transport to allow insecure https certificates
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	err := setupHTTPProxy(*httpProxy)
	if err != nil {
		fmt.Printf("ERROR: Ignoring error setting up proxy: %v\n", err)
	}

	ok, err := upgradeIfRequired(checkURL, *publicKey)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	if !ok {
		fmt.Println("No upgrade required")
	} else {
		fmt.Printf("Upgrade successful at %s.\n", time.Now().Format(time.RFC822))
	}
}
