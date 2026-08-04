package utils

import (
	"strings"

	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	hclog "github.com/hashicorp/go-hclog"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func GetKubeConfig() (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, pluginTypes.RpcError{ErrorString: err.Error()}
	}
	return config, nil
}

func SetupLog(logFormat string) hclog.Logger {
	jsonFormat := strings.EqualFold(logFormat, "json")
	return hclog.New(&hclog.LoggerOptions{
		Name:       "plugin.gatewayAPI",
		Level:      hclog.Info,
		JSONFormat: jsonFormat,
	})
}
