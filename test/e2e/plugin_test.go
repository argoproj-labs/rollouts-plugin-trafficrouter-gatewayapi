package e2e

import (
	"context"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var (
	global env.Environment
)

func TestMain(m *testing.M) {
	global = env.New()
	os.Exit(global.Run(m))
}

func setupEnvironment(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetFormatter(&logrus.TextFormatter{
		ForceColors:   true,
		PadLevelText:  true,
		FullTimestamp: true,
	})
	return ctx
}
