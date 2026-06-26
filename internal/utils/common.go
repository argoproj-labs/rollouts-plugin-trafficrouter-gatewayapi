package utils

import (
	"os"
	"strings"

	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func GetKubeConfig() (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// if you want to change the loading rules (which files in which order), you can do so here
	configOverrides := &clientcmd.ConfigOverrides{}
	// if you want to change override values or bind them to flags, there are methods to help you
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, pluginTypes.RpcError{ErrorString: err.Error()}
	}
	return config, nil
}

func SetupLog() *log.Entry {
	logger := log.New()
	logger.SetLevel(log.InfoLevel)
	if strings.EqualFold(os.Getenv("LOG_FORMAT"), "json") {
		logger.SetFormatter(&log.JSONFormatter{})
	} else {
		logger.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	}
	return logger.WithFields(log.Fields{"plugin": "trafficrouter"})
}
