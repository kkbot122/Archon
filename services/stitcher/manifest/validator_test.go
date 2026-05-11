package manifest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kisna/archon/services/stitcher/manifest"
)

func TestValidate_Success(t *testing.T) {
	m := &manifest.Manifest{
		Metadata: manifest.Metadata{ProjectName: "ValidProject"},
		Nodes: []manifest.Node{
			{ID: "api", Type: "go_backend"},
		},
	}

	err := manifest.Validate(m)
	assert.NoError(t, err)
}

func TestValidate_MissingProjectName(t *testing.T) {
	m := &manifest.Manifest{
		Metadata: manifest.Metadata{ProjectName: ""},
		Nodes: []manifest.Node{
			{ID: "api", Type: "go_backend"},
		},
	}

	err := manifest.Validate(m)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing project name")
}

func TestValidate_NoNodes(t *testing.T) {
	m := &manifest.Manifest{
		Metadata: manifest.Metadata{ProjectName: "EmptyProject"},
		Nodes:    []manifest.Node{},
	}

	err := manifest.Validate(m)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no nodes")
}

func TestGenerateIdempotencyKey_IsStable(t *testing.T) {
	m1 := &manifest.Manifest{
		Metadata: manifest.Metadata{ProjectName: "HashTest"},
		Nodes:    []manifest.Node{{ID: "db", Type: "postgres"}},
	}

	m2 := &manifest.Manifest{
		Metadata: manifest.Metadata{ProjectName: "HashTest"},
		Nodes:    []manifest.Node{{ID: "db", Type: "postgres"}},
	}

	hash1, err1 := manifest.GenerateIdempotencyKey(m1)
	hash2, err2 := manifest.GenerateIdempotencyKey(m2)

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.Equal(t, hash1, hash2, "Identical manifests should produce identical hashes")
}