package brain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"

	"github.com/kisna/archon/services/api-gateway/internal/brain"
	pb "github.com/kisna/archon/internal/pb" 
)

// Mock the generated gRPC client interface
type MockArchitectBrainClient struct {
	mock.Mock
}

func (m *MockArchitectBrainClient) RefineManifest(ctx context.Context, in *pb.RefineManifestRequest, opts ...grpc.CallOption) (*pb.RefineManifestResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.RefineManifestResponse), args.Error(1)
}

func TestRefineManifest_Success(t *testing.T) {
	mockGrpcClient := new(MockArchitectBrainClient)
	brainClient := brain.NewClient(mockGrpcClient)

	// The request we expect to send
	req := &pb.RefineManifestRequest{
		UserPrompt: "Build a redis cache",
	}

	// The response we fake from the AI Brain
	expectedResp := &pb.RefineManifestResponse{
		IsValid: true,
		UpdatedManifest: &pb.ProjectManifest{
			// FIX: Changed to pb.Metadata to match your manifest.proto
			Metadata: &pb.Metadata{ProjectName: "CacheProject"}, 
		},
	}

	mockGrpcClient.On("RefineManifest", mock.Anything, req).Return(expectedResp, nil)

	// Execute the wrapper
	result, err := brainClient.Refine(context.Background(), "Build a redis cache", nil)

	assert.NoError(t, err)
	assert.True(t, result.IsValid)
	assert.Equal(t, "CacheProject", result.UpdatedManifest.Metadata.ProjectName)
	mockGrpcClient.AssertExpectations(t)
}