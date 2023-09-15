# Using Google Cloud with Argo Rollouts

Google cloud has [native support](https://cloud.google.com/kubernetes-engine/docs/concepts/gateway-api) for the Gateway API making the integration with Argo Rollouts a straightforward process.

## Step 1 - Create a cluster with Gateway support in Google Cloud

Follow [the official instructions](https://cloud.google.com/kubernetes-engine/docs/how-to/deploying-gateways#internal-gateway)

The example below is for an internal gateway as it is simple but the integration should work for all Google cloud gateways.


You can create a new cluster with gateway support with:

```shell
  gcloud container clusters create CLUSTER_NAME \
	--gateway-api=standard \
	--cluster-version=VERSION \
	--region=COMPUTE_REGION
```

or update an existing one with:

```shell
gcloud container clusters update CLUSTER_NAME \
--gateway-api=standard \
--region=COMPUTE_REGION
```

## Step 2 - Create Google Load balancer with Gateway support

Then create a proxy subnet as shown in the [instructions](https://cloud.google.com/kubernetes-engine/docs/how-to/deploying-gateways#configure_a_proxy-only_subnet) 

```shell
gcloud compute networks subnets create demo-subnet \
	--purpose=REGIONAL_MANAGED_PROXY \
	--role=ACTIVE \
	--region=us-central1 \
	--network=default \
	--range=10.1.1.0/24
```

Create a gateway and apply it to the cluster with

```yaml
kind: Gateway
apiVersion: gateway.networking.k8s.io/v1beta1
metadata:
  name: internal-http
spec:
  gatewayClassName: gke-l7-rilb
  listeners:
  - name: http
    protocol: HTTP
    port: 80
```

Get the IP of the gateway with

```shell
kubectl get gateways.gateway.networking.k8s.io internal-http -o=jsonpath="{.status.addresses[0].value}"
```

Note down the IP address for testing the application later.

## Step 3 - Give access to Argo Rollouts for the Gateway/Http Route


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
    name: internal-http
  hostnames:
  - "demo.example.com"
  rules:
    - backendRefs:
        - name: argo-rollouts-stable-service
          port: 80
        - name: argo-rollouts-canary-service
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

Apply all the above manifests

## Step 5 - Create an example Rollout

Deploy a rollout to get the initial version

```yaml
Here is an example rollout

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


