package utils

import (
	"errors"
	"fmt"
	"path/filepath"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func GetKubeConfig() (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeConfig := filepath.Join("~", ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
		if err != nil {
			errorString := fmt.Sprintf("The kubeconfig cannot be loaded: %v\n", err)
			fmt.Println(errorString)
			return nil, errors.New(errorString)
		}
	}
	return config, nil
}
