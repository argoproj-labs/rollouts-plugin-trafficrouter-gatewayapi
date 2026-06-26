package utils

import (
	"os"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestSetupLog_DefaultIsTextFormatter(t *testing.T) {
	os.Unsetenv("LOG_FORMAT")
	t.Cleanup(func() { os.Unsetenv("LOG_FORMAT") })

	entry := SetupLog()

	assert.NotNil(t, entry)
	assert.IsType(t, &log.TextFormatter{}, entry.Logger.Formatter)
}

func TestSetupLog_JSONFormatterWhenEnvSet(t *testing.T) {
	t.Setenv("LOG_FORMAT", "json")

	entry := SetupLog()

	assert.NotNil(t, entry)
	assert.IsType(t, &log.JSONFormatter{}, entry.Logger.Formatter)
}

func TestSetupLog_JSONFormatterCaseInsensitive(t *testing.T) {
	t.Setenv("LOG_FORMAT", "JSON")

	entry := SetupLog()

	assert.NotNil(t, entry)
	assert.IsType(t, &log.JSONFormatter{}, entry.Logger.Formatter)
}
