# Using Traefik Gateway API with Argo Rollouts

This guide will describe how to use Traefik proxy an an implementation
for the Gateway API in order to do split traffic with Argo Rollouts.

Note that Argo Rollouts also [supports Traefik natively](https://argoproj.github.io/argo-rollouts/features/traffic-management/traefik/).

## Step 1 - Enable Gateway Provider and create Gateway entrypoint

Before enabling a Gateway Provider you also need to install Traefik. Follow the official [installation instructions](https://doc.traefik.io/traefik/getting-started/install-traefik/).

You should also read the documentation on how [Traefik implements the Gateway API](https://doc.traefik.io/traefik/providers/kubernetes-gateway/).

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

4. Create Cluster Role resource with needed permissions for Gateway API provider.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
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

Note that these permission are not very strict. You should lock them down according to your needs.

5. Create Cluster Role Binding

With the following role we allow Traefik to have write access to Http Routes and Gateways.

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

## Step 2 - Create GatewayClass and Gateway resources

After we enabled the Gateway API provider in our controller we can create a GatewayClass and Gateway:

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

## Step 3 - Create cluster entrypoint and map it with our Gateway entrypoint

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

This concludes the setup that is specific to Traefik Proxy. The rest of the steps are generic to any implementation of the Gateway API.

## Step 4 - Create HTTPRoute that defines a traffic split

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

## Step 5 - Create canary and stable services for your application

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
## Step 6 - Grant argo-rollouts permissions to view and modify Gateway HTTPRoute resources

The argo-rollouts service account needs the ability to be able to view and mofiy HTTPRoutes as well as its existing permissions. Edit the `argo-rollouts` cluster role to add the following permissions:

```yaml
rules:
- apiGroups:
  - gateway.networking.k8s.io
  resources:
  - httproutes
  verbs:
  - '*'
```

## Step 7 - Create argo-rollouts resources

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

## Step 8 - Test the canary

Perform a deployment like any other Rollout and the Gateway plugin will split the traffic to your canary by instructing Traefik proxy via the Gateway API.

### Notice

GatewayAPI plugin supports traffic routing based on a header values for canary, so you can also use setHeaderRoute step in Argo Rollouts manifest. It also means that plugin should control managed routes. It creates ConfigMap in the specified namespace in **namespace** field with specified name in **configMap** field for that.
```yaml
plugins:
  argoproj-labs/gatewayAPI:
    namespace: test # default value is default
    httpRoute: http-route
    configMap: test-gateway # default value is argo-gatewayapi-configmap
```

## How to use  multiple routes per rollout

## Step 1 - Create several routes

```yaml
apiVersion: gateway.networking.k8s.io/v1beta1
kind: HTTPRoute
metadata:
  name: first-route
spec:
  parentRefs:
    - name: gateway
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /first
    backendRefs:
    - name: argo-rollouts-stable-service
      kind: Service
      port: 80
    - name: argo-rollouts-canary-service
      kind: Service
      port: 80
---
apiVersion: gateway.networking.k8s.io/v1beta1
kind: HTTPRoute
metadata:
  name: second-route
spec:
  parentRefs:
    - name: gateway
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /second  
    backendRefs:
    - name: argo-rollouts-stable-service
      kind: Service
      port: 80
    - name: argo-rollouts-canary-service
      kind: Service
      port: 80
```

## Step 2 - Change argoproj-labs/gatewayAPI field in Argo Rollout manifest

```yaml
plugins:
  argoproj-labs/gatewayAPI:
    httpRoutes:
       - name: first-route # required
         useHeaderRoutes: true
       - name: second-route
```
You can control for what routes you need to add header routes during step of setHeaderRoute in Argo Rollout.

**Notice** All these features except traffic routing based on a header values for canary work also with TCPRoutes 
