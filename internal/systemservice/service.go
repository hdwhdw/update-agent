// Package systemservice implements the gNOI System service
package systemservice

import (
	"context"
	"log"
	"time"

	"github.com/openconfig/gnoi/system"
)

// Service implements the gNOI System service
type Service struct {
	system.UnimplementedSystemServer
}

// NewService creates a new System service instance
func NewService() *Service {
	return &Service{}
}

// Time implements the gNOI System.Time RPC
func (s *Service) Time(ctx context.Context, req *system.TimeRequest) (*system.TimeResponse, error) {
	log.Println("Received System.Time request")

	// Get the current system time in nanoseconds since epoch
	now := time.Now()
	nanos := now.UnixNano()

	log.Printf("Responding with current system time: %v", now)
	return &system.TimeResponse{
		Time: uint64(nanos),
	}, nil
}
