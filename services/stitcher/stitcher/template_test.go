package stitcher_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kisna/archon/services/stitcher/stitcher" // Adjust path if needed
)

func TestRenderTemplate_SanitizesDashes(t *testing.T) {
	tmpl := `Connection string: {{.db_host}}:{{.db_port}}`
	
	// Input has dangerous dashes!
	configVars := map[string]string{
		"db-host": "127.0.0.1",
		"db-port": "5432",
	}

	out, err := stitcher.RenderTemplate(tmpl, configVars)
	
	assert.NoError(t, err)
	// It should seamlessly resolve the sanitized keys
	assert.Equal(t, "Connection string: 127.0.0.1:5432", out)
}