package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Hyaxia/blogwatcher/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestRequireConfig(t *testing.T) {
	// Create a temp directory with no config
	tmpDir := t.TempDir()

	// Set home directory for cross-platform testing
	// On Unix: HOME, on Windows: USERPROFILE
	if runtime.GOOS == "windows" {
		originalUserprofile := os.Getenv("USERPROFILE")
		os.Setenv("USERPROFILE", tmpDir)
		defer os.Setenv("USERPROFILE", originalUserprofile)
	} else {
		originalHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpDir)
		defer os.Setenv("HOME", originalHome)
	}

	err := RequireConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not configured")
}

func TestRequireConfigExists(t *testing.T) {
	// Create a temp directory with a valid config
	tmpDir := t.TempDir()

	// Set home directory for cross-platform testing
	if runtime.GOOS == "windows" {
		originalUserprofile := os.Getenv("USERPROFILE")
		os.Setenv("USERPROFILE", tmpDir)
		defer os.Setenv("USERPROFILE", originalUserprofile)
	} else {
		originalHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpDir)
		defer os.Setenv("HOME", originalHome)
	}

	cfgDir := filepath.Join(tmpDir, ".blogwatcher")
	os.MkdirAll(cfgDir, 0o755)
	cfg := &config.Config{
		LLM: config.LLMConfig{
			APIKey:  "test-key",
			BaseURL: "https://api.openai.com/v1",
			Model:   "gpt-4o-mini",
		},
	}
	cfg.Save(filepath.Join(cfgDir, "config.yaml"))

	err := RequireConfig()
	assert.NoError(t, err)
}