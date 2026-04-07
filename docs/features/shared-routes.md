# Shared Routes

This page documents how the Gateway API plugin handles scenarios where a single route is controlled by multiple rollouts simultaneously.

## Use Case

In microservices architectures, it's common for multiple teams to deploy independent services that share a gateway route. For example:

- Team A deploys `service-a` through a shared API gateway route
- Team B deploys `service-b` through the same route
- Both teams use Argo Rollouts for canary deployments independently

When both teams run canary deployments at the same time, the plugin needs to coordinate its behavior to avoid conflicts.

### Example: Shared HTTPRoute with Multiple Services

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: api-gateway
  namespace: default
spec:
  parentRefs:
    - name: main-gateway
  hostnames:
    - api.example.com
  rules:
    # Service A (managed by Team A's rollout)
    - matches:
        - path:
            type: PathPrefix
            value: /service-a
      backendRefs:
        - name: service-a-stable
          port: 80
          weight: 100
        - name: service-a-canary
          port: 80
          weight: 0
    # Service B (managed by Team B's rollout)
    - matches:
        - path:
            type: PathPrefix
            value: /service-b
      backendRefs:
        - name: service-b-stable
          port: 80
          weight: 100
        - name: service-b-canary
          port: 80
          weight: 0
```

Each team configures their Rollout to reference the same HTTPRoute:

```yaml
# Team A's Rollout
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: service-a
  namespace: default
spec:
  strategy:
    canary:
      stableService: service-a-stable
      canaryService: service-a-canary
      trafficRouting:
        plugins:
          argoproj-labs/gatewayAPI:
            httpRoute: api-gateway
            namespace: default
      steps:
        - setWeight: 20
        - pause: {duration: 5m}
        - setWeight: 50
        - pause: {duration: 5m}
  # ... rest of rollout spec
---
# Team B's Rollout
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: service-b
  namespace: default
spec:
  strategy:
    canary:
      stableService: service-b-stable
      canaryService: service-b-canary
      trafficRouting:
        plugins:
          argoproj-labs/gatewayAPI:
            httpRoute: api-gateway
            namespace: default
      steps:
        - setWeight: 10
        - pause: {duration: 10m}
        - setWeight: 100
  # ... rest of rollout spec
```

## Reference Counting

The plugin uses **reference counting** to track which rollouts are actively using a route. This ensures:

1. The in-progress label is added when the **first** rollout starts
2. The in-progress label is only removed when the **last** rollout finishes
3. GitOps tools don't detect drift while any rollout is still active

### How It Works

The plugin tracks active rollouts using an annotation on the route:

| Annotation | Default Value |
|------------|---------------|
| `rollouts.argoproj.io/gatewayapi-active-rollouts` | Comma-separated list of `namespace/rollout-name` |

**Example behavior with two rollouts sharing a route:**

| Event | Label | Annotation |
|-------|-------|------------|
| Rollout A starts (weight=30) | `in-progress` | `default/rollout-a` |
| Rollout B starts (weight=50) | `in-progress` | `default/rollout-a,default/rollout-b` |
| Rollout A finishes (weight=0) | `in-progress` | `default/rollout-b` |
| Rollout B finishes (weight=0) | *removed* | *removed* |

### Configuration

You can customize the annotation key used for tracking active rollouts:

```yaml
trafficRouting:
  plugins:
    argoproj-labs/gatewayAPI:
      httpRoute: my-route
      namespace: default
      # Customize the annotation key used for tracking active rollouts
      activeRolloutsAnnotationKey: rollouts.argoproj.io/gatewayapi-active-rollouts
```

## Working with GitOps Controllers

When multiple rollouts share a route, GitOps tools may detect drift on the annotation that tracks active rollouts. To prevent this, configure your GitOps tool to ignore differences in both the in-progress label and the active-rollouts annotation.

### Argo CD `ignoreDifferences`

When using Argo CD, add the following configuration to ignore both the temporary weight changes and the active rollouts tracking annotation:

```yaml
configs:
  cm:
    resource.customizations.ignoreDifferences.gateway.networking.k8s.io_HTTPRoute: |
      jqPathExpressions:
        - select(.metadata.labels["rollouts.argoproj.io/gatewayapi-canary"] == "in-progress") | .spec.rules
        - select(.metadata.annotations["rollouts.argoproj.io/gatewayapi-active-rollouts"] != null) | .metadata.annotations["rollouts.argoproj.io/gatewayapi-active-rollouts"]
```

This configuration:
1. Ignores `.spec.rules` changes while the in-progress label is present (standard canary behavior)
2. Ignores changes to the active-rollouts annotation (specific to shared route scenarios)

Duplicate the block for `GRPCRoute`, `TCPRoute` and `TLSRoute` if you manage those kinds as well.

## Considerations

When using shared routes, keep in mind:

1. **Weight conflicts**: If two rollouts set different weights for the same service, the last update wins. Coordinate rollout schedules between teams if this is a concern.

2. **Annotation visibility**: The active rollouts annotation is visible to anyone with read access to the route. This can be useful for debugging which rollouts are currently active.

3. **Cleanup on failure**: If a rollout is deleted while in progress, the plugin may not clean up the annotation. Manual cleanup may be needed in edge cases.
