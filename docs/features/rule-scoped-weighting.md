# Rule-Scoped Weighting

By default, the plugin manages **every rule** on a route that contains both the configured
`canaryService` and `stableService` BackendRefs — it identifies "its" rule purely by those two
backend names. On a route with a single rule this is exactly what you want, but it means two
rules that both happen to reference `canaryService`/`stableService` will *both* get their
weights rewritten on every reconcile, even if only one of them is meant to carry live traffic.

`ruleName` lets you pin the plugin to a single rule, identified by the rule's own
[Gateway API `name`](https://gateway-api.sigs.k8s.io/reference/spec/#gateway.networking.k8s.io%2fv1.HTTPRouteRule) field, so it never touches any other rule — regardless of what backend
names that other rule contains.

## When you need this

A common case: you want a rule that's actually live and carries the canary/stable split
*plus* a third, unrelated static backend — for example a fixed 5% side-tap to a discovery
service alongside your canary/stable traffic. You also want a second rule, sharing the same
`stable`/`canary` names, that exists purely so the plugin has a clean pair of weights to write
to on every step. An external controller you run watches that second rule and recomputes the
live rule's three weights (rescaled to fit the reserved percentage) whenever it changes.

Without `ruleName`, the plugin can't tell these two rules apart — it finds `stable`+`canary` in
both and overwrites both, clobbering your rescaled live weights on every reconcile. With
`ruleName` set to the second rule's name, the plugin only ever touches that one.

```yaml
---
kind: HTTPRoute
apiVersion: gateway.networking.k8s.io/v1
metadata:
  name: test
  namespace: default
spec:
  parentRefs:
    - name: default-gateway
  hostnames:
    - test.example.com
  rules:
    # The live rule: actually receives traffic. The plugin never touches this rule directly;
    # an external controller keeps it in sync with the "argo" rule below.
    - matches:
        - path:
            type: PathPrefix
            value: /
      backendRefs:
        - name: discovery
          kind: Service
          port: 80
          weight: 5
        - name: stable
          kind: Service
          port: 80
          weight: 95
        - name: canary
          kind: Service
          port: 80
          weight: 0
    # The "argo" rule: unreachable (identical match to the rule above, so it never wins
    # precedence), but this is the only rule the plugin is configured to manage.
    - name: argo
      matches:
        - path:
            type: PathPrefix
            value: /
      backendRefs:
        - name: stable
          kind: Service
          port: 80
          weight: 100
        - name: canary
          kind: Service
          port: 80
          weight: 0
```

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: rollouts-demo
  namespace: default
spec:
  strategy:
    canary:
      canaryService: canary
      stableService: stable
      trafficRouting:
        plugins:
          argoproj-labs/gatewayAPI:
            httpRoute: test
            httpRouteRuleName: argo
            namespace: default
      steps:
        - setWeight: 10
        - pause: {}
        - setWeight: 50
        - pause: {}
        - setWeight: 100
        - pause: {}
  # ... rest of rollout spec
```

The plugin only ever reads and writes the `argo` rule. Reconciling the live rule's weights from
`argo`'s weights (rescaled to account for the reserved 5%) is your controller's job — this
option only guarantees the plugin won't touch the wrong rule; it doesn't perform that
normalization itself.

!!! tip
    If all you need is a fixed reserved percentage for a side backend — and you don't need a
    second, externally-reconciled rule at all — consider
    [`trafficRouting.maxTrafficWeight`](https://argo-rollouts.readthedocs.io/en/stable/features/traffic-management/)
    on the Rollout instead. Setting it to `100 - <reserved>` makes the canary/stable weights the
    plugin writes always sum to exactly that reserved amount, with no second rule or external
    controller required.

## Configuration reference

`ruleName` is available on every route kind, both in the list form and as a sibling to the
singular form:

| Config field | Applies to |
|---|---|
| `httpRoute` + `httpRouteRuleName` | Singular HTTPRoute |
| `httpRoutes[].ruleName` | Each entry under the `httpRoutes` list |
| `grpcRoute` + `grpcRouteRuleName` | Singular GRPCRoute |
| `grpcRoutes[].ruleName` | Each entry under the `grpcRoutes` list |
| `tcpRoute` + `tcpRouteRuleName` | Singular TCPRoute |
| `tcpRoutes[].ruleName` | Each entry under the `tcpRoutes` list |
| `tlsRoute` + `tlsRouteRuleName` | Singular TLSRoute |
| `tlsRoutes[].ruleName` | Each entry under the `tlsRoutes` list |

If `ruleName` is set but no rule with that name contains both the canary and stable BackendRefs
(HTTPRoute/GRPCRoute), or exists at all (TCPRoute/TLSRoute), `SetWeight`/`SetHeaderRoute` return
an error rather than silently matching nothing.

If `ruleName` is left unset, behavior is unchanged from previous versions of the plugin: every
rule containing both backend names is managed.
