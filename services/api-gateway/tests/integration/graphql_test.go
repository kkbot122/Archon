//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kisna/archon/services/api-gateway/tests/integration/helpers"
)

func TestCreateProjectSuccess(t *testing.T) {
	_, pool := helpers.SetupTestDB(t)
	helpers.EnsureDummyUser(t, pool)

	id := helpers.CreateTestProject(t)
	assert.NotEmpty(t, id, "project ID should be returned")
}

func TestGetLatestManifestAfterCreate(t *testing.T) {
	_, pool := helpers.SetupTestDB(t)
	helpers.EnsureDummyUser(t, pool)

	projectID := helpers.CreateTestProject(t)

	query := `query($projectId: ID!) { getLatestManifest(projectId: $projectId) { manifestData } }`
	variables := map[string]interface{}{
		"projectId": projectID,
	}
	resp, err := helpers.MakeGraphQLRequest(t, query, variables)
	require.NoError(t, err)

	data := resp["data"].(map[string]interface{})
	manifest := data["getLatestManifest"].(map[string]interface{})
	manifestData := manifest["manifestData"].(string)
	assert.Contains(t, manifestData, "IntegrationTestProject", "manifest should mention the project name")
}

func TestRefineArchitectureSuccess(t *testing.T) {
	_, pool := helpers.SetupTestDB(t)
	helpers.EnsureDummyUser(t, pool)

	projectID := helpers.CreateTestProject(t)

	mutation := `mutation($projectId: ID!, $prompt: String!) {
		refineArchitecture(projectId: $projectId, prompt: $prompt) {
			isValid
			reasoning
			manifest {
				id
				manifestData
			}
		}
	}`
	variables := map[string]interface{}{
		"projectId": projectID,
		"prompt":    "Add a postgres database",
	}
	resp, err := helpers.MakeGraphQLRequest(t, mutation, variables)
	require.NoError(t, err)

	data := resp["data"].(map[string]interface{})
	refine := data["refineArchitecture"].(map[string]interface{})
	assert.Equal(t, true, refine["isValid"])
	assert.NotEmpty(t, refine["reasoning"])
	manifest := refine["manifest"].(map[string]interface{})
	assert.NotEmpty(t, manifest["id"])
}

func TestShipProjectPublishesToKafka(t *testing.T) {
	_, pool := helpers.SetupTestDB(t)
	helpers.EnsureDummyUser(t, pool)

	projectID := helpers.CreateTestProject(t)

	// Refine first to have a manifest
	mutation := `mutation($projectId: ID!, $prompt: String!) {
		refineArchitecture(projectId: $projectId, prompt: $prompt) {
			isValid
		}
	}`
	variables := map[string]interface{}{
		"projectId": projectID,
		"prompt":    "Add a postgres database",
	}
	_, err := helpers.MakeGraphQLRequest(t, mutation, variables)
	require.NoError(t, err)

	// Setup Kafka consumer for build.requests
	ch, _ := helpers.SetupKafkaConsumerFromLatest(t, "build.requests")
	time.Sleep(1 * time.Second)

	// Ship the project
	shipMutation := `mutation($projectId: ID!) { shipProject(projectId: $projectId) }`
	shipVars := map[string]interface{}{
		"projectId": projectID,
	}
	resp, err := helpers.MakeGraphQLRequest(t, shipMutation, shipVars)
	require.NoError(t, err)

	data := resp["data"].(map[string]interface{})
	shipResult := data["shipProject"].(bool)
	assert.True(t, shipResult)

	// Wait for the specific message for this project (filters out old test data)
	msgBytes, err := helpers.WaitForKafkaMessageContaining(ch, projectID, 20*time.Second)
	require.NoError(t, err, "expected a Kafka message containing the project ID within timeout")
	assert.Contains(t, string(msgBytes), "manifest_raw")
}