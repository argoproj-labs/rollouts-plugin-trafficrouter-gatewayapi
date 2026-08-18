package utils

import (
	"os"
	"path/filepath"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupLog_DefaultIsTextFormatter(t *testing.T) {
	entry := SetupLog("text")

	assert.NotNil(t, entry)
	assert.IsType(t, &log.TextFormatter{}, entry.Logger.Formatter)
}

func TestSetupLog_EmptyIsTextFormatter(t *testing.T) {
	entry := SetupLog("")

	assert.NotNil(t, entry)
	assert.IsType(t, &log.TextFormatter{}, entry.Logger.Formatter)
}

func TestSetupLog_JSONFormatter(t *testing.T) {
	entry := SetupLog("json")

	assert.NotNil(t, entry)
	assert.IsType(t, &log.JSONFormatter{}, entry.Logger.Formatter)
}

func TestSetupLog_JSONFormatterCaseInsensitive(t *testing.T) {
	entry := SetupLog("JSON")

	assert.NotNil(t, entry)
	assert.IsType(t, &log.JSONFormatter{}, entry.Logger.Formatter)
}

func TestGetKubeConfig_WithExplicitPath(t *testing.T) {
	// Create a temporary kubeconfig file
	tmpDir := t.TempDir()
	kubeconfigPath := filepath.Join(tmpDir, "kubeconfig")
	kubeconfigContent := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://test-server:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: test-token
`
	err := os.WriteFile(kubeconfigPath, []byte(kubeconfigContent), 0600)
	require.NoError(t, err)

	config, err := GetKubeConfig(kubeconfigPath)

	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "https://test-server:6443", config.Host)
}

func TestGetKubeConfig_WithEmptyPath(t *testing.T) {
	// When empty path is provided, it should use default discovery
	// This test verifies the function doesn't panic with empty path
	// The actual result depends on the environment (may succeed or fail)
	_, _ = GetKubeConfig("")
}

func TestGetKubeConfig_WithInvalidPath(t *testing.T) {
	config, err := GetKubeConfig("/nonexistent/path/to/kubeconfig")

	assert.Error(t, err)
	assert.Nil(t, config)
}

func TestGetKubeConfig_WithMalformedKubeconfig(t *testing.T) {
	// Create a temporary kubeconfig file with invalid YAML
	tmpDir := t.TempDir()
	kubeconfigPath := filepath.Join(tmpDir, "kubeconfig")
	malformedContent := `this is not valid yaml: [[[`
	err := os.WriteFile(kubeconfigPath, []byte(malformedContent), 0600)
	require.NoError(t, err)

	config, err := GetKubeConfig(kubeconfigPath)

	assert.Error(t, err)
	assert.Nil(t, config)
}

func TestGetKubeConfig_WithEmptyKubeconfig(t *testing.T) {
	// Create an empty kubeconfig file
	tmpDir := t.TempDir()
	kubeconfigPath := filepath.Join(tmpDir, "kubeconfig")
	err := os.WriteFile(kubeconfigPath, []byte(""), 0600)
	require.NoError(t, err)

	config, err := GetKubeConfig(kubeconfigPath)

	// Empty kubeconfig should result in an error (no context set)
	assert.Error(t, err)
	assert.Nil(t, config)
}

func TestGetKubeConfig_WithMissingContext(t *testing.T) {
	// Create a kubeconfig file with missing current-context
	tmpDir := t.TempDir()
	kubeconfigPath := filepath.Join(tmpDir, "kubeconfig")
	kubeconfigContent := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://test-server:6443
  name: test-cluster
contexts: []
users: []
`
	err := os.WriteFile(kubeconfigPath, []byte(kubeconfigContent), 0600)
	require.NoError(t, err)

	config, err := GetKubeConfig(kubeconfigPath)

	assert.Error(t, err)
	assert.Nil(t, config)
}
