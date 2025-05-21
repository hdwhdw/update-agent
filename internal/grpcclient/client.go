package grpcclient

import (
	"context"
	"io"
	"log"
	"time"

	gnoisonic "upgrade-agent/gnoi_sonic"

	"google.golang.org/grpc"
)

// Client wraps the gRPC connection and SonicUpgradeService client.
type Client struct {
	conn   *grpc.ClientConn
	client gnoisonic.SonicUpgradeServiceClient
}

// NewClient creates a new gRPC client for the SonicUpgradeService.
func NewClient(target string) (*Client, error) {
	conn, err := grpc.Dial(target, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(5*time.Second))
	if err != nil {
		return nil, err
	}
	client := gnoisonic.NewSonicUpgradeServiceClient(conn)
	return &Client{conn: conn, client: client}, nil
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
