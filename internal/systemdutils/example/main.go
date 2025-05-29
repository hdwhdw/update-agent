// Example for using systemdutils to run commands via systemd
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"upgrade-agent/internal/systemdutils"
)

func main() {
	// Parse command line flags
	cmdFlag := flag.String("cmd", "", "Command to execute via systemd")
	scriptFlag := flag.String("script", "", "Script content to execute via systemd")
	flag.Parse()

	// Ensure at least one flag is provided
	if *cmdFlag == "" && *scriptFlag == "" {
		fmt.Println("Usage:")
		fmt.Println("  -cmd string    Command to execute via systemd")
		fmt.Println("  -script string Script content to execute via systemd")
		os.Exit(1)
	}

	var output string
	var err error

	// Execute the command or script
	if *cmdFlag != "" {
		fmt.Printf("Executing command: %s\n", *cmdFlag)
		output, err = systemdutils.RunCommandAsRoot(*cmdFlag)
	} else {
		fmt.Println("Executing script")
		output, err = systemdutils.RunScriptAsRoot(*scriptFlag)
	}

	// Handle errors
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// Output the result
	fmt.Println("Command output:")
	fmt.Println(output)
}
