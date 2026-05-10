package stitcher

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kisna/archon/services/stitcher/workspace"
	"github.com/stretchr/testify/assert"
)

func TestTemplateRendering(t *testing.T) {
	ws, _ := workspace.Create("test-render-123")
	defer ws.Cleanup()

	// 1. Create a dummy template file
	tmplPath := filepath.Join(ws.Path, "config.go.tmpl")
	destPath := filepath.Join(ws.Path, "config.go.tmpl") // Template rendering drops the .tmpl
	err := os.WriteFile(tmplPath, []byte("package main\n\nconst DB_USER = \"{{.db_user}}\""), 0644)
	assert.NoError(t, err)

	// 2. Render it
	vars := map[string]string{"db_user": "admin"}
	err = RenderTemplate(tmplPath, destPath, vars)
	assert.NoError(t, err)

	// 3. Verify output
	finalPath := filepath.Join(ws.Path, "config.go")
	content, err := os.ReadFile(finalPath)
	assert.NoError(t, err)
	assert.Equal(t, "package main\n\nconst DB_USER = \"admin\"", string(content))
}

func TestJSONDeepMerge(t *testing.T) {
	ws, _ := workspace.Create("test-merge-123")
	defer ws.Cleanup()

	// Source package.json
	srcPath := filepath.Join(ws.Path, "src.json")
	os.WriteFile(srcPath, []byte(`{"dependencies": {"express": "4.17.1"}}`), 0644)

	// Target package.json
	destPath := filepath.Join(ws.Path, "dest.json")
	os.WriteFile(destPath, []byte(`{"dependencies": {"react": "18.2.0"}, "name": "app"}`), 0644)

	// Merge!
	err := DeepMergeJSON(srcPath, destPath)
	assert.NoError(t, err)

	mergedBytes, _ := os.ReadFile(destPath)
	mergedStr := string(mergedBytes)

	// Should contain both express and react
	assert.Contains(t, mergedStr, `"express": "4.17.1"`)
	assert.Contains(t, mergedStr, `"react": "18.2.0"`)
	assert.Contains(t, mergedStr, `"name": "app"`)
}