package library_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kisna/archon/services/stitcher/library"
	"github.com/kisna/archon/services/stitcher/manifest"
)

// MockIndex simulates reading from atomic-library/index.json
type MockIndex struct {
	bricks map[string]library.BrickMeta
}

// GetBrick satisfies the new BrickRegistry interface!
func (m *MockIndex) GetBrick(brickType string) (library.BrickMeta, bool) {
	b, ok := m.bricks[brickType]
	return b, ok
}

func TestResolve_Success(t *testing.T) {
	mockIndex := &MockIndex{
		bricks: map[string]library.BrickMeta{
			"postgres": {
				Type:            "postgres",
				Description:     "PostgreSQL Database",
				AllowedVersions: []string{"14", "15"},
				RequiredConfig:  []string{"port"},
				Path:            "/atomic-library/databases/postgres",
				// FIX 1: BuildCommand is a string slice
				BuildCommand:    []string{"docker-compose", "up", "-d"}, 
			},
		},
	}

	resolver := library.NewResolver(mockIndex)

	node := manifest.Node{
		ID:      "db_1",
		Type:    "postgres",
		Version: "15",
		Config:  map[string]string{"port": "5432"},
	}

	// FIX 2: Pass pointer to node
	brick, err := resolver.Resolve(&node)

	assert.NoError(t, err)
	assert.Equal(t, "postgres", brick.Type)
}

func TestResolve_UnsupportedVersion(t *testing.T) {
	mockIndex := &MockIndex{
		bricks: map[string]library.BrickMeta{
			"postgres": {
				Type:            "postgres",
				AllowedVersions: []string{"14", "15"},
			},
		},
	}

	resolver := library.NewResolver(mockIndex)

	node := manifest.Node{
		ID:      "db_1",
		Type:    "postgres",
		Version: "99", // Invalid version, AI hallucinated!
	}

	brick, err := resolver.Resolve(&node)

	assert.Error(t, err)
	assert.Nil(t, brick)
	assert.Contains(t, err.Error(), "requires one of versions")
}

func TestResolve_MissingConfig(t *testing.T) {
	mockIndex := &MockIndex{
		bricks: map[string]library.BrickMeta{
			"go_backend": {
				Type:            "go_backend",
				AllowedVersions: []string{"1.21"},
				RequiredConfig:  []string{"port", "db_host"},
			},
		},
	}

	resolver := library.NewResolver(mockIndex)

	node := manifest.Node{
		ID:      "api_1",
		Type:    "go_backend",
		Version: "1.21",
		Config:  map[string]string{"port": "8080"}, // Missing db_host!
	}

	brick, err := resolver.Resolve(&node)

	assert.Error(t, err)
	assert.Nil(t, brick)
	assert.Contains(t, err.Error(), "missing required config keys")
}