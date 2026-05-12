//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kisna/archon/services/api-gateway/tests/integration/helpers"
)

func TestE2EFullUserJourney(t *testing.T) {
	_, pool := helpers.SetupTestDB(t)
	helpers.EnsureDummyUser(t, pool)

	// ── Step 1: Create project ──
	projectID := helpers.CreateTestProject(t)
	t.Logf("Created project: %s", projectID)

	// ── Step 2: Get latest manifest ──
	query := `query($id: ID!) { getLatestManifest(projectId: $id) { manifestData } }`
	resp, err := helpers.MakeGraphQLRequest(t, query, map[string]interface{}{
		"id": projectID,
	})
	require.NoError(t, err)
	data := resp["data"].(map[string]interface{})
	manifest := data["getLatestManifest"].(map[string]interface{})
	assert.Contains(t, manifest["manifestData"].(string), "IntegrationTestProject")

	// ── Step 3: Connect WebSocket ──
	wsConn := helpers.SetupWebSocketClient(t, projectID)

	// ── Step 4: First refine – add postgres ──
	mutation := `mutation($pid: ID!, $prompt: String!) {
		refineArchitecture(projectId: $pid, prompt: $prompt) {
			isValid
			reasoning
		}
	}`
	_, err = helpers.MakeGraphQLRequest(t, mutation, map[string]interface{}{
		"pid":    projectID,
		"prompt": "Add a postgres database",
	})
	require.NoError(t, err)

	// WebSocket receives the manifest update
	wsMsg, err := helpers.ReadWebSocketMessage(t, wsConn, 5*time.Second)
	require.NoError(t, err)
	assert.Contains(t, string(wsMsg), "mock_node")

	// ── Step 5: Second refine – add redis ──
	_, err = helpers.MakeGraphQLRequest(t, mutation, map[string]interface{}{
		"pid":    projectID,
		"prompt": "Add redis cache layer",
	})
	require.NoError(t, err)

	// Second WebSocket message (the second manifest)
	wsMsg2, err := helpers.ReadWebSocketMessage(t, wsConn, 5*time.Second)
	require.NoError(t, err)
	assert.Contains(t, string(wsMsg2), "mock_node")

	// ── Step 6: Ship project ──
	shipCh, _ := helpers.SetupKafkaConsumer(t, "build.requests")
	time.Sleep(1 * time.Second)

	shipMutation := `mutation($pid: ID!) { shipProject(projectId: $pid) }`
	shipResp, err := helpers.MakeGraphQLRequest(t, shipMutation, map[string]interface{}{
		"pid": projectID,
	})
	require.NoError(t, err)
	assert.True(t, shipResp["data"].(map[string]interface{})["shipProject"].(bool))

	// ── Step 7: Verify Kafka build request ──
	msgBytes, err := helpers.WaitForKafkaMessage(shipCh, 25*time.Second)
	require.NoError(t, err)
	assert.Contains(t, string(msgBytes), projectID)
	t.Logf("Kafka build request message: %s", string(msgBytes))
}