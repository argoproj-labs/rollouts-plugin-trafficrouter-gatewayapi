# Using Traefik Gateway API with Argo Rollouts

This guide will describe how to use [Traefik Proxy](https://traefik.io/) as an implementation
for the Gateway API in order to do split traffic with Argo Rollouts.

Versions used:

* Argo Rollouts [1.9.0](https://github.com/argoproj/argo-rollouts/releases)
* Argo Rollouts Gateway API plugin [0.13.0](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/releases)
* Traefik [3.7.0](https://github.com/traefik/traefik/releases/tag/v3.7.0) (Helm chart 40.0.0)
* Gateway API [1.5.1](https://github.com/kubernetes-sigs/gateway-api/releases/tag/v1.5.1)

## Prerequisites

A Kubernetes cluster.

__Note:__ Refer to the [Traefik compatibility documentation](https://doc.traefik.io/traefik/reference/install-configuration/providers/kubernetes/kubernetes-gateway/) for supported Kubernetes versions.

The Traefik Helm chart currently bundles the Gateway API CRDs, but this will change in a future version. Install them explicitly before installing Traefik:

```shell
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.5.1/standard-install.yaml
```

## Step 1 - Install Traefik with Gateway API support

Install Traefik with the Kubernetes Gateway provider enabled:

```shell
helm repo add traefik https://traefik.github.io/charts
helm repo update
helm install traefik traefik/traefik \
  --set providers.kubernetesGateway.enabled=true \
  --version 40.0.0 \
  --namespace traefik \
  --create-namespace
```

The Helm chart automatically installs the required RBAC resources and creates a GatewayClass named `traefik`.

Wait for Traefik to become available:

```shell
kubectl wait --timeout=5m -n traefik deployment/traefik --for=condition=Available
```

Verify the GatewayClass was created:

```shell
kubectl get gatewayclass
```

Make sure the `ACCEPTED` column shows `True`.

## Step 2 - Create a Gateway resource

The Helm chart creates an auto-managed Gateway in the `traefik` namespace that only accepts routes from the same namespace. For our application in the `default` namespace we create our own Gateway:

```yaml title="gateway.yml"
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: traefik-gateway
spec:
  gatewayClassName: traefik
  listeners:
    - protocol: HTTP
      name: web
      port: 8000
```

Apply the file with `kubectl`:

```shell
cd examples/traefik
kubectl apply -f gateway.yml
```

Get the IP of the Traefik LoadBalancer:

```shell
export GATEWAY_IP=$(kubectl get svc traefik -n traefik -o jsonpath="{.status.loadBalancer.ingress[0].ip}{.status.loadBalancer.ingress[0].hostname}")
echo $GATEWAY_IP
```

__Note:__ The Traefik service listens on port 80 externally and forwards to the `web` entrypoint (port 8000) internally. Use the service IP/hostname with port 80 when testing.

## Step 3 - Give access to Argo Rollouts for the Gateway/Http Route

Create Cluster Role resource with needed permissions for Gateway API provider.

```yaml title="cluster-role.yml"
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: gateway-controller-role
  namespace: argo-rollouts
rules:
  - apiGroups:
      - "*"
    resources:
      - "*"
    verbs:
      - "*"
```

__Note:__ These permissions are not very strict. You should lock them down according to your needs.

With the following role we allow Argo Rollouts to have write access to HTTPRoutes and Gateways.

```yaml title="cluster-role-binding.yml"
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: gateway-admin-rollouts
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: gateway-controller-role
subjects:
  - namespace: argo-rollouts
    kind: ServiceAccount
    name: argo-rollouts
```

Apply both files with `kubectl`:

```shell
kubectl apply -f cluster-role.yml
kubectl apply -f cluster-role-binding.yml
```

## Step 4 - Create HTTPRoute that defines a traffic split between two services

Create HTTPRoute and connect to the created Gateway resource:

```yaml title="httproute.yml"
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: argo-rollouts-http-route
spec:
  parentRefs:
    - name: traefik-gateway
      namespace: default
  rules:
    - backendRefs:
        - name: argo-rollouts-stable-service
          port: 80
        - name: argo-rollouts-canary-service
          port: 80
```

- Stable service

```yaml title="stable.yml"
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

- Canary service

```yaml title="canary.yml"
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

Apply the files with `kubectl`:

```shell
kubectl apply -f httproute.yml
kubectl apply -f stable.yml
kubectl apply -f canary.yml
```

## Step 5 - Create an example Rollout

Deploy a rollout to get the initial version.

Here is an example rollout:

```yaml title="rollout.yml"
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
            namespace: default
      steps:
        - setWeight: 30
        - pause: {}
        - setWeight: 40
        - pause: { duration: 10 }
        - setWeight: 60
        - pause: { duration: 10 }
        - setWeight: 80
        - pause: { duration: 10 }
        - setWeight: 100
        - pause: {}
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

Apply the file with `kubectl`:

```shell
kubectl apply -f rollout.yml
```

Check the rollout status:

```shell
kubectl argo rollouts get rollout rollouts-demo
```

Once the application is deployed you can visit your browser at `$GATEWAY_IP` or test from the command line:

```shell
export GATEWAY_IP=$(kubectl get svc traefik -n traefik -o jsonpath="{.status.loadBalancer.ingress[0].ip}{.status.loadBalancer.ingress[0].hostname}")
curl $GATEWAY_IP
```

## Step 6 - Test the canary

Change the Rollout to use a different color for the `argoproj/rollouts-demo` image:

```shell
kubectl patch rollout rollouts-demo -n default \
  --type='json' -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/image", "value":"argoproj/rollouts-demo:blue"}]'
```

The Gateway plugin will split the traffic to your canary by instructing Traefik via the Gateway API. Run the command below and depending on the canary status you will sometimes see the red or blue version returned:

```shell
while true; do curl $GATEWAY_IP; done
```

You can monitor the canary progress from the command line with:

```shell
watch kubectl argo rollouts get rollout rollouts-demo
```
