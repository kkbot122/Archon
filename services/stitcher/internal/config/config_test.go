package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	// Clear env vars to test defaults
	os.Clearenv()
	cfg := Load()

	assert.Equal(t, "development", cfg.Environment)
	assert.Equal(t, "localhost:9092", cfg.KafkaBrokers)
	assert.Equal(t, time.Duration(300)*time.Second, cfg.BuildTimeout)

	// Test overrides
	os.Setenv("APP_ENV", "production")
	os.Setenv("BUILD_TIMEOUT_SEC", "600")
	cfgOverride := Load()

	assert.Equal(t, "production", cfgOverride.Environment)
	assert.Equal(t, time.Duration(600)*time.Second, cfgOverride.BuildTimeout)
}