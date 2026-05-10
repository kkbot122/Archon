package stitcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// RenderTemplate reads a .tmpl file, injects the AI variables, and writes the pure code file
func RenderTemplate(srcPath string, destPath string, configVars map[string]string) error {
	content, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read template %s: %w", srcPath, err)
	}

	tmpl, err := template.New(filepath.Base(srcPath)).Parse(string(content))
	if err != nil {
		return fmt.Errorf("failed to parse template syntax: %w", err)
	}

	// Remove the .tmpl extension for the final output
	finalDest := strings.TrimSuffix(destPath, ".tmpl")
	
	outFile, err := os.Create(finalDest)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer outFile.Close()

	// Execute the template with the config variables map
	if err := tmpl.Execute(outFile, configVars); err != nil {
		return fmt.Errorf("failed to inject variables into template: %w", err)
	}

	return nil
}