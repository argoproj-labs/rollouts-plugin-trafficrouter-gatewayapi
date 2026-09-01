# Ping-Pong

!!! note "Argo Rollouts version"
    Ping-pong support for plugin-based traffic routers requires **Argo Rollouts v1.10** or later.

## Overview

The ping-pong feature enables zero-downtime rollouts for long-lived TCP and gRPC connections. It is the recommended approach for workloads where dropped connections during promotion are not acceptable — for example, services behind an AWS Network Load Balancer (NLB) with weighted target groups.

### How it differs from the default canary strategy

In a normal canary rollout, `canaryService` and `stableService` use selector swaps at promotion time: the stable service starts pointing at the new ReplicaSet. For AWS NLB (and similar load balancers that track connections at the target-group level), this swap triggers target deregistration and re-registration, which drops all in-flight connections.

Ping-pong avoids the swap entirely. Two persistent services — `pingService` and `pongService` — are defined upfront. Their selectors never change. Instead, the rollout controller alternates which one is "stable" via `status.canary.stablePingPong`, and the plugin shifts traffic weight between them. No selector change occurs at promotion, so no connections are dropped.

## Configuration

Instead of `canaryService` / `stableService`, define a `pingPong` block:

```yaml
spec:
  strategy:
    canary:
      pingPong:
        pingService: ping-service
        pongService: pong-service
```

`pingService` and `pongService` must exist as Kubernetes Services before the rollout starts. Both services should initially select the same pods as the stable deployment.

The TCPRoute (or HTTPRoute / GRPCRoute / TLSRoute) must reference both services as `backendRefs`.

## Example: TCPRoute with ping-pong

This is the primary use case — zero-downtime rollouts for long-lived TCP or gRPC connections.

### 1. Create ping and pong services

```yaml
apiVersion: v1
kind: Service
metadata:
  name: ping-service
  namespace: default
spec:
  selector:
    app: rollouts-demo
  ports:
    - port: 80
      targetPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: pong-service
  namespace: default
spec:
  selector:
    app: rollouts-demo
  ports:
    - port: 80
      targetPort: 8080
```

### 2. Create TCPRoute with both services as backendRefs

```yaml
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: TCPRoute
metadata:
  name: rollouts-demo-tcproute
  namespace: default
spec:
  parentRefs:
    - name: my-gateway
      sectionName: tcp
      namespace: default
  rules:
    - backendRefs:
        - name: ping-service
          port: 80
        - name: pong-service
          port: 80
```

### 3. Create the Rollout

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
      pingPong:
        pingService: ping-service
        pongService: pong-service
      trafficRouting:
        plugins:
          argoproj-labs/gatewayAPI:
            tcpRoute: rollouts-demo-tcproute
            namespace: default
      steps:
        - setWeight: 20
        - pause: { duration: 10 }
        - setWeight: 50
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
          image: argoproj/rollouts-demo:blue
          ports:
            - name: tcp
              containerPort: 8080
              protocol: TCP
```

## Example: HTTPRoute with ping-pong

The same approach works for HTTPRoute when you want to avoid selector swaps for HTTP workloads.

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: rollouts-demo-httproute
  namespace: default
spec:
  parentRefs:
    - name: my-gateway
  rules:
    - backendRefs:
        - name: ping-service
          port: 80
        - name: pong-service
          port: 80
---
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: rollouts-demo
  namespace: default
spec:
  replicas: 2
  strategy:
    canary:
      pingPong:
        pingService: ping-service
        pongService: pong-service
      trafficRouting:
        plugins:
          argoproj-labs/gatewayAPI:
            httpRoute: rollouts-demo-httproute
            namespace: default
      steps:
        - setWeight: 20
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
          image: argoproj/rollouts-demo:blue
          ports:
            - containerPort: 8080
```

## How it works

At each rollout step:

1. The controller reads `status.canary.stablePingPong` to determine which service is currently stable (`ping` or `pong`).
2. The plugin calls `GetStableAndCanaryServices` from the Argo Rollouts core library, which returns the correct `(stable, canary)` pair based on that status field.
3. The plugin updates the `backendRef` weights in the route accordingly.

At promotion, the controller flips `status.canary.stablePingPong` to point at the new stable service. No Kubernetes Service selector is modified, so no connections are dropped.