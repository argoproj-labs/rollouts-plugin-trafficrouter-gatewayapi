package main

import (
	"flag"

	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/internal/utils"
	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/pkg/plugin"

	rolloutsPlugin "github.com/argoproj/argo-rollouts/rollout/trafficrouting/plugin/rpc"
	goPlugin "github.com/hashicorp/go-plugin"
)

// handshakeConfigs are used to just do a basic handshake between
// a plugin and host. If the handshake fails, a user friendly error is shown.
// This prevents users from executing bad plugins or executing a plugin
// directory. It is a UX feature, not a security feature.
var handshakeConfig = goPlugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "ARGO_ROLLOUTS_RPC_PLUGIN",
	MagicCookieValue: "trafficrouter",
}

func main() {
	// Define and parse flags for your command line options:
	kubeClientQPS := flag.Int("kubeClientQPS", 5, "The QPS to use for the Kubernetes client.")
	kubeClientBurst := flag.Int("kubeClientBurst", 10, "The Burst to use for the Kubernetes client.")
	logFormat := flag.String("logformat", "text", "Set the logging format. One of: text|json")
	kubeconfig := flag.String("kubeconfig", "", "Path to kubeconfig file. If not set, uses default discovery.")
	pluginAlias := flag.String("pluginAlias", "", "Alias for the plugin name used in the Rollout configuration. Defaults to argoproj-labs/gatewayAPI.")
	flag.Parse()

	// Create the plugin implementation, injecting command line options:
	rpcPluginImp := &plugin.RpcPlugin{
		CommandLineOpts: plugin.CommandLineOpts{
			KubeClientQPS:   float32(*kubeClientQPS),
			KubeClientBurst: *kubeClientBurst,
			KubeConfigPath:  *kubeconfig,
			PluginAlias:     *pluginAlias,
		},
		LogCtx: utils.SetupLog(*logFormat),
	}

	pluginMap := map[string]goPlugin.Plugin{
		"RpcTrafficRouterPlugin": &rolloutsPlugin.RpcTrafficRouterPlugin{Impl: rpcPluginImp},
	}

	goPlugin.Serve(&goPlugin.ServeConfig{
		HandshakeConfig: handshakeConfig,
		Plugins:         pluginMap,
	})
}
