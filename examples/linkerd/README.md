# Using NGINX Kubernetes Gateway with Argo Rollouts

This guide will describe how to use NGINX Kubernetes Gateway as an implementation
for the Gateway API in order to do split traffic with Argo Rollouts.

Note that Argo Rollouts also [supports NGINX natively](https://argoproj.github.io/argo-rollouts/features/traffic-management/nginx/).

## Step 1 - Enable Gateway Provider and create Gateway entrypoint

Before enabling a Gateway Provider you also need to install NGINX Kubernetes Gateway. Follow the official [installation instructions](https://docs.nginx.com/nginx-gateway-fabric/installation/installing-ngf/helm/).

This installation will create an `nginx` gateway class that we can use later on.


1. If not already done through the previous installation instructions, register the [Gateway API CRDs](https://gateway-api.sigs.k8s.io/guides/#install-standard-channel)

```
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v0.8.0/standard-install.yaml
```

## Step 2 - Create a Gateway resource and HTTPRoute that defines a traffic split


After we deployed the Gateway API provider and Gateway class, we can create a Gateway resource:


```yaml
apiVersion: gateway.networking.k8s.io/v1beta1
kind: Gateway
metadata:
  name: argo-rollouts-gateway
spec:
  gatewayClassName: nginx
  listeners:
    - protocol: HTTP
      name: web
      port: 80 # one of Gateway entrypoint that was created following the official installation instructions
```

Create HTTPRoute and connect to the created Gateway resource:

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

## Step 3 - Create canary and stable services for your application

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
## Step 4 - Grant argo-rollouts permissions to view and modify Gateway HTTPRoute resources

The argo-rollouts service account needs the ability to be able to view and modify HTTPRoutes as well as its existing permissions. Edit the `argo-rollouts` cluster role to add the following permissions:

```yaml
rules:
- apiGroups:
  - gateway.networking.k8s.io
  resources:
  - httproutes
  verbs:
  - get
  - list
  - watch
  - update
  - patch
```

## Step 5 - Create argo-rollouts resources

We can finally create the definition of the application.

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
        plugins:
          argoproj-labs/gatewayAPI:
            httpRoute: argo-rollouts-http-route # our created httproute
            namespace: default # namespace where this rollout resides.
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

Apply all the yaml files to your cluster

## Step 6 - Test the canary

Perform a deployment like any other Rollout and the Gateway plugin will split the traffic to your canary by instructing NGINX Gateway to proxy via the Gateway API


