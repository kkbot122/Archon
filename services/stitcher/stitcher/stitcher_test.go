package stitcher_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kisna/archon/services/stitcher/library"
	"github.com/kisna/archon/services/stitcher/manifest"
	"github.com/kisna/archon/services/stitcher/stitcher"
)

// MockRegistry allows us to test the Stitcher without hitting the real file system index
type MockRegistry struct{}

func (m *MockRegistry) GetBrick(brickType string) (library.BrickMeta, bool) {
	if brickType == "postgres" {
		return library.BrickMeta{
			Type:            "postgres",
			AllowedVersions: []string{"15"},
			Path:            "/tmp/mock/postgres",
		}, true
	}
	return library.BrickMeta{}, false
}

func TestEngine_Stitch_Success(t *testing.T) {
	resolver := library.NewResolver(&MockRegistry{})
	engine := stitcher.NewEngine(resolver)

	// Using the correct manifest.Manifest struct!
	m := &manifest.Manifest{
		Nodes: []manifest.Node{
			{ID: "db_1", Type: "postgres", Version: "15", Config: map[string]string{}},
		},
	}

	// Create an isolated workspace directory for the test
	workspace := t.TempDir()

	err := engine.Stitch(m, workspace)
	assert.NoError(t, err)

	// Verify the engine wrote the correct output file to the workspace
	expectedFile := filepath.Join(workspace, "db_1-docker-compose.yml")
	_, err = os.Stat(expectedFile)
	assert.NoError(t, err, "The hydrated template should exist in the workspace")
}

func TestRenderTemplate_StringSignature(t *testing.T) {
	// Verifies the test suite doesn't panic on the new signature
	out, err := stitcher.RenderTemplate("Hello {{.name}}", map[string]string{"name": "World"})
	assert.NoError(t, err)
	assert.Equal(t, "Hello World", out)
}