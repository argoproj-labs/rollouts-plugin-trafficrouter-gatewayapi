# TLS Routes

To use TLSRoute:

1. Install your traffic provider
2. Install [GatewayAPI CRD](https://gateway-api.sigs.k8s.io/guides/#installing-gateway-api) if your traffic provider doesn't do it by default
3. Install [Argo Rollouts](https://argoproj.github.io/argo-rollouts/installation/)
4. Install [Argo Rollouts GatewayAPI plugin](../installation.md)
5. Create stable and canary services
6. Create TLSRoute resource according to the GatewayAPI and your traffic provider documentation
```yaml
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: TLSRoute
metadata:
  name: first-tlsroute
  namespace: default
spec:
  parentRefs:
    - name: traefik-gateway # read documentation of your traffic provider to understand what you need to specify here
      sectionName: tls
      namespace: default
      kind: Gateway
  hostnames:
    - "example.com" # SNI hostname for TLS traffic routing
  rules:
    - backendRefs:
        - name: argo-rollouts-stable-service # stable service you have created on the 5th step
          port: 443
        - name: argo-rollouts-canary-service # canary service you have created on the 5th step
          port: 443
```
7. Create Rollout resource
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: rollouts-demo
  namespace: default
spec:
  replicas: 2
  strategy:
    canary:
      canaryService: argo-rollouts-canary-service
      stableService: argo-rollouts-stable-service
      trafficRouting:
        plugins:
          argoproj-labs/gatewayAPI:
            tlsRoute: first-tlsroute # tlsroute you have created on the 6th step
            namespace: default # namespace where your tlsroute is
      steps:
        - setWeight: 30
        - pause: { duration: 2 }
  revisionHistoryLimit: 1
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
            - name: https
              containerPort: 8080
              protocol: TCP
          resources:
            requests:
              memory: 32Mi
              cpu: 5m
```

## Traffic Provider Support

TLSRoute is part of the Gateway API experimental channel. Ensure your traffic provider supports TLSRoute before using it in production. Check the [Gateway API implementations list](https://gateway-api.sigs.k8s.io/implementations/) for TLSRoute support.
