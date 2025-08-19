// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2014-2025 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/papercutsoftware/silver/lib/jsonsig"
)

const (
	ExitSuccess = 0
	ExitError   = 1
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(ExitError)
	}

	var err error
	switch os.Args[1] {
	case "generate":
		err = generateCmd()
	case "sign":
		err = signCmd()
	case "verify":
		err = verifyCmd()
	default:
		printUsage()
		os.Exit(ExitError)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(ExitError)
	}
}

func printUsage() {
	fmt.Println("Usage: jsonsign [command] [arguments]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  generate - Generate a new key pair.")
	fmt.Println("      --public-key=<file>    File to save the public key to. (default: stdout)")
	fmt.Println("      --private-key=<file>   File to save the private key to. (default: stdout)")
	fmt.Println("")
	fmt.Println("  sign - Sign a JSON document.")
	fmt.Println("      --private-key=<file>   File containing the private key. (required)")
	fmt.Println("      --input=<file>         File to read the JSON document from. (default: stdin)")
	fmt.Println("      --output=<file>        File to write the signed JSON document to. (default: stdout)")
	fmt.Println("")
	fmt.Println("  verify - Verify a signed JSON document.")
	fmt.Println("      --public-key=<file>    File containing the public key. (required)")
	fmt.Println("      --input=<file>         File to read the signed JSON document from. (default: stdin)")
}

func generateCmd() error {
	generateCmd := flag.NewFlagSet("generate", flag.ContinueOnError)
	publicKeyFile := generateCmd.String("public-key", "", "The file to save the public key to.")
	privateKeyFile := generateCmd.String("private-key", "", "The file to save the private key to.")

	if err := generateCmd.Parse(os.Args[2:]); err != nil {
		return err
	}

	publicKey, privateKey, err := jsonsig.GenerateKeys()
	if err != nil {
		return fmt.Errorf("generating keys: %w", err)
	}

	if *publicKeyFile != "" {
		if err := os.WriteFile(*publicKeyFile, []byte(publicKey), 0644); err != nil {
			return fmt.Errorf("writing public key to file: %w", err)
		}
	} else {
		fmt.Println("Public Key:")
		fmt.Println(publicKey)
		fmt.Println("")
	}

	if *privateKeyFile != "" {
		if err := os.WriteFile(*privateKeyFile, []byte(privateKey), 0600); err != nil {
			return fmt.Errorf("writing private key to file: %w", err)
		}
	} else {
		fmt.Println("Private Key:")
		fmt.Println(privateKey)
	}
	return nil
}

func signCmd() error {
	signCmd := flag.NewFlagSet("sign", flag.ContinueOnError)
	privateKeyFile := signCmd.String("private-key", "", "The file containing the private key.")
	inputFile := signCmd.String("input", "", "The file to read the JSON document from. If not specified, reads from standard input.")
	outputFile := signCmd.String("output", "", "The file to write the signed JSON document to. If not specified, writes to standard output.")

	if err := signCmd.Parse(os.Args[2:]); err != nil {
		return err
	}

	if *privateKeyFile == "" {
		signCmd.Usage()
		return fmt.Errorf("--private-key is required")
	}

	privateKeyBytes, err := os.ReadFile(*privateKeyFile)
	if err != nil {
		return fmt.Errorf("reading private key: %w", err)
	}

	var inputBytes []byte
	if *inputFile != "" {
		inputBytes, err = os.ReadFile(*inputFile)
		if err != nil {
			return fmt.Errorf("reading input file: %w", err)
		}
	} else {
		inputBytes, err = io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("reading from stdin: %w", err)
		}
	}

	signedPayload, err := jsonsig.Sign(inputBytes, string(privateKeyBytes))
	if err != nil {
		return fmt.Errorf("signing payload: %w", err)
	}

	if *outputFile != "" {
		if err := os.WriteFile(*outputFile, signedPayload, 0644); err != nil {
			return fmt.Errorf("writing output file: %w", err)
		}
	} else {
		fmt.Println(string(signedPayload))
	}
	return nil
}

func verifyCmd() error {
	verifyCmd := flag.NewFlagSet("verify", flag.ContinueOnError)
	publicKeyFile := verifyCmd.String("public-key", "", "The file containing the public key.")
	inputFile := verifyCmd.String("input", "", "The file to read the signed JSON document from. If not specified, reads from standard input.")

	if err := verifyCmd.Parse(os.Args[2:]); err != nil {
		return err
	}

	if *publicKeyFile == "" {
		verifyCmd.Usage()
		return fmt.Errorf("--public-key is required")
	}

	publicKeyBytes, err := os.ReadFile(*publicKeyFile)
	if err != nil {
		return fmt.Errorf("reading public key: %w", err)
	}

	var inputBytes []byte
	if *inputFile != "" {
		inputBytes, err = os.ReadFile(*inputFile)
		if err != nil {
			return fmt.Errorf("reading input file: %w", err)
		}
	} else {
		inputBytes, err = io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("reading from stdin: %w", err)
		}
	}

	_, err = jsonsig.Verify(inputBytes, string(publicKeyBytes))
	if err != nil {
		return err
	}

	fmt.Println("Verification successful!")
	return nil
}
