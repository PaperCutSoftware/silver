package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/papercutsoftware/silver/lib/jsonsig"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "generate":
		generateCmd()
	case "sign":
		signCmd()
	case "verify":
		verifyCmd()
	default:
		printUsage()
		os.Exit(1)
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

func generateCmd() {
	generateCmd := flag.NewFlagSet("generate", flag.ExitOnError)
	publicKeyFile := generateCmd.String("public-key", "", "The file to save the public key to.")
	privateKeyFile := generateCmd.String("private-key", "", "The file to save the private key to.")

	generateCmd.Parse(os.Args[2:])

	publicKey, privateKey, err := jsonsig.GenerateKeys()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating keys: %v\n", err)
		os.Exit(1)
	}

	if *publicKeyFile != "" {
		if err := os.WriteFile(*publicKeyFile, []byte(publicKey), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing public key to file: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Println("Public Key:")
		fmt.Println(publicKey)
		fmt.Println("")
	}

	if *privateKeyFile != "" {
		if err := os.WriteFile(*privateKeyFile, []byte(privateKey), 0600); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing private key to file: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Println("Private Key:")
		fmt.Println(privateKey)
	}
}

func signCmd() {
	signCmd := flag.NewFlagSet("sign", flag.ExitOnError)
	privateKeyFile := signCmd.String("private-key", "", "The file containing the private key.")
	inputFile := signCmd.String("input", "", "The file to read the JSON document from. If not specified, reads from standard input.")
	outputFile := signCmd.String("output", "", "The file to write the signed JSON document to. If not specified, writes to standard output.")

	signCmd.Parse(os.Args[2:])

	if *privateKeyFile == "" {
		fmt.Fprintln(os.Stderr, "Error: --private-key is required.")
		signCmd.Usage()
		os.Exit(1)
	}

	privateKeyBytes, err := os.ReadFile(*privateKeyFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading private key: %v\n", err)
		os.Exit(1)
	}

	var inputBytes []byte
	if *inputFile != "" {
		inputBytes, err = os.ReadFile(*inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading input file: %v\n", err)
			os.Exit(1)
		}
	} else {
		inputBytes, err = io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading from stdin: %v\n", err)
			os.Exit(1)
		}
	}

	signedPayload, err := jsonsig.Sign(inputBytes, string(privateKeyBytes))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error signing payload: %v\n", err)
		os.Exit(1)
	}

	if *outputFile != "" {
		if err := os.WriteFile(*outputFile, signedPayload, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Println(string(signedPayload))
	}
}

func verifyCmd() {
	verifyCmd := flag.NewFlagSet("verify", flag.ExitOnError)
	publicKeyFile := verifyCmd.String("public-key", "", "The file containing the public key.")
	inputFile := verifyCmd.String("input", "", "The file to read the signed JSON document from. If not specified, reads from standard input.")

	verifyCmd.Parse(os.Args[2:])

	if *publicKeyFile == "" {
		fmt.Fprintln(os.Stderr, "Error: --public-key is required.")
		verifyCmd.Usage()
		os.Exit(1)
	}

	publicKeyBytes, err := os.ReadFile(*publicKeyFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading public key: %v\n", err)
		os.Exit(1)
	}

	var inputBytes []byte
	if *inputFile != "" {
		inputBytes, err = os.ReadFile(*inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading input file: %v\n", err)
			os.Exit(1)
		}
	} else {
		inputBytes, err = io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading from stdin: %v\n", err)
			os.Exit(1)
		}
	}

	valid, err := jsonsig.Verify(inputBytes, string(publicKeyBytes))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error verifying signature: %v\n", err)
		os.Exit(1)
	}

	if valid {
		fmt.Println("Verification successful!")
		os.Exit(0)
	} else {
		fmt.Fprintln(os.Stderr, "Verification failed!")
		os.Exit(1)
	}
}
