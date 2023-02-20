package mocks

import (
	"context"
	"errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/gateway-api/apis/v1beta1"
)

type FakeClient struct {
	IsGetError    bool
	IsUpdateError bool
}

var (
	port         = v1beta1.PortNumber(80)
	weight int32 = 0
)

var httpRouteObj = v1beta1.HTTPRoute{
	ObjectMeta: metav1.ObjectMeta{
		Name: "argo-rollouts-http-route",
	},
	Spec: v1beta1.HTTPRouteSpec{
		CommonRouteSpec: v1beta1.CommonRouteSpec{
			ParentRefs: []v1beta1.ParentReference{
				{
					Name: "argo-rollouts-gateway",
				},
			},
		},
		Rules: []v1beta1.HTTPRouteRule{
			{
				BackendRefs: []v1beta1.HTTPBackendRef{
					{
						BackendRef: v1beta1.BackendRef{
							BackendObjectReference: v1beta1.BackendObjectReference{
								Name: "argo-rollouts-stable-service",
								Port: &port,
							},
							Weight: &weight,
						},
					},
					{
						BackendRef: v1beta1.BackendRef{
							BackendObjectReference: v1beta1.BackendObjectReference{
								Name: "argo-rollouts-canary-service",
								Port: &port,
							},
							Weight: &weight,
						},
					},
				},
			},
		},
	},
}

func (f *FakeClient) Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1beta1.HTTPRoute, error) {
	if f.IsGetError {
		return &httpRouteObj, errors.New("gateway API get error")
	}
	return &httpRouteObj, nil
}

func (f *FakeClient) Update(ctx context.Context, httpRoute *v1beta1.HTTPRoute, opts metav1.UpdateOptions) (*v1beta1.HTTPRoute, error) {
	if f.IsUpdateError {
		return httpRoute, errors.New("gateway API update error")
	}
	return httpRoute, nil
}
