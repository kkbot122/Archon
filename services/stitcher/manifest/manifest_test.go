package manifest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kisna/archon/services/stitcher/manifest"
)

func TestParseManifest_ProtoJsonCompatibility(t *testing.T) {
	// This JSON simulates EXACTLY what protojson.Marshal outputs from the Gateway.
	// Notice the camelCase keys (projectName, sourceId).
	rawProtoJSON := `{
		"metadata": {
			"projectName": "TestProject",
			"targetCloud": "AWS",
			"schemaVersion": "1.0"
		},
		"nodes": [
			{
				"id": "db_1",
				"type": "postgres",
				"version": "15",
				"config": {
					"port": "5432"
				}
			}
		],
		"connections": [
			{
				"sourceId": "api",
				"targetId": "db_1",
				"protocol": "TCP"
			}
		]
	}`

	m, err := manifest.ParseManifest(rawProtoJSON)
	
	// If the structs are correct, it will parse seamlessly
	assert.NoError(t, err)
	assert.NotNil(t, m)

	// Validate Metadata
	assert.Equal(t, "TestProject", m.Metadata.ProjectName)
	assert.Equal(t, "AWS", m.Metadata.TargetCloud)

	// Validate Nodes
	assert.Len(t, m.Nodes, 1)
	assert.Equal(t, "db_1", m.Nodes[0].ID)
	assert.Equal(t, "5432", m.Nodes[0].Config["port"])

	// Validate Connections
	assert.Len(t, m.Connections, 1)
	assert.Equal(t, "api", m.Connections[0].SourceID)
}