package stitcher

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/kisna/archon/services/stitcher/library"
	"github.com/kisna/archon/services/stitcher/manifest"
	"github.com/kisna/archon/services/stitcher/workspace"
)

type Engine struct {
	resolver *library.Resolver
}

func NewEngine(resolver *library.Resolver) *Engine {
	return &Engine{resolver: resolver}
}

// Stitch executes the full file-generation pipeline
func (e *Engine) Stitch(ws *workspace.Workspace, m *manifest.ProjectManifest) error {
	
	// 1. Process each node defined in the manifest
	for _, node := range m.Nodes {
		
		// 2. Resolve the brick from the Atomic Library
		brickMeta, err := e.resolver.Resolve(&node)
		if err != nil {
			return fmt.Errorf("failed to resolve node %s: %w", node.ID, err)
		}

		// 3. Define the source directory (Atomic Library) and target directory (Workspace)
		brickSrcPath := filepath.Join(e.resolver.BasePath(), brickMeta.Path)
		
		// 4. Recursively process all files in the brick's directory
		err = filepath.WalkDir(brickSrcPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}

			// Determine relative path to replicate folder structure in the workspace
			relPath, _ := filepath.Rel(brickSrcPath, path)
			destPath := filepath.Join(ws.Path, relPath)
			
			// Ensure parent directories exist
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return err
			}

			// Apply file handling strategies
			if strings.HasSuffix(path, ".tmpl") {
				return RenderTemplate(path, destPath, node.Config)
			} else if strings.HasSuffix(path, ".json") {
				return DeepMergeJSON(path, destPath)
			} else {
				return copyFile(path, destPath)
			}
		})

		if err != nil {
			return fmt.Errorf("failed to stitch brick %s: %w", brickMeta.Type, err)
		}
	}

	return nil
}