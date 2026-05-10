package library

import (
	"fmt"
	"github.com/kisna/archon/services/stitcher/manifest"
)

type Resolver struct {
	registry *Registry
}

func NewResolver(registry *Registry) *Resolver {
	return &Resolver{registry: registry}
}

func (r *Resolver) Resolve(node *manifest.Node) (*BrickMeta, error) {
	// Look up the brick by its type
	brick, exists := r.registry.bricks[node.Type]
	if !exists {
		return nil, fmt.Errorf("brick type '%s' not found in library index", node.Type)
	}

	// Validate version match against the array of allowed_versions
	validVersion := false
	for _, allowed := range brick.AllowedVersions {
		if allowed == node.Version {
			validVersion = true
			break
		}
	}
	if !validVersion {
		return nil, fmt.Errorf("brick type '%s' requires one of versions %v, but manifest requested %s", node.Type, brick.AllowedVersions, node.Version)
	}

	// Validate required config
	for _, reqKey := range brick.RequiredConfig {
		if val, ok := node.Config[reqKey]; !ok || val == "" {
			return nil, fmt.Errorf("node '%s' (type %s) is missing required config key: %s", node.ID, node.Type, reqKey)
		}
	}

	return &brick, nil
}

// BasePath returns the root directory of the atomic library from the underlying registry
func (r *Resolver) BasePath() string {
	return r.registry.BasePath()
}