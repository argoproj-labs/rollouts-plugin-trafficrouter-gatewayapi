package e2e

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/e2e-framework/pkg/env"
)

var (
	global env.Environment
)

func TestMain(m *testing.M) {
	global = env.New()
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetFormatter(&logrus.TextFormatter{
		ForceColors:  true,
		PadLevelText: true,
	})
	os.Exit(global.Run(m))
}
