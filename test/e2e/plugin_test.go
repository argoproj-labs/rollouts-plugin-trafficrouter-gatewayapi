package e2e

import (
	"context"
	"os"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var (
    global env.Environment
)

func envSetup(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
	return ctx, nil
}

func envCleanUp(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
	return ctx, nil
}

func TestMain(m *testing.M) {
	global = env.New()
	global.Setup(envSetup)
	global.Finish(envCleanUp)
	os.Exit(global.Run(m))
}