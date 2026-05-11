-- name: CreateProject :one
INSERT INTO projects (user_id, name) 
VALUES ($1, $2) 
RETURNING id, user_id, name, created_at, updated_at;

-- name: SaveManifest :exec
INSERT INTO manifests (project_id, version_hash, manifest_data) 
VALUES ($1, $2, $3);

-- name: GetLatestManifest :one
SELECT id, project_id, version_hash, manifest_data, created_at 
FROM manifests 
WHERE project_id = $1 
ORDER BY created_at DESC 
LIMIT 1;