package grpcclient

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	gnoisonic "upgrade-agent/gnoi_sonic"

	ospb "github.com/openconfig/gnoi/os"
	syspb "github.com/openconfig/gnoi/system"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client wraps the gRPC connection and SonicUpgradeService client.
type Client struct {
	conn          *grpc.ClientConn
	client        gnoisonic.SonicUpgradeServiceClient
	systemClient  syspb.SystemClient
	osClient      ospb.OSClient
}

// NewClient creates a new gRPC client for the SonicUpgradeService.
func NewClient(target string) (*Client, error) {
	log.Printf("Creating new gRPC client with target: %q", target)
	if target == "" {
		return nil, fmt.Errorf("empty gRPC target specified")
	}

	// Use the recommended gRPC connection options with NewClient
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		return nil, err
	}

	sonicClient := gnoisonic.NewSonicUpgradeServiceClient(conn)
	systemClient := syspb.NewSystemClient(conn)
	osClient := ospb.NewOSClient(conn)

	return &Client{
		conn:         conn,
		client:       sonicClient,
		systemClient: systemClient,
		osClient:     osClient,
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

// GetOSVersion retrieves the current OS version from gNOI OS service
func (c *Client) GetOSVersion(ctx context.Context) (*ospb.VerifyResponse, error) {
	if c.osClient == nil {
		return nil, fmt.Errorf("OS client not initialized")
	}

	log.Println("Requesting OS version via gNOI.OS.Verify")
	verifyResp, err := c.osClient.Verify(ctx, &ospb.VerifyRequest{})
	if err != nil {
		log.Printf("Failed to get OS version: %v", err)
		return nil, err
	}

	log.Printf("OS version response: version=%v", verifyResp.GetVersion())
	return verifyResp, nil
}

// Reboot initiates a system reboot via gNOI System service
func (c *Client) Reboot(ctx context.Context) error {
	if c.systemClient == nil {
		return fmt.Errorf("system client not initialized")
	}

	log.Println("Initiating system COLD reboot via gNOI.System.Reboot")
	_, err := c.systemClient.Reboot(ctx, &syspb.RebootRequest{
		Method: syspb.RebootMethod_COLD,
		Force:  true,
		Message: "Rebooting to complete SONiC firmware update",
	})

	if err != nil {
		log.Printf("Failed to initiate reboot: %v", err)
		return err
	}

	log.Println("Reboot request successfully sent")
	return nil
}

// GetRebootStatus checks the status of a reboot via gNOI System service
func (c *Client) GetRebootStatus(ctx context.Context) (*syspb.RebootStatusResponse, error) {
	if c.systemClient == nil {
		return nil, fmt.Errorf("system client not initialized")
	}

	log.Println("Checking reboot status via gNOI.System.RebootStatus")
	resp, err := c.systemClient.RebootStatus(ctx, &syspb.RebootStatusRequest{})
	if err != nil {
		log.Printf("Failed to get reboot status: %v", err)
		return nil, err
	}

	log.Printf("Reboot status: active=%v", resp.GetActive())
	return resp, nil
}
