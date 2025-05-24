package main

import (
	"flag"
	"log"

	"upgrade-agent/internal/grpcserver"
)

func main() {
	// Parse command line flags
	port := flag.String("port", "8080", "The server port")
	flag.Parse()

	log.Printf("Starting upgrade server on port %s", *port)

	// Create and run the server
	srv, err := grpcserver.NewServer(*port)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Run the server until it receives a termination signal
	srv.RunUntilSignaled()
}
