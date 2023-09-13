**Code:**
[![Go Report Card](https://goreportcard.com/badge/github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi)](https://goreportcard.com/report/github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi)
[![Gateway API plugin CI](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/actions/workflows/ci.yaml/badge.svg)](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/actions/workflows/ci.yaml)

**Social:**
[![Twitter Follow](https://img.shields.io/twitter/follow/argoproj?style=social)](https://twitter.com/argoproj)
[![Slack](https://img.shields.io/badge/slack-argoproj-brightgreen.svg?logo=slack)](https://argoproj.github.io/community/join-slack)

# Argo Rollouts Gateway API plugin

[Argo Rollouts](https://argoproj.github.io/rollouts/) is a progressive delivery controller for Kubernetes. It supports several advanced deployment methods such as blue/green and canaries.
For canary deployments Argo Rollouts can optionally use [a traffic provider](https://argoproj.github.io/argo-rollouts/features/traffic-management/) to split traffic between pods with full control and in a gradual way.

![Gateway API with traffic providers](public/images/gateway-api.png)

Until recently adding a new traffic provider to Argo Rollouts needed ad-hoc support code. With the adoption of the [Gateway API](https://gateway-api.sigs.k8s.io/), the integration becomes much easier as any traffic provider that implements the API will automatically be supported by Argo Rollouts.

## The Kubernetes Gateway API

The Gateway API is an open source project managed by the [SIG-NETWORK](https://github.com/kubernetes/community/tree/master/sig-network) community. It is a collection of resources that model service networking in Kubernetes.

See a [list of current projects](https://gateway-api.sigs.k8s.io/implementations/) that support the API.

## Prerequisites

You need the following

1. A Kubernetes cluster
2. An [installation](https://argoproj.github.io/argo-rollouts/installation/) of the Argo Rollouts controller
3. A traffic provider that [supports the Gateway API](https://gateway-api.sigs.k8s.io/implementations/)
4. An installation of the Gateway plugin 

Once everything is ready you need to create [a Rollout resource](https://argoproj.github.io/argo-rollouts/features/specification/) for all workloads that will use the integration.

## How to integrate Gateway API with Argo Rollouts

This is the installation process.

1. Enable Gateway Provider and create Gateway entrypoint
1. Create GatewayClass and Gateway resources
1. Create cluster entrypoint and map it with our Gateway entrypoint
1. Install Argo Rollouts in your cluster along with the Gateway API plugin
1. Create HTTPRoute
1. Create canary and stable services
1. Create argo-rollouts resources
1. Start a deployment

The first 3 steps are specific to your provider/implementation of the Gateway API inside the Kubernetes cluster. The rest of the steps are the same regardless of the specific implementation you chose.

See end-to-end examples for:

* [Cilium](examples/cilium)
* [EnvoyGateway](examples/envoygateway)
* [Google Cloud](examples/google-cloud)
* [Kong](examples/kong)
* [NGINX Kubernetes Gateway](examples/nginx/)
* [Traefik](examples/traefik/)



Note that these examples are included just for illustration purposes. You should be able
to use any solution that implements the Gateway API. 

## Installing the plugin

There are many ways to install the Gateway API plugin in Argo Rollouts. The easiest
one is to simply download it during startup.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argo-rollouts-config # must be so name
  namespace: argo-rollouts # must be in this namespace
data:
  trafficRouterPlugins: |-
    - name: "argoproj-labs/gatewayAPI"
      location: "https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/releases/download/v0.0.0-rc1/gateway-api-plugin-linux-amd64"
```

Deploy this file with `kubectl` or Argo CD or any other deployment method you use for your cluster.

For more details see the [Plugin documentation](https://argoproj.github.io/argo-rollouts/features/traffic-management/plugins/) at Argo Rollouts.

## Feedback needed

The gateway API plugin should cover all providers that support it. If you find an issue
please [tell us about it](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/issues)

If you also want to add an example with your favorite gateway API provider please send us a [Pull Request](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/pulls).



