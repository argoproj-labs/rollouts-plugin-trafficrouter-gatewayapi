package utils

import (
	"testing"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

func TestSetupLog_DefaultIsTextFormat(t *testing.T) {
	logger := SetupLog("text")

	assert.NotNil(t, logger)
	assert.Implements(t, (*hclog.Logger)(nil), logger)
}

func TestSetupLog_EmptyIsTextFormat(t *testing.T) {
	logger := SetupLog("")

	assert.NotNil(t, logger)
	assert.Implements(t, (*hclog.Logger)(nil), logger)
}

func TestSetupLog_JSONFormat(t *testing.T) {
	logger := SetupLog("json")

	assert.NotNil(t, logger)
	assert.Implements(t, (*hclog.Logger)(nil), logger)
}

func TestSetupLog_JSONFormatCaseInsensitive(t *testing.T) {
	logger := SetupLog("JSON")

	assert.NotNil(t, logger)
	assert.Implements(t, (*hclog.Logger)(nil), logger)
}
