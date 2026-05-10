package library

import (
	"testing"
	"github.com/kisna/archon/services/stitcher/manifest"
	"github.com/stretchr/testify/assert"
)

func TestResolver(t *testing.T) {
	// 1. Setup an in-memory mock registry
	mockRegistry := &Registry{
		bricks: map[string]BrickMeta{
			"postgres_db": {
				Name:           "Postgres",
				Version:        "1.0",
				RequiredConfig: []string{"db_user", "db_password"},
			},
		},
	}
	resolver := NewResolver(mockRegistry)

	// 2. Test successful resolution
	t.Run("Valid Node Resolution", func(t *testing.T) {
		validNode := &manifest.Node{
			ID:      "db_1",
			Type:    "postgres_db",
			Version: "1.0",
			Config: map[string]string{
				"db_user":     "admin",
				"db_password": "secret_password",
			},
		}

		brick, err := resolver.Resolve(validNode)
		assert.NoError(t, err)
		assert.Equal(t, "Postgres", brick.Name)
	})

	// 3. Test missing brick type
	t.Run("Brick Not Found", func(t *testing.T) {
		invalidNode := &manifest.Node{
			Type: "unknown_tech",
		}
		_, err := resolver.Resolve(invalidNode)
		assert.ErrorContains(t, err, "not found in library index")
	})

	// 4. Test version mismatch
	t.Run("Version Mismatch", func(t *testing.T) {
		wrongVersionNode := &manifest.Node{
			Type:    "postgres_db",
			Version: "2.0",
		}
		_, err := resolver.Resolve(wrongVersionNode)
		assert.ErrorContains(t, err, "requires version 1.0, but manifest requested 2.0")
	})

	// 5. Test missing required config variables
	t.Run("Missing Required Config", func(t *testing.T) {
		missingConfigNode := &manifest.Node{
			ID:      "db_1",
			Type:    "postgres_db",
			Version: "1.0",
			Config: map[string]string{
				"db_user": "admin",
				// db_password is missing!
			},
		}
		_, err := resolver.Resolve(missingConfigNode)
		assert.ErrorContains(t, err, "missing required config key: db_password")
	})
}