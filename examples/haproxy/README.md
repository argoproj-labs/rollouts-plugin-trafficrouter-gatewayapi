# Using HAProxy Ingress with Argo Rollouts

This guide will describe how to use [HAProxy Ingress](https://haproxy-ingress.github.io/) as an implementation
for the Gateway API in order to do split traffic with Argo Rollouts.

Versions used:

* Argo Rollouts [1.9.1](https://github.com/argoproj/argo-rollouts/releases) (Helm chart 2.41.1)
* Argo Rollouts Gateway API plugin [0.16.0](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/releases)
* HAProxy Ingress [v0.17.0-alpha.2](https://github.com/jcmoraisjr/haproxy-ingress/releases/tag/v0.17.0-alpha.2) (embedded HAProxy 3.0.23)
* Gateway API [1.5.1](https://github.com/kubernetes-sigs/gateway-api/releases/tag/v1.5.1)

## Prerequisites

A Kubernetes cluster.

__Note:__ There are two projects that call themselves an HAProxy ingress controller. The one used
here is the community [HAProxy Ingress](https://haproxy-ingress.github.io/) controller, which is the
[Gateway API conformant](https://gateway-api.sigs.k8s.io/implementations/) HAProxy implementation.
Gateway API conformance arrived in **v0.17**, which is still a pre-release at the time of writing, so
the Helm commands below need `--devel`. Earlier versions have partial Gateway API support and will not
work with this plugin.

Install the Gateway API CRDs (standard channel):

```shell
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.5.1/standard-install.yaml
```

__Note:__ HAProxy Ingress looks for the Gateway API CRDs only once, at startup. Install the CRDs
**before** the controller. If you install them afterwards, restart the controller pods:

```shell
kubectl --namespace ingress-controller delete pod -l app.kubernetes.io/name=haproxy-ingress
```

Install Argo Rollouts together with this plugin. See the [installation guide](../../docs/installation.md)
for all the options, or use the Helm chart directly:

```shell
helm repo add argo https://argoproj.github.io/argo-helm
helm repo update
helm install argo-rollouts argo/argo-rollouts \
  --namespace argo-rollouts \
  --create-namespace \
  --set-json 'controller.trafficRouterPlugins=[{"name":"argoproj-labs/gatewayAPI","location":"https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/releases/download/v0.16.0/gatewayapi-plugin-linux-amd64"}]'
```

Check the controller log to confirm the plugin was picked up:

```shell
kubectl logs -n argo-rollouts deployment/argo-rollouts | grep gatewayAPI
```

You also need the [Argo Rollouts CLI](https://argo-rollouts.readthedocs.io/en/stable/features/kubectl-plugin/)
for the commands in step 5 and 6.

## Step 1 - Install HAProxy Ingress

```shell
helm repo add haproxy-ingress https://haproxy-ingress.github.io/charts
helm repo update
helm install haproxy-ingress haproxy-ingress/haproxy-ingress \
  --version 0.17.0-alpha.2 --devel \
  --namespace ingress-controller \
  --create-namespace
```

Wait for HAProxy Ingress to become available:

```shell
kubectl wait --timeout=5m -n ingress-controller deployment/haproxy-ingress --for=condition=Available
```

Unlike most other Gateway API implementations, HAProxy Ingress runs a single shared HAProxy
deployment and does **not** create one Service per Gateway. All Gateway traffic enters through the
chart's `haproxy-ingress` service:

```shell
export GATEWAY_IP=$(kubectl get svc haproxy-ingress -n ingress-controller -o jsonpath="{.status.loadBalancer.ingress[0].ip}{.status.loadBalancer.ingress[0].hostname}")
echo $GATEWAY_IP
```

__Note:__ If your cluster has no LoadBalancer implementation the service will stay in `<pending>`.
In that case port-forward instead and use `localhost:8080` in place of `$GATEWAY_IP` for the rest of
this guide:

```shell
kubectl port-forward -n ingress-controller svc/haproxy-ingress 8080:80
```

## Step 2 - Create a GatewayClass and a Gateway resource

The HAProxy Ingress Helm chart does not create a GatewayClass, so we create one ourselves:

```yaml title="gatewayclass.yml"
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: haproxy
spec:
  controllerName: haproxy-ingress.github.io/controller
```

Then a Gateway with an HTTP listener on port 80:

```yaml title="gateway.yml"
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: haproxy-gateway
spec:
  gatewayClassName: haproxy
  listeners:
    - protocol: HTTP
      name: http
      port: 80
```

Apply both files with `kubectl`:

```shell
cd examples/haproxy
kubectl apply -f gatewayclass.yml
kubectl apply -f gateway.yml
```

Verify them:

```shell
kubectl get gatewayclass haproxy
kubectl get gateway haproxy-gateway
```

Make sure the `ACCEPTED` column of the GatewayClass and the `PROGRAMMED` column of the Gateway both
show `True`.

__Note:__ The `ADDRESS` column of the Gateway stays empty. HAProxy Ingress only publishes an address
there if it is configured with a publish service. Use the `$GATEWAY_IP` from step 1 instead.

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
The minimum set the plugin actually requires is documented in the
[installation guide](../../docs/installation.md#permissions).

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
    - name: haproxy-gateway
      namespace: default
  rules:
    - backendRefs:
        - name: argo-rollouts-stable-service
          port: 80
        - name: argo-rollouts-canary-service
          port: 80
```

There is no `hostnames` field, so HAProxy serves this route on all hostnames. Add one if you want to
restrict the route to a specific domain.

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
            namespace: default # namespace where this rollout resides
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

Once the application is deployed you can visit your browser at `$GATEWAY_IP` or test from the command
line. Every request should return the red version:

```shell
curl $GATEWAY_IP/color
```

## Step 6 - Test the canary

Change the Rollout to use a different color for the `argoproj/rollouts-demo` image:

```shell
kubectl patch rollout rollouts-demo -n default \
  --type='json' -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/image", "value":"argoproj/rollouts-demo:blue"}]'
```

The Gateway plugin will split the traffic to your canary by instructing HAProxy via the Gateway API.
Run the command below and depending on the canary status you will sometimes see the red or blue
version returned:

```shell
while true; do curl $GATEWAY_IP; done
```

You can monitor the canary progress from the command line with:

```shell
watch kubectl argo rollouts get rollout rollouts-demo
```

You can also inspect the weights the plugin writes into the HTTPRoute:

```shell
kubectl get httproute argo-rollouts-http-route -o jsonpath='{range .spec.rules[0].backendRefs[*]}{.name}={.weight}{"\n"}{end}'
```

At the first pause (`setWeight: 30`) this prints:

```
argo-rollouts-stable-service=70
argo-rollouts-canary-service=30
```

And sampling the traffic confirms HAProxy honours the split:

```shell
for i in $(seq 1 200); do curl -s $GATEWAY_IP/color; echo; done | sort | uniq -c
```

```
     62 "blue"
    138 "red"
```

The exact counts vary a little between runs. HAProxy normalises the Gateway API weights across the
individual pod endpoints, so with a small number of replicas the split lands near the requested ratio
rather than exactly on it.

The example above has two indefinite `pause: {}` steps, so promote twice to finish the canary:

```shell
kubectl argo rollouts promote rollouts-demo
```

Once the rollout is `Healthy` again the plugin resets the canary weight back to `0` and removes the
`rollouts.argoproj.io/gatewayapi-canary` label from the HTTPRoute, and every request returns the blue
version.

## Limitations

HAProxy Ingress implements the core features of `Gateway`, `HTTPRoute`, `TLSRoute`, `TCPRoute` and
`ReferenceGrant`. It does **not** implement `GRPCRoute`, so the plugin's
[gRPC routing](../../docs/features/grpc.md) support cannot be used with this provider. `UDPRoute` is
also unsupported and will stay that way, since HAProxy does not route UDP.

Gateway API resources in HAProxy Ingress do not support annotations. Provider specific settings are
applied by annotating the target Services with
[backend or path scoped configuration keys](https://haproxy-ingress.github.io/docs/configuration/keys/#scope)
instead.

See the [HAProxy Ingress Gateway API documentation](https://haproxy-ingress.github.io/docs/configuration/gateway-api/)
for the full conformance details.
