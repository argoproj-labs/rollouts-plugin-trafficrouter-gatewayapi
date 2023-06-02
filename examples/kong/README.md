# Using Kong Gateway with Argo Rollouts

Kong Ingress has [native support](https://docs.konghq.com/kubernetes-ingress-controller/latest/concepts/gateway-api/) for the Gateway API making the integration with Argo Rollouts a straightforward process.


## Step 0 - Install the Gateway APIs

Kong does not install the Gateway APIs by default. You need to install them manually
as described in the [instructions](https://gateway-api.sigs.k8s.io/guides/#installing-gateway-api).

```shell
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v0.7.1/standard-install.yaml
```

It is imperative you install the APIs **before** installing Kong, as the Helm chart detects the APIs and also installs the correct roles for Kong itself to manage Gateway resources.

## Step 1 - Deploy Kong Ingress to the cluster

Follow [the official instructions](https://docs.konghq.com/kubernetes-ingress-controller/2.9.x/deployment/k4k8s/#helm)


```shell
helm repo add kong https://charts.konghq.com
helm repo update


# Helm 3
helm install kong/kong --generate-name --set ingressController.installCRDs=false -n kong --create-namespace

```

Then enable Gateway support by toggling [the respective feature](https://docs.konghq.com/kubernetes-ingress-controller/2.9.x/deployment/install-gateway-apis/):

```shell
kubectl set env -n kong deployment/ingress-kong CONTROLLER_FEATURE_GATES="GatewayAlpha=true" -c ingress-controller
kubectl rollout restart -n NAMESPACE deployment DEPLOYMENT_NAME
```

## Step 2 - Create Gateway and Gateway class

Now create the GatewayClass object (it needs to be created only once). 

```yaml
apiVersion: gateway.networking.k8s.io/v1beta1
kind: GatewayClass
metadata:
  name: kong
  annotations:
    konghq.com/gatewayclass-unmanaged: 'true'

spec:
  controllerName: konghq.com/kic-gateway-controller
```

Apply the file with `kubectl`

Create a gateway:

```yaml
apiVersion: gateway.networking.k8s.io/v1beta1
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

Get the IP of the gateway with:

```shell
kubectl get gateways.gateway.networking.k8s.io kong -o=jsonpath="{.status.addresses[0].value}"
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

Apply both files with `kubectl`.

## Step 4 - Create HTTPRoute that defines a traffic split between two services

Create HTTPRoute and connect to the created Gateway resource

```yaml
kind: HTTPRoute
apiVersion: gateway.networking.k8s.io/v1beta1
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

Apply all the above manifests with `kubectl`.

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

