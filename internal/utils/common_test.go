package utils

import (
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
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
