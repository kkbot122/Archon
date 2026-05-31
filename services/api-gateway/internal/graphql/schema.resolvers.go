package graphql

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/kisna/archon/internal/pb"
	"github.com/kisna/archon/services/api-gateway/internal/db"
	"github.com/kisna/archon/services/api-gateway/internal/kafka"
	"github.com/kisna/archon/services/api-gateway/internal/middleware"
	"google.golang.org/protobuf/encoding/protojson"
)

// mustUserID extracts the authenticated user ID from context.
// Returns an error (never panics) if the value is absent — this should be
// unreachable in production because the Auth middleware runs first.
func userIDFromCtx(ctx context.Context) (string, error) {
	id, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return "", fmt.Errorf("authenticated user ID not found in context")
	}
	return id, nil
}

// forbiddenError maps db.ErrForbidden to an HTTP 403-equivalent GraphQL error.
func mapDBError(err error) error {
	if errors.Is(err, db.ErrForbidden) {
		return &gqlError{msg: "forbidden", code: http.StatusForbidden}
	}
	return err
}

type gqlError struct {
	msg  string
	code int
}

func (e *gqlError) Error() string          { return e.msg }
func (e *gqlError) Extensions() map[string]any {
	return map[string]any{"code": e.code}
}

// Projects resolves the `projects` query — returns only the caller's projects.
func (r *queryResolver) Projects(ctx context.Context) ([]*Project, error) {
	userID, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := r.DB.GetProjectsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("fetching projects: %w", err)
	}

	out := make([]*Project, len(rows))
	for i, p := range rows {
		out[i] = &Project{
			ID:        p.ID.String(),
			Name:      p.Name,
			CreatedAt: p.CreatedAt.Time.Format(time.RFC3339),
		}
	}
	return out, nil
}

// CreateProject creates a new project owned by the authenticated user.
func (r *mutationResolver) CreateProject(ctx context.Context, name string) (*Project, error) {
	userID, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	emptyManifest := []byte(`{"metadata":{"project_name":"` + name + `"},"nodes":[],"connections":[]}`)

	p, err := r.DB.CreateProjectWithManifest(ctx, userID, name, emptyManifest)
	if err != nil {
		return nil, err
	}

	return &Project{
		ID:        p.ID.String(),
		Name:      p.Name,
		CreatedAt: p.CreatedAt.Time.Format(time.RFC3339),
	}, nil
}

// RefineArchitecture calls the AI brain. Enforces project ownership before proceeding.
func (r *mutationResolver) RefineArchitecture(ctx context.Context, projectID string, prompt string) (*RefineResponse, error) {
	userID, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	pID, err := uuid.Parse(projectID)
	if err != nil {
		return nil, fmt.Errorf("invalid project ID")
	}

	// Ownership check — returns 403 if the project belongs to another user.
	if _, err := r.DB.AssertProjectOwner(ctx, pID, userID); err != nil {
		return nil, mapDBError(err)
	}

	currentManifest, err := r.DB.GetLatestManifest(ctx, pID)
	var currentData []byte
	if err == nil && currentManifest != nil {
		currentData = currentManifest.ManifestData
	}

	currentPb := &pb.ProjectManifest{}
	if len(currentData) > 0 {
		if err := protojson.Unmarshal(currentData, currentPb); err != nil {
			return nil, fmt.Errorf("failed to parse current manifest: %w", err)
		}
	}

	req := &pb.RefineManifestRequest{
		TraceId:         uuid.New().String(),
		UserId:          userID, // real user ID
		CurrentManifest: currentPb,
		UserPrompt:      prompt,
		ProjectId:       projectID,
	}

	aiCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	resp, err := r.AI.Refine(aiCtx, req)
	if err != nil {
		return nil, fmt.Errorf("AI architect service unavailable: %w", err)
	}

	if !resp.IsValid {
		return &RefineResponse{
			IsValid:   false,
			Reasoning: resp.AiReasoning,
		}, nil
	}

	newData, err := protojson.Marshal(resp.UpdatedManifest)
	if err != nil {
		return nil, fmt.Errorf("failed to serialise AI response: %w", err)
	}

	hash := sha256.Sum256(newData)
	versionHash := hex.EncodeToString(hash[:])

	if err := r.DB.SaveManifest(ctx, pID, versionHash, newData); err != nil {
		return nil, fmt.Errorf("failed to save new manifest: %w", err)
	}

	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				fmt.Printf("⚠️ [PANIC RECOVERED] Redis broadcast: %v\n", rec)
			}
		}()
		bgCtx, bgCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer bgCancel()
		if pubErr := r.Redis.PublishManifestUpdate(bgCtx, projectID, string(newData)); pubErr != nil {
			fmt.Printf("⚠️ Redis publish error: %v\n", pubErr)
		}
	}()

	return &RefineResponse{
		IsValid:   true,
		Reasoning: resp.AiReasoning,
		Manifest: &ManifestRecord{
			ID:           "latest",
			ProjectID:    projectID,
			VersionHash:  versionHash,
			ManifestData: string(newData),
		},
	}, nil
}

// ShipProject fires the build event. Enforces ownership.
func (r *mutationResolver) ShipProject(ctx context.Context, projectID string) (bool, error) {
	userID, err := userIDFromCtx(ctx)
	if err != nil {
		return false, err
	}

	pID, err := uuid.Parse(projectID)
	if err != nil {
		return false, fmt.Errorf("invalid project ID")
	}

	if _, err := r.DB.AssertProjectOwner(ctx, pID, userID); err != nil {
		return false, mapDBError(err)
	}

	manifest, err := r.DB.GetLatestManifest(ctx, pID)
	if err != nil {
		return false, fmt.Errorf("could not find architecture to build: %w", err)
	}

	event := kafka.BuildRequestedEvent{
		ProjectID:   projectID,
		UserID:      userID, // ADDED
		VersionHash: manifest.VersionHash,
		ManifestRaw: string(manifest.ManifestData),
		RequestedAt: time.Now(),
	}

	if err := r.Kafka.PublishBuildRequest(ctx, event); err != nil {
		return false, fmt.Errorf("failed to trigger build pipeline: %w", err)
	}

	return true, nil
}

// GetLatestManifest enforces ownership before returning manifest data.
func (r *queryResolver) GetLatestManifest(ctx context.Context, projectID string) (*ManifestRecord, error) {
	userID, err := userIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	pID, err := uuid.Parse(projectID)
	if err != nil {
		return nil, fmt.Errorf("invalid project ID")
	}

	if _, err := r.DB.AssertProjectOwner(ctx, pID, userID); err != nil {
		return nil, mapDBError(err)
	}

	m, err := r.DB.GetLatestManifest(ctx, pID)
	if err != nil {
		return nil, fmt.Errorf("failed to query manifest: %w", err)
	}
	if m == nil {
		return nil, nil
	}

	return &ManifestRecord{
		ID:           m.ID.String(),
		ProjectID:    m.ProjectID.String(),
		VersionHash:  m.VersionHash,
		ManifestData: string(m.ManifestData),
	}, nil
}

func (r *Resolver) Mutation() MutationResolver { return &mutationResolver{r} }
func (r *Resolver) Query() QueryResolver       { return &queryResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }