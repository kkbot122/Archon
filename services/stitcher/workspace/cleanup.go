package workspace

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
)

// StartGarbageCollector runs a background loop to delete orphaned workspaces
func StartGarbageCollector(ctx context.Context, ttl time.Duration) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	basePath := filepath.Join(os.TempDir(), "archon-builds")

	log.Info().Str("path", basePath).Msg("🧹 Workspace GC started")

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("🛑 Workspace GC shutting down")
			return
		case <-ticker.C:
			entries, err := os.ReadDir(basePath)
			if err != nil {
				if !os.IsNotExist(err) {
					log.Error().Err(err).Msg("GC failed to read workspace directory")
				}
				continue
			}

			now := time.Now()
			cleaned := 0

			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}

				info, err := entry.Info()
				if err != nil {
					continue
				}

				// If the folder is older than the TTL (e.g., 1 hour), it's a zombie. Kill it.
				if now.Sub(info.ModTime()) > ttl {
					target := filepath.Join(basePath, entry.Name())
					if err := os.RemoveAll(target); err == nil {
						cleaned++
					}
				}
			}

			if cleaned > 0 {
				log.Info().Int("orphans_removed", cleaned).Msg("🧹 GC Sweep Complete")
			}
		}
	}
}