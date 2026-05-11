package library

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Registry holds the in-memory cache of available bricks
type Registry struct {
	bricks   map[string]BrickMeta
	basePath string
}

// LoadRegistry parses the index.json from the atomic library path
func LoadRegistry(libraryPath string) (*Registry, error) {
	indexPath := filepath.Join(libraryPath, "index.json")
	
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read library index at %s: %w", indexPath, err)
	}

	var index Index
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to parse library index JSON: %w", err)
	}

	// Convert the JSON array into a map for fast O(1) lookups
	brickMap := make(map[string]BrickMeta)
	for _, node := range index.AvailableNodes {
		brickMap[node.Type] = node
	}

	return &Registry{
		bricks:   brickMap,
		basePath: libraryPath,
	}, nil
}

// BasePath returns the root directory of the atomic library
func (r *Registry) BasePath() string {
	return r.basePath
}

// GetBrick satisfies the BrickRegistry interface for the Resolver
func (r *Registry) GetBrick(brickType string) (BrickMeta, bool) {
	b, ok := r.bricks[brickType]
	return b, ok
}