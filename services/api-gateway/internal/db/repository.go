package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository provides access to the database.
type Repository struct {
	Pool *pgxpool.Pool // Capital 'P' so it is exported and accessible!
}

// NewRepository creates a new db repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{Pool: pool}
}

type Manifest struct {
	ID           uuid.UUID
	ProjectID    uuid.UUID
	VersionHash  string
	ManifestData []byte
	CreatedAt    time.Time
}

// CreateProject creates a new project
func (r *Repository) CreateProject(ctx context.Context, userID uuid.UUID, name string) (*Project, error) {
	var p Project
	err := r.Pool.QueryRow(ctx,
		"INSERT INTO projects (user_id, name) VALUES ($1, $2) RETURNING id, user_id, name, created_at, updated_at",
		userID, name,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}
	return &p, nil
}

// SaveManifest saves a new architecture manifest version
func (r *Repository) SaveManifest(ctx context.Context, projectID uuid.UUID, versionHash string, manifestData []byte) error {
	_, err := r.Pool.Exec(ctx,
		"INSERT INTO manifests (project_id, version_hash, manifest_data) VALUES ($1, $2, $3)",
		projectID, versionHash, manifestData,
	)
	return err
}

// GetLatestManifest retrieves the most recent architecture for a project
func (r *Repository) GetLatestManifest(ctx context.Context, projectID uuid.UUID) (*Manifest, error) {
	query := `
		SELECT id, project_id, version_hash, manifest_data, created_at
		FROM manifests
		WHERE project_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	
	// FIX 1 & 2: Actually execute the query and assign it to 'row'
	row := r.Pool.QueryRow(ctx, query, projectID)

	var m Manifest
	err := row.Scan(&m.ID, &m.ProjectID, &m.VersionHash, &m.ManifestData, &m.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// This is an expected state for new projects, return nil without throwing an error
			return nil, nil
		}
		return nil, fmt.Errorf("querying latest manifest: %w", err)
	}

	// FIX 3: Added the missing return statement
	return &m, nil
}

// CreateProjectWithManifest safely creates a project and its initial manifest in a single transaction.
func (r *Repository) CreateProjectWithManifest(ctx context.Context, userID uuid.UUID, name string, manifestData []byte) (*Project, error) {
	// FIX 4: Used capital P for r.Pool
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var p Project
	err = tx.QueryRow(ctx,
		"INSERT INTO projects (user_id, name) VALUES ($1, $2) RETURNING id, user_id, name, created_at, updated_at",
		userID, name,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert project: %w", err)
	}

	_, err = tx.Exec(ctx,
		"INSERT INTO manifests (project_id, version_hash, manifest_data) VALUES ($1, $2, $3)",
		p.ID, "init-hash", manifestData,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert initial manifest: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &p, nil
}