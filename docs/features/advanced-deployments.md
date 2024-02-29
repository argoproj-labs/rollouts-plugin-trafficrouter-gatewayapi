# Advanced Deployment methods

Once you have the basic canary deployment in place, you can explore
several other deployment scenarios with more flexible routing options

## Pinning clients to a specific version

Sometimes you have some special clients (either humans or other services) which you consider critical and want to stay in the stable version as long as possible even if a canary is in progress.

On the other end of the spectrum you might have some users that want to see the new versions as soon as possible (e.g. internal company users).

There are many ways to achieve this, but one of the most simple scenarios is to use additional HTTP routes that point only to a specific service.

![Version pinning](../images/advanced-deployments/pinning-versions.png)

In the example above, VIP users connect to `old.example.com` and always see the previous version. Bleeding edge users connect to `new.example.com` and see the new version as soon as the canary starts (for 100% of their traffic). Everybody else connects to `app.example.com` and sees the canary
according to the current percentage of the rollout.


Here is definition for the 3 HTTP routes.

```yaml
---
kind: HTTPRoute
apiVersion: gateway.networking.k8s.io/v1beta1
metadata:
  name: canary-route
  namespace: default
spec:
  parentRefs:
    - name: eg
  hostnames:
    - app.example.com  
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
---
kind: HTTPRoute
apiVersion: gateway.networking.k8s.io/v1beta1
metadata:
  name: always-old-version
  namespace: default
spec:
  parentRefs:
    - name: eg
  hostnames:
    - old.example.com     
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /
    backendRefs:
    - name: argo-rollouts-stable-service
      kind: Service
      port: 80
---
kind: HTTPRoute
apiVersion: gateway.networking.k8s.io/v1beta1
metadata:
  name: always-new-version
  namespace: default
spec:
  parentRefs:
    - name: eg
  hostnames:
    - new.example.com     
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /  
    backendRefs:
    - name: argo-rollouts-canary-service
      kind: Service
      port: 80            
```      

This defines the following routes

1. `canary-route` at host `app.example.com` sees canary as current percentage (2 backing services)
1. `always-old-version` at host `old.example.com` sees always old/stable version (1 backing service)
1. `always-new-version` at new `app.example.com` sees always new/unstable version (1 backing service)

When a canary is not in progress then all clients see the same/active version without any further changes.

## Making applications "canary-aware"