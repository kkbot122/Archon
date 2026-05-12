//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kisna/archon/services/api-gateway/tests/integration/helpers"
)

func TestRefineArchitectureBroadcastsViaWebSocket(t *testing.T) {
	_, pool := helpers.SetupTestDB(t)
	helpers.EnsureDummyUser(t, pool)

	projectID := helpers.CreateTestProject(t)

	// Connect WebSocket for this project
	wsConn := helpers.SetupWebSocketClient(t, projectID)

	// Refine – triggers a Redis publish with the updated manifest
	_, err := helpers.MakeGraphQLRequest(t,
		`mutation($projectId: ID!, $prompt: String!) {
			refineArchitecture(projectId: $projectId, prompt: $prompt) {
				isValid
				reasoning
			}
		}`, map[string]interface{}{
			"projectId": projectID,
			"prompt":    "Add a postgres database",
		})
	require.NoError(t, err)

	// The WebSocket receives the manifest JSON directly
	msg, err := helpers.ReadWebSocketMessage(t, wsConn, 5*time.Second)
	require.NoError(t, err)
	msgStr := string(msg)

	// Check for expected mock LLM content
	assert.Contains(t, msgStr, "mock_node", "should contain the mock node")
	assert.Contains(t, msgStr, "postgres_db", "should mention postgres type")
}

func TestWebSocketOnlyReceivesOwnProjectUpdates(t *testing.T) {
	_, pool := helpers.SetupTestDB(t)
	helpers.EnsureDummyUser(t, pool)

	projectA := helpers.CreateTestProject(t)
	projectB := helpers.CreateTestProject(t)

	wsA := helpers.SetupWebSocketClient(t, projectA)
	wsB := helpers.SetupWebSocketClient(t, projectB)

	// Refine project A only
	_, err := helpers.MakeGraphQLRequest(t,
		`mutation($projectId: ID!, $prompt: String!) { refineArchitecture(projectId: $projectId, prompt: $prompt) { isValid } }`,
		map[string]interface{}{
			"projectId": projectA,
			"prompt":    "Add a postgres database",
		})
	require.NoError(t, err)

	// WS‑A must receive a message (contains the mock manifest)
	msgA, err := helpers.ReadWebSocketMessage(t, wsA, 5*time.Second)
	require.NoError(t, err)
	assert.Contains(t, string(msgA), "mock_node", "WS‑A should see the mock node")

	// WS‑B must NOT receive a message (short timeout confirms it)
	_, err = helpers.ReadWebSocketMessage(t, wsB, 2*time.Second)
	assert.Error(t, err, "WS‑B should not receive a message for project A")
}

func TestShipProjectBroadcastsBuildStatus(t *testing.T) {
	t.Skip("Skipping: build status bridging from Kafka to Redis not yet implemented")
}

func TestWebSocketRejectsMissingProjectId(t *testing.T) {
	conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:4000/ws", nil)
	if err == nil {
		conn.Close()
		t.Fatal("expected an error when connecting without projectId")
	}
}