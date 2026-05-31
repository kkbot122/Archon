-- Migration 001: per-user project query support
-- Safe to run multiple times (IF NOT EXISTS).

CREATE INDEX IF NOT EXISTS idx_projects_user_created
    ON projects (user_id, created_at DESC);