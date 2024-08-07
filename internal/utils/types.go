package utils

import (
	"context"

	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type CreateConfigMapOptions struct {
	Clientset v1.ConfigMapInterface
	Ctx       context.Context
}

type UpdateConfigMapOptions struct {
	Clientset    v1.ConfigMapInterface
	Ctx          context.Context
	ConfigMapKey string
}

type Task struct {
	Action        func() error
	ReverseAction func() error
}
