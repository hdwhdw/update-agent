// Package grpcserver implements the gRPC server functionality
package grpcserver

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	gnoisonic "upgrade-agent/gnoi_sonic"
	"upgrade-agent/internal/sonicservice"
	"upgrade-agent/internal/systemservice"

	"github.com/openconfig/gnoi/system"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Server encapsulates the gRPC server functionality
type Server struct {
	grpcServer      *grpc.Server
	sonicService    *sonicservice.Service
	systemService   *systemservice.Service
	listener        net.Listener
}

// NewServer creates a new instance of Server
func NewServer(port string) (*Server, error) {
	lis, err := net.Listen("tcp", "0.0.0.0:"+port)
	if err != nil {
		return nil, err
	}

	grpcServer := grpc.NewServer()
	sonicSvc := sonicservice.NewService()
	systemSvc := systemservice.NewService()

	// Register services
	gnoisonic.RegisterSonicUpgradeServiceServer(grpcServer, sonicSvc)
	system.RegisterSystemServer(grpcServer, systemSvc)

	// Register reflection service on gRPC server
	reflection.Register(grpcServer)

	return &Server{
		grpcServer:     grpcServer,
		sonicService:   sonicSvc,
		systemService:  systemSvc,
		listener:       lis,
	}, nil
}

// Start begins serving gRPC requests
func (s *Server) Start() error {
	log.Printf("Server listening at %v", s.listener.Addr())
	return s.grpcServer.Serve(s.listener)
}

// Stop gracefully stops the server
func (s *Server) Stop() {
	s.grpcServer.GracefulStop()
	log.Println("Server stopped gracefully")
}

// RunUntilSignaled runs the server until it receives a termination signal
func (s *Server) RunUntilSignaled() {
	// Start server in a goroutine
	go func() {
		if err := s.Start(); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for termination signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	log.Printf("Received signal %v, shutting down...", sig)
	s.Stop()
}
