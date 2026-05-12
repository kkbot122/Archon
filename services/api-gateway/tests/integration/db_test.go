//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kisna/archon/services/api-gateway/tests/integration/helpers"
)

func TestCreateProjectInsertsRow(t *testing.T) {
	repo, pool := helpers.SetupTestDB(t)
	helpers.EnsureDummyUser(t, pool)

	name := "TestProject_Create"
	p, err := repo.CreateProject(context.Background(), uuid.MustParse("11111111-1111-1111-1111-111111111111"), name)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, p.ID, "project ID should be a valid UUID")
	assert.Equal(t, name, p.Name)
}

func TestSaveManifestInsertsRow(t *testing.T) {
	repo, pool := helpers.SetupTestDB(t)
	helpers.EnsureDummyUser(t, pool)

	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	proj, err := repo.CreateProject(context.Background(), userID, "TestProject_SaveManifest")
	require.NoError(t, err)

	versionHash := "abc123"
	manifestData := []byte(`{"nodes":[{"id":"db1","type":"postgres"}]}`)

	err = repo.SaveManifest(context.Background(), proj.ID, versionHash, manifestData)
	require.NoError(t, err)

	m, err := repo.GetLatestManifest(context.Background(), proj.ID)
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, versionHash, m.VersionHash)
	assert.JSONEq(t, `{"nodes":[{"id":"db1","type":"postgres"}]}`, string(m.ManifestData))
}

func TestGetLatestManifestReturnsMostRecent(t *testing.T) {
	repo, pool := helpers.SetupTestDB(t)
	helpers.EnsureDummyUser(t, pool)

	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	proj, err := repo.CreateProject(context.Background(), userID, "TestProject_LatestManifest")
	require.NoError(t, err)

	hash1 := "hash1"
	hash2 := "hash2"
	data1 := []byte(`{"version":1}`)
	data2 := []byte(`{"version":2}`)

	err = repo.SaveManifest(context.Background(), proj.ID, hash1, data1)
	require.NoError(t, err)
	time.Sleep(1 * time.Millisecond)
	err = repo.SaveManifest(context.Background(), proj.ID, hash2, data2)
	require.NoError(t, err)

	latest, err := repo.GetLatestManifest(context.Background(), proj.ID)
	require.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, hash2, latest.VersionHash)
	assert.JSONEq(t, `{"version":2}`, string(latest.ManifestData))
}

func TestGetLatestManifestReturnsNilForNewProject(t *testing.T) {
	repo, _ := helpers.SetupTestDB(t)

	m, err := repo.GetLatestManifest(context.Background(), uuid.New())
	assert.NoError(t, err)
	assert.Nil(t, m, "should be nil for a project with no manifests")
}

func TestCreateProjectWithManifestIsAtomic(t *testing.T) {
	repo, pool := helpers.SetupTestDB(t)
	helpers.EnsureDummyUser(t, pool)

	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	name := "TestProject_Atomic"
	manifestData := []byte(`{"metadata":{"project_name":"` + name + `"}}`)

	p, err := repo.CreateProjectWithManifest(context.Background(), userID, name, manifestData)
	require.NoError(t, err)
	require.NotNil(t, p)

	var count int
	err = pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM projects WHERE id=$1", p.ID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	err = pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM manifests WHERE project_id=$1", p.ID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	tx, err := pool.Begin(context.Background())
	require.NoError(t, err)
	projID := uuid.New()
	_, err = tx.Exec(context.Background(),
		"INSERT INTO projects (id, user_id, name) VALUES ($1, $2, $3)",
		projID, userID, "RollbackTest")
	require.NoError(t, err)
	tx.Rollback(context.Background())

	err = pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM projects WHERE id=$1", projID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}