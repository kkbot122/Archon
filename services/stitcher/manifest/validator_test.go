package manifest

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestValidator(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name          string
		manifest      *ProjectManifest
		expectedError string
	}{
		{
			name: "Valid Manifest",
			manifest: &ProjectManifest{
				Metadata: Metadata{ProjectName: "TestApp"},
				Nodes: []Node{
					{ID: "db_1", Type: "postgres"},
					{ID: "api_1", Type: "go_backend"},
				},
				Connections: []Connection{
					{SourceID: "api_1", TargetID: "db_1"},
				},
			},
			expectedError: "",
		},
		{
			name: "Missing Project Name",
			manifest: &ProjectManifest{
				Metadata: Metadata{ProjectName: "   "},
				Nodes:    []Node{{ID: "n1", Type: "t1"}},
			},
			expectedError: "project_name is missing",
		},
		{
			name: "No Nodes",
			manifest: &ProjectManifest{
				Metadata: Metadata{ProjectName: "TestApp"},
				Nodes:    []Node{},
			},
			expectedError: "manifest contains no nodes to build",
		},
		{
			name: "Duplicate Node ID",
			manifest: &ProjectManifest{
				Metadata: Metadata{ProjectName: "TestApp"},
				Nodes: []Node{
					{ID: "db_1", Type: "postgres"},
					{ID: "db_1", Type: "redis"}, // Duplicate!
				},
			},
			expectedError: "duplicate node ID found: db_1",
		},
		{
			name: "Orphaned Connection Target",
			manifest: &ProjectManifest{
				Metadata: Metadata{ProjectName: "TestApp"},
				Nodes:    []Node{{ID: "api_1", Type: "go_backend"}},
				Connections: []Connection{
					{SourceID: "api_1", TargetID: "ghost_db"}, // ghost_db doesn't exist
				},
			},
			expectedError: "connection target_id ghost_db does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.manifest)
			if tt.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			}
		})
	}
}

func TestIdempotencyKey(t *testing.T) {
	key1 := GenerateIdempotencyKey("proj-123", "hash-abc")
	key2 := GenerateIdempotencyKey("proj-123", "hash-abc")
	key3 := GenerateIdempotencyKey("proj-123", "hash-xyz")

	assert.Equal(t, key1, key2, "Identical inputs should yield identical keys")
	assert.NotEqual(t, key1, key3, "Different inputs should yield different keys")
}