# Using Gloo Gateway with Argo Rollouts

[Gloo Gateway](https://docs.solo.io/gloo-gateway/v2/) is a cloud-native Layer 7 proxy that is based on the [Kubernetes Gateway API](https://gateway-api.sigs.k8s.io/).
## Prerequisites

* Kubernetes cluster with minimum version 1.23

### Install K8s Gateway API CRDs
```shell
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.0.0/standard-install.yaml
```

### Install Gloo Gateway

```shell
helm install default -n gloo-system --create-namespace \
    oci://ghcr.io/solo-io/helm-charts/gloo-gateway \
    --version 2.0.0-beta1 \
    --wait --timeout 1m
```

The following `GatewayClass` resource is automatically created as part of the helm installion:

```yaml
apiVersion: gateway.networking.k8s.io/v1beta1
kind: GatewayClass
metadata:
  name: gloo-gateway
spec:
  controllerName: solo.io/gloo-gateway
```

The presence of this `GatewayClass` enables us to define `Gateways` which will then dynamically provision proxies to handle incoming traffic.

Let's confirm that the `GatewayClass` resource is created correctly:

```shell
kubectl wait --timeout=1m -n gloo-system gatewayclass/gloo-gateway --for=condition=Accepted
```

### Install Argo Rollouts

```shell
kubectl create namespace argo-rollouts
kubectl apply -n argo-rollouts -f https://github.com/argoproj/argo-rollouts/releases/latest/download/install.yaml
```
See the [installation docs](https://argo-rollouts.readthedocs.io/en/stable/installation) for more detail.

### Install Argo Rollout Gateway API Plugin

```shell
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: argo-rollouts-config # must be so name
  namespace: argo-rollouts # must be in this namespace
data:
  trafficRouterPlugins: |-
    - name: "argoproj-labs/gatewayAPI"
      location: "https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/releases/download/v0.0.0-rc1/gateway-api-plugin-linux-amd64"
EOF
```

See the [project README](/README.md#installing-the-plugin) for more info.

You may need to restart the Argo Rollouts pod for the plugin to take effect
```shell
kubectl rollout restart deployment -n argo-rollouts argo-rollouts
```


## Step 1 - Create Gateway object

Now we will actually configure Gloo Gateway and Argo Rollouts to manage the progressive deployment of a test application. All of the following resources are located in the `examples/gloo-gateway` directory.

```shell
cd examples/gloo-gateway
```

Create a gateway:

```yaml
apiVersion: gateway.networking.k8s.io/v1beta1
kind: Gateway
metadata:
  name: gloo
  namespace: default
spec:
  gatewayClassName: gloo-gateway
  listeners:
    - name: http
      protocol: HTTP
      port: 80
```

Apply the file:

```shell
kubectl apply -f gateway.yaml
```

## Step 2 - Configure RBAC to allow Argo Rollouts to control HTTPRoute resources

Create a `ClusterRole` resource with permissions to manage `HTTPRoute` resources:

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

__Note:__ This `ClusterRole` is overly permissive and is provided __only for demo purposes__.

Now we will create a binding to give the Argo Rollouts `ServiceAccount` these permissions:

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

Apply the file:

```shell
kubectl apply -f rbac.yaml
```

## Step 4 - Create HTTPRoute to route to a stable and canary service
Create HTTPRoute associated with the `Gateway` to be managed by Argo Rollouts:

```yaml
kind: HTTPRoute
apiVersion: gateway.networking.k8s.io/v1beta1
metadata:
  name: argo-rollouts-http-route
  namespace: default
spec:
  parentRefs:
    - name: gloo
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

Create the stable service:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: argo-rollouts-stable-service
  namespace: default
spec:
  ports:
    - port: 80
      targetPort: http
      protocol: TCP
      name: http
  selector:
    app: rollouts-demo
```

Create the canary service:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: argo-rollouts-canary-service
  namespace: default
spec:
  ports:
    - port: 80
      targetPort: http
      protocol: TCP
      name: http
  selector:
    app: rollouts-demo
```

Apply the files:

```shell
kubectl apply -f httproute.yaml
kubectl apply -f stable.yaml
kubectl apply -f canary.yaml
```


## Step 5 - Create an example Rollout

Deploy a rollout to get the initial version.

```yaml title="rollout.yaml"
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: rollouts-demo
  namespace: default
spec:
  replicas: 3
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
      - pause: { duration: 30s }
      - setWeight: 60
      - pause: { duration: 30s }
      - setWeight: 100
      - pause: { duration: 30s }
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

Apply the file:

```shell
kubectl apply -f rollout.yaml
```

Check the rollout by using `curl` to make a request: 
```shell
export GATEWAY_IP=$(kubectl get gateway gloo -o=jsonpath="{.status.addresses[0].value}")
curl -H "host: demo.example.com" $GATEWAY_IP/callme
```

This command got the IP of the proxy directly from the `Status` field of the `Gateway` resource. Alternatively, you can port-forward to the pod.

The output should be:

```shell
<div class='pod' style='background:#44B3C2'> ver: 1.0
 </div>%
```

Change the manifest to the `v2` tag and while the rollout is progressing you should see
the split traffic by visiting the IP of the gateway (see step 2)

```shell
kubectl patch rollout rollouts-demo -n default \
  --type='json' -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/image", "value":"kostiscodefresh/summer-of-k8s-app:v2"}]'
```

Run the command and depending on the canary status you will sometimes see "v1" returned and sometimes "v2"
```shell
while true; do curl -H "host: demo.example.com" $GATEWAY_IP/callme; done
```
