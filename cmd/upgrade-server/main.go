package main

import (
	"flag"
	"log"
	"os"
	"strings"

	"upgrade-agent/internal/grpcserver"
)

func init() {
	// Set up logging
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// Check if verbose logging is enabled via environment
	logLevel := strings.ToLower(os.Getenv("LOG_LEVEL"))
	if logLevel == "debug" || logLevel == "verbose" {
		log.Println("Debug logging enabled")
	}
}

func main() {
	// Parse command line flags
	port := flag.String("port", "8080", "The server port")
	fakeReboot := flag.Bool("fake-reboot", false, "If enabled, the server will fake reboots instead of actually rebooting")
	flag.Parse()

	log.Printf("Starting upgrade server on port %s", *port)
	if *fakeReboot {
		log.Printf("Fake reboot mode enabled - the server will not actually reboot the system")
	}

	// Show some diagnostic information
	procMounted := fileExists("/proc/cmdline")
	log.Printf("Diagnostic: /proc/cmdline accessible: %v", procMounted)

	if !procMounted {
		log.Printf("Warning: /proc/cmdline is not accessible. OS.Verify service may not work correctly.")
		log.Printf("To fix, run the container with: docker run -v /proc:/proc:ro ...")
	}

	// Create and run the server
	srv, err := grpcserver.NewServer(*port, *fakeReboot)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Run the server until it receives a termination signal
	srv.RunUntilSignaled()
}

// fileExists checks if a file or directory exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
