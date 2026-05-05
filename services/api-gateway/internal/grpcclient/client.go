// services/api-gateway/internal/grpcclient/client.go
package grpcclient

import (
	"context"
	"fmt"

	// Import the protobuf code we generated earlier
	"github.com/kisna/archon/internal/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
)

// ArchitectClient wraps the gRPC connection to the AI Brain
type ArchitectClient struct {
	conn *grpc.ClientConn
	api  pb.ArchitectBrainClient
}

// NewClient establishes a connection to the Python gRPC server
func NewClient(targetURL string) (*ArchitectClient, error) {
	// We use "insecure" credentials here because this will be a local Docker-to-Docker
	// internal network connection. Production over the open internet would require TLS.
	conn, err := grpc.Dial(targetURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to dial AI Brain: %w", err)
	}

	return &ArchitectClient{
		conn: conn,
		api:  pb.NewArchitectBrainClient(conn),
	}, nil
}

// Close gracefully shuts down the connection
func (c *ArchitectClient) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

// Refine takes raw JSON from Postgres, talks to the AI, and returns the updated JSON
func (c *ArchitectClient) Refine(ctx context.Context, traceID, userID, prompt string, currentManifestJSON []byte) ([]byte, string, error) {
	
	// 1. Convert DB JSON []byte into the Protobuf Struct
	var currentManifest pb.ProjectManifest
	if len(currentManifestJSON) > 0 {
		// We use protojson, NOT standard json, because protojson handles Protobuf specific edge-cases
		if err := protojson.Unmarshal(currentManifestJSON, &currentManifest); err != nil {
			return nil, "", fmt.Errorf("failed to parse DB manifest into Protobuf: %w", err)
		}
	}

	// 2. Build the exact Request required by our manifest.proto
	req := &pb.RefineManifestRequest{
		TraceId:         traceID,
		UserId:          userID,
		UserPrompt:      prompt,
		CurrentManifest: &currentManifest,
	}

	// 3. Fire the gRPC call to Python!
	res, err := c.api.RefineManifest(ctx, req)
	if err != nil {
		return nil, "", fmt.Errorf("gRPC call to AI Brain failed: %w", err)
	}

	// 4. Handle AI Validation
	if !res.IsValid {
		// If the AI says "You can't connect Postgres to a static HTML site", 
		// we return the reasoning but NO new JSON.
		return nil, res.AiReasoning, fmt.Errorf("AI rejected the change")
	}

	// 5. Convert the newly updated Protobuf Struct back into raw JSON for Postgres
	newManifestJSON, err := protojson.Marshal(res.UpdatedManifest)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal updated Protobuf to JSON: %w", err)
	}

	return newManifestJSON, res.AiReasoning, nil
}