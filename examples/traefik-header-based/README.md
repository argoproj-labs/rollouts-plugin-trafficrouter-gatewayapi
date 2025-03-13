# Using Traefik Gateway API with Argo Rollouts

This guide will describe how to use Traefik proxy an an implementation
for the Gateway API in order to do split traffic with Argo Rollouts.

Versions used

- Argo Rollouts [1.7.2](https://github.com/argoproj/argo-rollouts/releases)
- Argo Rollouts Gateway API plugin [0.4.0](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/releases)
- [Traefik 3.1.4](https://doc.traefik.io/traefik/getting-started/install-traefik/)
- GatewayAPI 1.1 (Part of the [Traefik Helm chart](https://github.com/traefik/traefik-helm-chart))

Note that Argo Rollouts also [supports Traefik natively](https://argoproj.github.io/argo-rollouts/features/traffic-management/traefik/).

## Step 1 - Enable Gateway Provider and create Gateway entrypoint

First let's install Traefik as a Gateway provider. Follow the official [installation instructions](https://doc.traefik.io/traefik/getting-started/install-traefik/).

You should also read the documentation on how [Traefik implements the Gateway API](https://doc.traefik.io/traefik/providers/kubernetes-gateway/).

Install Traefik with Gateway support

```
helm repo add traefik https://traefik.github.io/charts
helm repo update
helm install traefik traefik/traefik --set experimental.kubernetesGateway.enabled=true --set providers.kubernetesGateway.enabled=true --set ingressRoute.dashboard.enabled=true --version v32.0.0 --namespace=traefik --create-namespace
```

Note that using Helm automatically installs the Kubernetes Gateway API CRDs
as well as the appropriate RBAC resources so that Traefik can manage
HTTP routes inside your cluster.

Enabling the Traefik dashboard is optional, but helpful when debugging
routes.

After initial installation you can expose the dashboard with

```
kubectl port-forward -n traefik $(kubectl -n traefik get pods --selector "app.kubernetes.io/name=traefik" --output=name) 9000:9000
```

And then visit `http://127.0.0.1:9000/dashboard/` (or whatever is the IP
of your Loadbalancer)

Also notice that the Helm chart creates a Loadbalancer service by default.
If your cluster has already a Loadbalancer or you want to customize Traefik installation you need to pass your own [options](https://github.com/traefik/traefik-helm-chart/blob/master/traefik/values.yaml).

## Step 2 - Create GatewayClass and Gateway resources

After installing Traefik with need a GatewayClass and an actual Gateway.

The Helm chart already created a GatewayClass for you.

You can verify it with

```
kubectl get gatewayclass
```

Make sure that the value returned is "True" in the "Accepted" column.

Now let's create a Gateway

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: traefik-gateway
spec:
  gatewayClassName: traefik
  listeners:
    - protocol: HTTP
      name: web
      port: 8000 # Default endpoint for Helm chart
```

Apply the file it with

```
kubectl apply -f gateway.yml
```

Notice that we installed the gateway on the default namespace which is where our application will be deployed as well. If you want the gateway to honor routes from other namespaces you need to install Traefik with a different option for `namespacePolicy` in the Helm chart.

This concludes the setup that is specific to Traefik Proxy. The rest of the steps are generic to any implementation of the Gateway API.

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

**Note:** These permission are not very strict. You should lock them down according to your needs.

With the following role we allow Argo Rollouts to have write access to HTTPRoutes and Gateways.

```yaml title="cluster-role-binding.yml"
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
kubectl apply -f cluster-role.yml
kubectl apply -f cluster-role-binding.yml
```

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

Note that this route is accessible the route prefix `/` in your browser
simply by visiting your Loadbalancer IP address. You'll also notice that any rules present for header based routing are absent. The Gateway API plugin will add these rules as the rollout progresses, and keep track of them in a configmap on the namespace where the rollout resides.

Apply it with:

```shell
kubectl apply -f cluster-role.yaml
kubectl apply -f cluster-role-binding.yaml
```

## Step 5 - Create canary and stable services for your application

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

Apply both file with kubectl.

## Step 6 - Create an example Rollout

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
        managedRoutes:
          - name: rollouts-demo-canary-internal
          - name: rollouts-demo-canary-beta-customers
        plugins:
          argoproj-labs/gatewayAPI:
            httpRoutes:
              - name: argo-rollouts-http-route # our created httproute
                useHeaderRoutes: true
            namespace: default # namespace where this rollout resides
      steps:
        - setCanaryScale:
            weight: 1 # Scale pods equivalent to 1% of the total number of pods
        - setHeaderRoute:
            match:
              - headerName: X-Canary-Candidate
                headerValue:
                  exact: internal
            name: rollouts-demo-canary-internal
        - pause: {} # Run synthetics tests or manual validation from internal users
        - setHeaderRoute:
            match:
              - headerName: X-Canary-Candidate
                headerValue:
                  exact: beta-customers
            name: rollouts-demo-canary-beta-customers
        - pause: {} # Run analysis or manual validation from beta customers
        - setCanaryScale:
            weight: 30 # Prepare for real customer traffic
        - setWeight: 30
        - setCanaryScale:
            matchTrafficWeight: true # Allow pods to scale with setWeight steps
        - pause: { duration: 10 }
        - setWeight: 40
        - pause: { duration: 10 }
        - setWeight: 60
        - pause: { duration: 10 }
        - setWeight: 80
        - pause: { duration: 10 }
        - setWeight: 100
        - setHeaderRoute:
            name: rollouts-demo-canary-internal # Remove internal traffic route
        - setHeaderRoute:
            name: rollouts-demo-canary-beta-customers # Remove beta-customers traffic route
        - pause: {} # Final sanity check on 100% traffic
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

Apply the file with kubectl

You can check the Rollout status with

```
kubectl argo rollouts get rollout rollouts-demo
```

Once the application is deployed you can visit your browser at `localhost`
or whatever is the IP of your loadbalancer.

## Step 8 - Confirm traffic without headers is going to stable

Change the Rollout YAML and use a different color for `argoproj/rollouts-demo` image such as red or green.

Apply the `rollout.yml` file again and the Gateway plugin will split the traffic to your canary by instructing Traefik proxy via the Gateway API.

In the initial steps, we will only allow traffic to the canary based on a header value for `X-Canary-Candidate`. Notice how the colors in your browser do not change even though canary pods are spun up and running.

You can also monitor the canary with from the command line with:

```
watch kubectl argo rollouts get rollout rollouts-demo
```

## Step 9 - Test the header-based routing with curl

You can test the header-based routing by sending requests with and without the specified header:

### Without header (goes to stable)

```shell
curl http://localhost:80
```

### With header (goes to canary)

```shell
curl -H "X-Canary-Candidate: internal" http://localhost:80
```

Now resume the canary, so that it adds the second header based route.

```shell
kubectl argo rollouts promote rollouts-demo
```

You can now check the second beta-customers header value goes to the canary with:

```shell
curl -H "X-Canary-Candidate: beta-customers" http://localhost:80
```

For a better understanding of how this behavior is happening, check out the new state of the HTTPRoute:

```shell
kubectl describe httproute argo-rollouts-http-route
```

You should see that two header-based routes are now present as additional rules in the HTTPRoute definition.

```yaml
...
  Rules:
    Backend Refs:
      Group:
      Kind:       Service
      Name:       argo-rollouts-stable-service
      Port:       80
      Weight:     100
      Group:
      Kind:       Service
      Name:       argo-rollouts-canary-service
      Port:       80
      # Note how weight is still 0 for this default rule, meaning 100% of traffic goes to stable
      Weight:     0
    Matches:
      Path:
        Type:   PathPrefix
        Value:  /
    Backend Refs:
      Group:
      Kind:    Service
      Name:    argo-rollouts-canary-service
      Port:    80
      # Note how weight is set to 1 with no alternative backendRef - this means 100% of traffic goes to canary
      Weight:  1
    Matches:
      Headers:
        Name:   X-Canary-Candidate
        Type:   Exact
        Value:  internal
      Path:
        Type:   PathPrefix
        Value:  /
    Backend Refs:
      Group:
      Kind:    Service
      Name:    argo-rollouts-canary-service
      Port:    80
      # Note how weight is set to 1 with no alternative backendRef - this means 100% of traffic goes to canary
      Weight:  1
    Matches:
      Headers:
        Name:   X-Canary-Candidate
        Type:   Exact
        Value:  beta-customers
      Path:
        Type:   PathPrefix
        Value:  /
```

## Step 10 - Test the weighted routing alongside header-based routing with curl in the next steps

Resume the rollout again to reach the steps where 30%, 40%, 60%, 80%, and eventually 100% of traffic directs to the canary.

Visit the browser and notice how the colors change as the rollout progresses without the header present.

You should see that 100% of your requests with the headers above will still direct to the canary throughout this process.

## Step 11 - Confirm the header routes on the HTTPRoute are removed after 100% of traffic is directed

After the rollout has reached 100% traffic, it will pause indefinitely once again. Take a look at the HTTPRoute definition to see that it has returned to the state that matches what was initially applied, except that the canary service has 100% weight.

```shell
kubectl describe httproute argo-rollouts-http-route
```

## Step 12 - Allow the canary to complete and validate state of HTTPRoute

Finally, promote the rollout to complete the canary deployment.

```shell
kubectl argo rollouts promote rollouts-demo
```

You can once again check the state of the HTTPRoute to see that the header routes are still absent, and the stable service once again has 100% weight.

```shell
kubectl describe httproute argo-rollouts-http-route
```
