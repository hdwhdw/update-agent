package grpcclient

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	gnoisonic "upgrade-agent/gnoi_sonic"

	syspb "github.com/openconfig/gnoi/system"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client wraps the gRPC connection and SonicUpgradeService client.
type Client struct {
	conn          *grpc.ClientConn
	client        gnoisonic.SonicUpgradeServiceClient
	systemClient  syspb.SystemClient
}

// NewClient creates a new gRPC client for the SonicUpgradeService.
func NewClient(target string) (*Client, error) {
	log.Printf("Creating new gRPC client with target: %q", target)
	if target == "" {
		return nil, fmt.Errorf("empty gRPC target specified")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use the recommended gRPC connection options
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	conn, err := grpc.DialContext(ctx, target, opts...)
	if err != nil {
		return nil, err
	}

	sonicClient := gnoisonic.NewSonicUpgradeServiceClient(conn)
	systemClient := syspb.NewSystemClient(conn)

	return &Client{
		conn:         conn,
		client:       sonicClient,
		systemClient: systemClient,
	}, nil
}

// Close closes the gRPC connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// UpdateFirmware starts a firmware update and streams status/log lines back.
func (c *Client) UpdateFirmware(ctx context.Context, params *gnoisonic.FirmwareUpdateParams) error {
	stream, err := c.client.UpdateFirmware(ctx)
	if err != nil {
		return err
	}
	// Send the initial request
	req := &gnoisonic.UpdateFirmwareRequest{
		Request: &gnoisonic.UpdateFirmwareRequest_FirmwareUpdate{
			FirmwareUpdate: params,
		},
	}
	if err := stream.Send(req); err != nil {
		return err
	}
	// Close the send direction to indicate no more requests
	if err := stream.CloseSend(); err != nil {
		return err
	}
	// Receive and print status updates
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		log.Printf("[FW Update] %s (state=%s, exit_code=%d)", resp.GetLogLine(), resp.GetState().String(), resp.GetExitCode())
	}
	return nil
}

// GetSystemTime retrieves the current time from gNOI System service
func (c *Client) GetSystemTime(ctx context.Context) (*syspb.TimeResponse, error) {
	if c.systemClient == nil {
		return nil, fmt.Errorf("system client not initialized")
	}

	log.Println("Requesting system time via gNOI.System.Time")
	timeResp, err := c.systemClient.Time(ctx, &syspb.TimeRequest{})
	if err != nil {
		log.Printf("Failed to get system time: %v", err)
		return nil, err
	}

	// The time is in nanoseconds since epoch
	nanos := int64(timeResp.GetTime())
	systemTime := time.Unix(nanos/1e9, nanos%1e9)
	log.Printf("System time response: time=%v", systemTime)
	return timeResp, nil
}
