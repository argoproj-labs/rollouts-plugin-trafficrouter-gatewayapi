# Using Istio with Argo Rollouts Gateway API Plugin

This guide describes how to use Istio as a Gateway API implementation
to perform traffic splitting with Argo Rollouts.

Versions used:

* Argo Rollouts [1.8.4](https://github.com/argoproj/argo-rollouts/releases)
* Argo Rollouts Gateway API plugin [0.11.0](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/releases)
* [Istio 1.29.1](https://istio.io/latest/news/releases/)
* Gateway API 1.4.0 (installed separately, see below)

## Step 1 - Install Gateway API CRDs

Istio requires the standard Gateway API CRDs to be installed before Istio itself:

```
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.4.0/standard-install.yaml --server-side=true --force-conflicts
```

## Step 2 - Install Istio

Download the Istio CLI:

```
curl -L https://istio.io/downloadIstio | sh -
cd istio-1.29.1
export PATH=$PWD/bin:$PATH
```

Install Istio with the minimal profile. This installs only `istiod` (the control plane)
without any sidecar injection or ingress gateway — the Kubernetes Gateway API is used
instead:

```
istioctl install --set profile=minimal -y
```

Verify Istio is running:

```
kubectl get pods -n istio-system
```

You should see `istiod` in `Running` state.

After installation, Istio automatically creates the `istio` GatewayClass:

```
kubectl get gatewayclass
```

The `istio` GatewayClass should show `Accepted: True`.

## Step 3 - Give Argo Rollouts access to the Gateway API resources

Create a ClusterRole with the necessary permissions:

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

**Note:** These permissions are broad. Lock them down according to your needs.

Bind the role to the Argo Rollouts service account:

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

Apply both files:

```
kubectl apply -f cluster-role.yml
kubectl apply -f cluster-role-binding.yml
```

## Step 4 - Create a Gateway

When you create a Gateway resource with the `istio` GatewayClass, Istio automatically
provisions a Deployment and a LoadBalancer Service to handle the traffic.

```yaml title="gateway.yml"
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: istio-gateway
spec:
  gatewayClassName: istio
  listeners:
    - protocol: HTTP
      name: http
      port: 8080
```

Apply it:

```
kubectl apply -f gateway.yml
```

Wait for the Gateway to be programmed:

```
kubectl wait gateway/istio-gateway --for=condition=Programmed --timeout=120s
```

In Docker Desktop or cloud environments, Istio creates a LoadBalancer Service
(`istio-gateway-istio`) that receives an external IP automatically. Once the external
IP is assigned, the Gateway status becomes `Programmed: True`.

Verify:

```
kubectl get gateway istio-gateway
```

You should see an `ADDRESS` and `PROGRAMMED: True`.

This concludes the Istio-specific setup. The remaining steps are the same for any
Gateway API implementation.

## Step 5 - Create canary and stable services

- Stable service:

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

- Canary service:

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

Apply both:

```
kubectl apply -f stable.yml
kubectl apply -f canary.yml
```

## Step 6 - Create an HTTPRoute

```yaml title="httproute.yml"
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: argo-rollouts-http-route
spec:
  parentRefs:
    - name: istio-gateway
      namespace: default
  rules:
    - backendRefs:
        - name: argo-rollouts-stable-service
          port: 80
        - name: argo-rollouts-canary-service
          port: 80
```

Apply it:

```
kubectl apply -f httproute.yml
```

## Step 7 - Create an example Rollout

```yaml title="rollout.yml"
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: rollouts-demo
spec:
  replicas: 5
  strategy:
    canary:
      canaryService: argo-rollouts-canary-service
      stableService: argo-rollouts-stable-service
      trafficRouting:
        plugins:
          argoproj-labs/gatewayAPI:
            httpRoute: argo-rollouts-http-route
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

Apply it:

```
kubectl apply -f rollout.yml
```

Check the rollout status:

```
kubectl argo rollouts get rollout rollouts-demo
```

Once the application is deployed you can visit your browser at the external IP of the
`istio-gateway-istio` service on port 8080:

```
kubectl get svc istio-gateway-istio
```

## Step 8 - Test the canary

Update the image in `rollout.yml` to a different color (e.g. `argoproj/rollouts-demo:blue`)
and apply it again. The plugin will instruct Istio to split the traffic via the Gateway API.

You can monitor the canary from the command line with:

```
watch kubectl argo rollouts get rollout rollouts-demo
```

At the first pause step (30% canary), verify the HTTPRoute weights:

```
kubectl get httproute argo-rollouts-http-route -o yaml
```

You should see `weight: 70` on the stable service and `weight: 30` on the canary service.

To promote the canary to the next step:

```
kubectl argo rollouts promote rollouts-demo
```

Finished!
