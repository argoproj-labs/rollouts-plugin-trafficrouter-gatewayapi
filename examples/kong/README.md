# Using Kong Gateway with Argo Rollouts

[Kong Kubernetes Ingress Controller](https://developer.konghq.com/kubernetes-ingress-controller/) has native support for the Gateway API making the integration with Argo Rollouts a straightforward process.

Note that Argo Rollouts also [supports Kong natively via its NGINX-based ingress](https://argoproj.github.io/argo-rollouts/features/traffic-management/nginx/).

## Prerequisites

A Kubernetes cluster.

__Note:__ Refer to the [compatibility documentation](https://docs.konghq.com/kubernetes-ingress-controller/latest/support/version-support-policy/) for supported Kubernetes versions.

Kong does not install the Gateway API CRDs by default. Install them before installing Kong, as the Helm chart detects the APIs and configures the correct roles automatically:

```shell
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.3.0/standard-install.yaml
```

Install Kong Ingress Controller:

```shell
helm repo add kong https://charts.konghq.com
helm repo update
helm install kong kong/ingress --version 0.24.0 -n kong --create-namespace
```

Wait for Kong to become available:

```shell
kubectl wait --timeout=5m -n kong deployment/kong-controller --for=condition=Available
kubectl wait --timeout=5m -n kong deployment/kong-gateway --for=condition=Available
```

## Step 1 - Create GatewayClass and Gateway

Create the GatewayClass (created only once per cluster):

```yaml title="gatewayclass.yml"
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: kong
  annotations:
    konghq.com/gatewayclass-unmanaged: 'true'
spec:
  controllerName: konghq.com/kic-gateway-controller
```

Create a Gateway:

```yaml title="gateway.yml"
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: kong
spec:
  gatewayClassName: kong
  listeners:
  - name: proxy
    port: 80
    protocol: HTTP
```

Apply the files with `kubectl`:

```shell
cd examples/kong
kubectl apply -f gatewayclass.yml
kubectl apply -f gateway.yml
```

Get the IP of the Kong proxy service:

```shell
export GATEWAY_IP=$(kubectl get svc kong-gateway-proxy -n kong -o jsonpath="{.status.loadBalancer.ingress[0].ip}{.status.loadBalancer.ingress[0].hostname}")
echo $GATEWAY_IP
```

## Step 2 - Give access to Argo Rollouts for the Gateway/Http Route

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

## Step 3 - Create HTTPRoute that defines a traffic split between two services

Create HTTPRoute and connect to the created Gateway resource:

```yaml title="httproute.yml"
kind: HTTPRoute
apiVersion: gateway.networking.k8s.io/v1
metadata:
  name: argo-rollouts-http-route
  annotations:
    konghq.com/strip-path: 'true'
spec:
  parentRefs:
  - kind: Gateway
    name: kong
  hostnames:
  - "demo.example.com"
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /
    backendRefs:
    - name: argo-rollouts-stable-service
      kind: Service
      port: 80
    - name: argo-rollouts-canary-service
      kind: Service
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

## Step 4 - Create an example Rollout

Deploy a rollout to get the initial version.

Here is an example rollout:

```yaml title="rollout.yml"
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: rollouts-demo
  namespace: default
spec:
  replicas: 10
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
      - setWeight: 60
      - pause: {}
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
          image: kostiscodefresh/summer-of-k8s-app:v1
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

Check the rollout:

```shell
export GATEWAY_IP=$(kubectl get svc kong-gateway-proxy -n kong -o jsonpath="{.status.loadBalancer.ingress[0].ip}{.status.loadBalancer.ingress[0].hostname}")
curl -H "host: demo.example.com" $GATEWAY_IP/callme
```

The output should be:

```shell
<div class='pod' style='background:#44B3C2'> ver: 1.0
 </div>%
```

Change the manifest to the `v2` tag and while the rollout is progressing you should see
the split traffic by visiting the IP of the gateway (see step 1)

```shell
kubectl patch rollout rollouts-demo -n default \
  --type='json' -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/image", "value":"kostiscodefresh/summer-of-k8s-app:v2"}]'
```

Run the command and depending on the canary status you will sometimes see "v1" returned and sometimes "v2"

```shell
while true; do curl -H "host: demo.example.com" $GATEWAY_IP/callme; done
```
