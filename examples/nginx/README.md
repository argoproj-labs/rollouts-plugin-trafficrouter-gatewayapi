# Using NGINX Gateway Fabric with Argo Rollouts

[NGINX Gateway Fabric](https://github.com/nginx/nginx-gateway-fabric) is an open source project that implements the Gateway API using NGINX as the data plane. It provisions a dedicated NGINX instance for each Gateway resource.

Note that Argo Rollouts also [supports NGINX natively](https://argoproj.github.io/argo-rollouts/features/traffic-management/nginx/).

## Prerequisites

A Kubernetes cluster.

__Note:__ Refer to the [Technical Specifications](https://docs.nginx.com/nginx-gateway-fabric/overview/technical-specifications/) for supported Kubernetes versions.

Install the Gateway API CRDs:

```shell
kubectl kustomize "https://github.com/nginx/nginx-gateway-fabric/config/crd/gateway-api/standard?ref=v2.5.1" | kubectl apply -f -
```

Install NGINX Gateway Fabric:

```shell
helm install ngf oci://ghcr.io/nginx/charts/nginx-gateway-fabric --version 2.5.1 --create-namespace -n nginx-gateway
```

Wait for NGINX Gateway Fabric to become available:

```shell
kubectl wait --timeout=5m -n nginx-gateway deployment/ngf-nginx-gateway-fabric --for=condition=Available
```

## Step 1 - Create a Gateway resource

NGINX Gateway Fabric automatically provisions an NGINX instance when a Gateway resource is created.

```yaml title="gateway.yml"
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: argo-rollouts-gateway
  namespace: default
spec:
  gatewayClassName: nginx
  listeners:
    - protocol: HTTP
      name: web
      port: 80
```

Apply the file with `kubectl`:

```shell
cd examples/nginx
kubectl apply -f gateway.yml
```

Wait for the Gateway to be programmed:

```shell
kubectl wait --timeout=3m gateway/argo-rollouts-gateway -n default --for=condition=Programmed
```

Get the IP of your Gateway (NGINX Gateway Fabric creates a LoadBalancer service per Gateway):

```shell
export GATEWAY_IP=$(kubectl get svc argo-rollouts-gateway-nginx -n default -o jsonpath="{.status.loadBalancer.ingress[0].ip}{.status.loadBalancer.ingress[0].hostname}")
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

Create HTTPRoute and connect to the created Gateway resource:

```yaml title="httproute.yml"
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: argo-rollouts-http-route
spec:
  parentRefs:
    - name: argo-rollouts-gateway
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

## Step 4 - Create an example Rollout

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

Check the rollout:

```shell
export GATEWAY_IP=$(kubectl get svc argo-rollouts-gateway-nginx -n default -o jsonpath="{.status.loadBalancer.ingress[0].ip}{.status.loadBalancer.ingress[0].hostname}")
curl $GATEWAY_IP
```

Change the manifest to the `blue` tag and while the rollout is progressing you should see
the split traffic by visiting the IP of the gateway (see step 1)

```shell
kubectl patch rollout rollouts-demo -n default \
  --type='json' -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/image", "value":"argoproj/rollouts-demo:blue"}]'
```

Run the command and depending on the canary status you will sometimes see the red or blue version returned:

```shell
while true; do curl $GATEWAY_IP; done
```
