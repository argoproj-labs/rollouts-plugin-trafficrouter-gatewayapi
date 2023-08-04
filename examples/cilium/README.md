# Using Cilium Gateway API with Argo Rollouts

Cilium has [native support]() for the Gateway API making the integration with Argo Rollouts a straightforward process. Read more on eBPF and Cilium [here]().

## Prerequisites

- Cilium must be installed and configured with `kubeProxyReplacement=true`
- The CRDs for the Gateway API installed on your cluster:
```
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/gateway-api/v0.7.0/config/crd/standard/gateway.networking.k8s.io_gatewayclasses.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/gateway-api/v0.7.0/config/crd/standard/gateway.networking.k8s.io_gateways.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/gateway-api/v0.7.0/config/crd/standard/gateway.networking.k8s.io_httproutes.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/gateway-api/v0.7.0/config/crd/standard/gateway.networking.k8s.io_referencegrants.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/gateway-api/v0.7.0/config/crd/experimental/gateway.networking.k8s.io_tlsroutes.yaml

```
- [Cilium Gateway API Controller enabled](https://docs.cilium.io/en/stable/network/servicemesh/gateway-api/gateway-api/). This will create a `GatewayClass` object on your behalf.
- Similar to Ingress, Gateway API controller creates a service of LoadBalancer type, so your environment will need to support this (for example MetalLB if running locally).

## Step 1 - Create your Gateway object

Create a gateway:
```yaml
kind: Gateway
apiVersion: gateway.networking.k8s.io/v1beta1
metadata:
  name: cilium
spec:
  gatewayClassName: cilium
  listeners:
    - name: http
      protocol: HTTP
      port: 80
      allowedRoutes:
        namespaces:
          from: All
```

Get the IP of your Gateway
```shell
kubectl get gateway cilium -o=jsonpath="{.status.addresses[0].value}"
```

## Step 2 - Give access to Argo Rollouts for the Gateway/Http Route


Create Cluster Role resource with needed permissions for Gateway API provider.

```yaml
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
Note that these permission are not very strict. You should lock them down according to your needs.

With the following role we allow Argo Rollouts to have write access to Http Routes and Gateways.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: gateway-admin
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: gateway-controller-role
subjects:
  - namespace: argo-rollouts
    kind: ServiceAccount
    name: argo-rollouts
```

Apply both files with `kubectl`.

## Step 4 - Create HTTPRoute that defines a traffic split between two services
Create HTTPRoute and connect to the created Gateway resource

```yaml
kind: HTTPRoute
apiVersion: gateway.networking.k8s.io/v1beta1
metadata:
  name: argo-rollouts-http-route
spec:
  parentRefs:
  - kind: Gateway
    name: cilium
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

## Step 5 - Create an example Rollout

Deploy a rollout to get the initial version.

Here is an example rollout:

```yaml

apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: rollouts-demo
  namespace: default
spec:
  revisionHistoryLimit: 1
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

Change the manifest to the `v2` tag and while the rollout is progressing you should see
the split traffic by visiting the IP of the gateway (see step 2)

```shell
curl -H "host: demo.example.com" <IP>/call-me
```
Run the command above multiple times and depending on the canary status you will sometimes see "v1" returned and sometimes "v2"

