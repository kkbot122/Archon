package library

import (
	"fmt"

	"github.com/kisna/archon/services/stitcher/manifest"
)

// BrickRegistry abstracts the index so we can seamlessly mock it in tests
type BrickRegistry interface {
	GetBrick(brickType string) (BrickMeta, bool)
}

type Resolver struct {
	registry BrickRegistry
}

// NewResolver now accepts any struct that satisfies the BrickRegistry interface
func NewResolver(registry BrickRegistry) *Resolver {
	return &Resolver{registry: registry}
}

// Resolve validates a manifest node against the atomic library rules
func (r *Resolver) Resolve(node *manifest.Node) (*BrickMeta, error) {
	brick, ok := r.registry.GetBrick(node.Type)
	if !ok {
		return nil, fmt.Errorf("brick type '%s' not found in library", node.Type)
	}

	// 1. Version Check
	validVersion := false
	for _, v := range brick.AllowedVersions {
		if v == node.Version {
			validVersion = true
			break
		}
	}
	if !validVersion {
		return nil, fmt.Errorf("node '%s' version '%s' unsupported; requires one of versions %v", node.ID, node.Version, brick.AllowedVersions)
	}

	// 2. Config Check
	for _, reqKey := range brick.RequiredConfig {
		if _, exists := node.Config[reqKey]; !exists {
			return nil, fmt.Errorf("node '%s' is missing required config keys. Expected: %v", node.ID, brick.RequiredConfig)
		}
	}

	return &brick, nil
}