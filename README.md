**Code:** 
[![Go Report Card](https://goreportcard.com/badge/github.com/argoproj-labs/rollouts-gatewayapi-trafficrouter-plugin)](https://goreportcard.com/report/github.com/argoproj-labs/rollouts-gatewayapi-trafficrouter-plugin)
[![Gateway API plugin CI](https://github.com/argoproj-labs/rollouts-gatewayapi-trafficrouter-plugin/actions/workflows/ci.yaml/badge.svg)](https://github.com/argoproj-labs/rollouts-gatewayapi-trafficrouter-plugin/actions/workflows/ci.yaml)

**Social:**
[![Twitter Follow](https://img.shields.io/twitter/follow/argoproj?style=social)](https://twitter.com/argoproj)
[![Slack](https://img.shields.io/badge/slack-argoproj-brightgreen.svg?logo=slack)](https://argoproj.github.io/community/join-slack)

# Argo Rollouts Gateway API plugin



Argo Rollouts is a progressivey delivery controller for Kubernetes. It supports several advanced deployment methods such as blue/green and canaries.
For canary deployments Argo Rollouts can optionally use a traffic provider to split traffic between pods with full control and in a gradual way.

![Gateway API with traffic providers](public/images/gateway-api.png)

Until recently adding a new traffic provider to Argo Rollouts needed ad-hoc support code. With the adoption of the [Gateway API](https://gateway-api.sigs.k8s.io/), the integration becomes much easier as any traffic provider that implements the API will automatically be supported by Argo Rollouts.

## The Kubernetes Gateway API

The Gateway API is an open source project managed by the [SIG-NETWORK](https://github.com/kubernetes/community/tree/master/sig-network) community. It is a collection of resources that model service networking in Kubernetes.

See a [list of current projects](https://gateway-api.sigs.k8s.io/implementations/) that support the API.

## Prerequisites

You need the following

1. A Kubernetes cluster
2. An [installation](https://argoproj.github.io/argo-rollouts/installation/) of the Argo Rollouts controller 
3. A traffic provider that supports the Gateway API (e.g. [Traefik Proxy](https://traefik.io/))
4. An installation of the Gateway plugin as described below

Once everything is ready you need to create [a Rollout resource](https://argoproj.github.io/argo-rollouts/features/specification/) for all workloads that will use the integration.

## How to integrate Gateway API with Argo Rollouts

1. Enable Gateway Provider and create Gateway entrypoint
2. Create GatewayClass and Gateway resources
3. Create cluster entrypoint and map it with our Gateway entrypoint
4. Create HTTPRoute
5. Create canary and stable services
6. Create argo-rollouts resources
7. Create config map from where argo rollouts takes information where to search our binary plugin file for Gateway API
8. Install binary of this plugin and put it in the location that you specified in config map

We will go through all these steps together with an example Traefik

### Enable Gateway Provider and create Gateway entrypoint

Before enabling Gateway Provider oyu also need to install traefik. How to install it you can find [here](https://doc.traefik.io/traefik/getting-started/install-traefik/).

Every contoller has its own instruction how we need to enable Gateway API provider. I will follow to the instructions of [Traefik controller](https://doc.traefik.io/traefik/providers/kubernetes-gateway/)

1. Register [Gateway API CRD](https://gateway-api.sigs.k8s.io/guides/#install-standard-channel)

```
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v0.6.1/standard-install.yaml
```

2. Create the same deployment resource with service account

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: traefik
spec:
  replicas: 1
  selector:
    matchLabels:
      app: argo-rollouts-traefik-lb
  template:
    metadata:
      labels:
        app: argo-rollouts-traefik-lb
    spec:
      serviceAccountName: traefik-controller
      containers:
        - name: traefik
          image: traefik:v2.9
          args:
            - --entrypoints.web.address=:80
            - --entrypoints.websecure.address=:443
            - --experimental.kubernetesgateway
            - --providers.kubernetesgateway
          ports:
            - name: web
              containerPort: 80
```

3. Create the same ServiceAccount
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: traefik-controller
```

4. Create Cluster Role resource with needing permissions for Gateway API provider

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  name: traefik-controller-role
  namespace: aws-local-runtime
rules:
  - apiGroups:
      - "*"
    resources:
      - "*"
    verbs:
      - "*"
```

5. Create Cluster Role Binding

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: traefik-admin
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: traefik-controller-role
subjects:
  - namespace: default
    kind: ServiceAccount
    name: traefik-controller
```

### Create GatewayClass and Gateway resources

After we enabled Gateway API provider in our controller we can create GatewayClass and Gateway:

- GatewayClass

```yaml
apiVersion: gateway.networking.k8s.io/v1beta1
kind: GatewayClass
metadata:
  name: argo-rollouts-gateway-class
spec:
  controllerName: traefik.io/gateway-controller
```

- Gateway

```yaml
apiVersion: gateway.networking.k8s.io/v1beta1
kind: Gateway
metadata:
  name: argo-rollouts-gateway
spec:
  gatewayClassName: argo-rollouts-gateway-class
  listeners:
    - protocol: HTTP
      name: web
      port: 80 # one of Gateway entrypoint that we created at 1 step
```

### Create cluster entrypoint and map it with our Gateway entrypoint

In different controllers entry points can be created differently. For Traefik controller we can create entry point like this:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: argo-rollouts-traefik-lb
spec:
  type: LoadBalancer
  selector:
    app: argo-rollouts-traefik-lb # selector of Gateway provider(step 1)
  ports:
    - protocol: TCP
      port: 8080
      targetPort: web # map with Gateway entrypoint
      name: web
```

### Create HTTPRoute

Create HTTPRoute and connect to the created Gateway resource

```yaml
apiVersion: gateway.networking.k8s.io/v1beta1
kind: HTTPRoute
metadata:
  name: argo-rollouts-http-route
spec:
  parentRefs:
    - name: argo-rollouts-gateway
  rules:
    - backendRefs:
        - name: argo-rollouts-stable-service
          port: 80
        - name: argo-rollouts-canary-service
          port: 80
```

### Create canary and stable services

- Canary service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: argo-rollouts-canary-service
spec:
  ports:
    - port: 80
      targetPort: http
      protocol: TCP
      name: http
  selector:
    app: rollouts-demo
```

- Stable service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: argo-rollouts-stable-service
spec:
  ports:
    - port: 80
      targetPort: http
      protocol: TCP
      name: http
  selector:
    app: rollouts-demo
```

### Create argo-rollouts resources

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: rollouts-demo
spec:
  replicas: 5
  strategy:
    canary:
      canaryService: argo-rollouts-canary-service # our created canary service
      stableService: argo-rollouts-stable-service # our created stable service
      trafficRouting:
        plugin:
          gatewayAPI:
            httpRoute: argo-rollouts-http-route # our created httproute
      steps:
        - setWeight: 30
        - pause: {}
        - setWeight: 40
        - pause: { duration: 10 }
        - setWeight: 60
        - pause: { duration: 10 }
        - setWeight: 80
        - pause: { duration: 10 }
  revisionHistoryLimit: 2
  selector:
    matchLabels:
      app: rollouts-demo
  template:
    metadata:
      labels:
        app: rollouts-demo
    spec:
      containers:
        - name: rollouts-demo
          image: argoproj/rollouts-demo:red
          ports:
            - name: http
              containerPort: 8080
              protocol: TCP
          resources:
            requests:
              memory: 32Mi
              cpu: 5m
```

### Create config map

You can specify any name inside *trafficRouterPlugins* you would like but it should be the same as in argo rollouts you specify under the key *plugins*.
The value of location key also can be any path or url where your binary plugin file is located

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argo-rollouts-config # must be so name
  namespace: argo-rollouts # must be in this namespace
data:
  trafficRouterPlugins: |-
    - name: "argoproj-labs/gatewayAPI"
      location: "file:///Users/test/go/src/github.com/argoproj-labs/rollouts-trafficrouter-gatewayapi-plugin/gatewayapi-plugin"
```

### Install binary of this plugin and put it in the location that you specified in config map
