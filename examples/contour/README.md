# Using Contour with Argo Rollouts

[Contour](https://projectcontour.io/) is an open source Kubernetes ingress controller that acts as a control plane for the Envoy edge and service proxy. It implements the Gateway API using a dynamic provisioning model where Contour automatically deploys and manages Envoy proxy instances in response to Gateway resources.

## Prerequisites

A Kubernetes cluster.

__Note:__ Refer to the [Compatibility Matrix](https://projectcontour.io/resources/compatibility-matrix/) for supported Kubernetes versions.

Install the Contour Gateway Provisioner (this also installs the required Gateway API CRDs):

```shell
kubectl apply -f https://projectcontour.io/quickstart/contour-gateway-provisioner.yaml
```

Wait for the Contour Gateway Provisioner to become available:

```shell
kubectl wait --timeout=5m -n projectcontour deployment/contour-gateway-provisioner --for=condition=Available
```

## Step 1 - Create Contour GatewayClass and Gateway object

Create a gateway:

```yaml title="gateway.yaml"
---
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: contour
spec:
  controllerName: projectcontour.io/gateway-controller
---
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: contour
  namespace: projectcontour
spec:
  gatewayClassName: contour
  listeners:
    - name: http
      protocol: HTTP
      port: 80
      allowedRoutes:
        namespaces:
          from: All
```

Apply the file with `kubectl`:

```shell
cd examples/contour
kubectl apply -f gateway.yaml
```

Wait for the Gateway to be programmed (Contour will automatically provision an Envoy instance):

```shell
kubectl wait --timeout=3m -n projectcontour gateway/contour --for=condition=Programmed
```

Get the IP of your Gateway:

```shell
export GATEWAY_IP=$(kubectl get gateway contour -n projectcontour -o=jsonpath="{.status.addresses[0].value}")
echo $GATEWAY_IP
```

## Step 2 - Give access to Argo Rollouts for the Gateway/Http Route

Create Cluster Role resource with needed permissions for Gateway API provider.

```yaml title="cluster-role.yaml"
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

```yaml title="cluster-role-binding.yaml"
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

Apply both files with `kubectl`:

```shell
kubectl apply -f cluster-role.yaml
kubectl apply -f cluster-role-binding.yaml
```

## Step 3 - Create HTTPRoute that defines a traffic split between two services

Create HTTPRoute and connect to the created Gateway resource.

__Note:__ The HTTPRoute must reference the Gateway by both name and namespace since the Gateway lives in the `projectcontour` namespace while the HTTPRoute lives in `default`.

```yaml title="httproute.yaml"
kind: HTTPRoute
apiVersion: gateway.networking.k8s.io/v1
metadata:
  name: argo-rollouts-http-route
  namespace: default
spec:
  parentRefs:
    - name: contour
      namespace: projectcontour
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

```yaml title="stable.yaml"
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

- Canary service

```yaml title="canary.yaml"
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

Apply the files with `kubectl`:

```shell
kubectl apply -f httproute.yaml
kubectl apply -f stable.yaml
kubectl apply -f canary.yaml
```

## Step 4 - Create an example Rollout

Deploy a rollout to get the initial version.

Here is an example rollout:

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
kubectl apply -f rollout.yaml
```

Check the rollout:

```shell
export GATEWAY_IP=$(kubectl get gateway contour -n projectcontour -o=jsonpath="{.status.addresses[0].value}")
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
