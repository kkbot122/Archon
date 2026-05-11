package brain

import (
	"context"

	"google.golang.org/grpc"

	// FIX: Point to the shared internal pb package
	pb "github.com/kisna/archon/internal/pb"
)

// ArchitectClient interface matches the generated gRPC client interface.
// This allows us to pass in a mock during testing.
type ArchitectClient interface {
	RefineManifest(ctx context.Context, in *pb.RefineManifestRequest, opts ...grpc.CallOption) (*pb.RefineManifestResponse, error)
}

// Client wraps the gRPC stub to provide a clean API for the Gateway's GraphQL resolvers.
type Client struct {
	grpcClient ArchitectClient
}

// NewClient creates a new AI Brain client wrapper.
func NewClient(grpcClient ArchitectClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

// Refine calls the AI Brain over gRPC to refine or generate a manifest.
func (c *Client) Refine(ctx context.Context, userPrompt string, currentManifest *pb.ProjectManifest) (*pb.RefineManifestResponse, error) {
	req := &pb.RefineManifestRequest{
		UserPrompt:      userPrompt,
		CurrentManifest: currentManifest,
	}

	// Execute the gRPC call strictly using Protobuf boundaries
	return c.grpcClient.RefineManifest(ctx, req)
}