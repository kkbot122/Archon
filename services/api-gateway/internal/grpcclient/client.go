// services/api-gateway/internal/grpcclient/client.go
package grpcclient

import (
	"context"
	"fmt"
	"time"

	"github.com/kisna/archon/internal/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

type ArchitectClient struct {
	conn   *grpc.ClientConn
	client pb.ArchitectBrainClient // explicitly named 'client' now
}

func NewClient(target string) (*ArchitectClient, error) {
	// Configure Keepalive parameters
	kacp := keepalive.ClientParameters{
		Time:                10 * time.Second, // Send pings every 10s if idle
		Timeout:             3 * time.Second,  // Wait 3s for ping ack before declaring dead
		PermitWithoutStream: true,             // Send pings even without active RPCs
	}

	conn, err := grpc.NewClient(target, 
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(kacp), // NEW: Injecting keepalive
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}

	client := pb.NewArchitectBrainClient(conn)

	return &ArchitectClient{
		conn:   conn,
		client: client,
	}, nil
}

// Close gracefully shuts down the connection
// AUDIT FIX: Returning the error so the caller knows if cleanup failed
func (c *ArchitectClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Refine sends the manifest and prompt to the Python AI Brain
func (c *ArchitectClient) Refine(ctx context.Context, req *pb.RefineManifestRequest) (*pb.RefineManifestResponse, error) {
	resp, err := c.client.RefineManifest(ctx, req)
	if err != nil {
		// This is a REAL network/system error (e.g., connection refused, timeout)
		return nil, fmt.Errorf("gRPC call failed: %w", err)
	}

	// We return the raw response and let the GraphQL resolver handle the business logic.
	return resp, nil
}