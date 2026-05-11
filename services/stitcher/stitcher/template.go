package stitcher

import (
	"bytes"
	"strings"
	"text/template"
)

// RenderTemplate parses a raw template string and injects config variables.
func RenderTemplate(tmplStr string, configVars map[string]string) (string, error) {
	// FIX: Sanitize keys. Convert "db-host" into "db_host" so {{.db_host}} evaluates correctly.
	safeVars := make(map[string]string)
	for k, v := range configVars {
		safeKey := strings.ReplaceAll(k, "-", "_")
		safeVars[safeKey] = v
	}

	t, err := template.New("file").Parse(tmplStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, safeVars); err != nil {
		return "", err
	}

	return buf.String(), nil
}