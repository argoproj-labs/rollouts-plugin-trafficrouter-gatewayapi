package utils

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
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
	assert.IsType(t, &hcLogJSONFormatter{}, entry.Logger.Formatter)
}

func TestSetupLog_JSONFormatterCaseInsensitive(t *testing.T) {
	entry := SetupLog("JSON")

	assert.NotNil(t, entry)
	assert.IsType(t, &hcLogJSONFormatter{}, entry.Logger.Formatter)
}

func TestSetupLog_JSONUsesHCLogKeys(t *testing.T) {
	entry := SetupLog("json")
	buf := &bytes.Buffer{}
	entry.Logger.SetOutput(buf)

	entry.Info("hello from plugin")

	var raw map[string]string
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &raw))
	assert.Equal(t, "hello from plugin", raw["@message"])
	assert.Equal(t, "info", raw["@level"])
	assert.NotEmpty(t, raw["@timestamp"])

	level := hclog.LevelFromString(raw["@level"])
	assert.Equal(t, hclog.Info, level)

	_, err := time.Parse(hclog.TimeFormatJSON, raw["@timestamp"])
	assert.NoError(t, err)
}

func TestSetupLog_JSONNormalizesWarningLevel(t *testing.T) {
	logger := log.New()
	logger.SetFormatter(newHCLogJSONFormatter())
	buf := &bytes.Buffer{}
	logger.SetOutput(buf)

	logger.Warn("warn message")

	var raw map[string]string
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &raw))
	assert.Equal(t, "warn", raw["@level"])
	assert.Equal(t, hclog.Warn, hclog.LevelFromString(raw["@level"]))
}
