// services/api-gateway/internal/db/repository.go
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	Pool *pgxpool.Pool
}

// NewRepository initializes the Postgres connection pool
func NewRepository(ctx context.Context, connString string) (*Repository, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}

	// Ping to verify connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	return &Repository{Pool: pool}, nil
}

// CreateProject creates a new workspace for the user
func (r *Repository) CreateProject(ctx context.Context, userID uuid.UUID, name string) (*Project, error) {
	query := `
		INSERT INTO projects (user_id, name) 
		VALUES ($1, $2) 
		RETURNING id, user_id, name, created_at, updated_at`

	var p Project
	err := r.Pool.QueryRow(ctx, query, userID, name).Scan(
		&p.ID, &p.UserID, &p.Name, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// SaveManifest inserts a new version of the architecture into the DB
func (r *Repository) SaveManifest(ctx context.Context, projectID uuid.UUID, hash string, manifestJSON []byte) error {
	query := `
		INSERT INTO manifests (project_id, version_hash, manifest_data) 
		VALUES ($1, $2, $3)`

	_, err := r.Pool.Exec(ctx, query, projectID, hash, manifestJSON)
	return err
}

// GetLatestManifest fetches the most recently saved architecture for a project
func (r *Repository) GetLatestManifest(ctx context.Context, projectID uuid.UUID) (*ManifestRecord, error) {
	query := `
		SELECT id, project_id, version_hash, manifest_data, created_at 
		FROM manifests 
		WHERE project_id = $1 
		ORDER BY created_at DESC 
		LIMIT 1`

	var m ManifestRecord
	err := r.Pool.QueryRow(ctx, query, projectID).Scan(
		&m.ID, &m.ProjectID, &m.VersionHash, &m.ManifestData, &m.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &m, nil
}