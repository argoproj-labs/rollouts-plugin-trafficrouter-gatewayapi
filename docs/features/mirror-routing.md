# Mirror Routing

Mirror routing lets you duplicate live traffic to the canary pods without affecting the response returned to users. The canary pods receive a copy of every matched request and process it normally, but their responses are discarded. This is useful for validating canary behaviour under real-world traffic before exposing it to users.

## How mirror routing works

During a standard canary rollout a percentage of requests is routed to the canary pods. Users whose requests land on the canary pods see the new version.

Mirror routing is different: **all matched requests continue to be served by the stable pods**, while an identical copy of each request is also sent to the canary pods in the background.

![Mirror routing overview](../images/mirror-routing/mirror-routing.png)

This gives you several advantages:

- Canary pods receive production traffic patterns and load without any user-visible impact
- You can observe canary metrics, logs, and error rates against real requests
- If the canary has a bug, users are unaffected because only the stable response is returned

## Using mirror routing

!!! important
    Your [Gateway API provider](https://gateway-api.sigs.k8s.io/implementations/) must support the `RequestMirror` filter on HTTPRoute before you can use this feature. Mirror routing is only available for **HTTPRoute** — GRPCRoute, TCPRoute, and TLSRoute do not support mirroring in the Gateway API specification.

### Basic example

The `setMirrorRoute` step instructs the plugin to add a mirroring rule to the existing HTTPRoute. The rule name must also appear in `managedRoutes` so that Argo Rollouts knows to clean it up when the rollout finishes.

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: rollouts-demo
  namespace: default
spec:
  replicas: 5
  strategy:
    canary:
      canaryService: argo-rollouts-canary-service
      stableService: argo-rollouts-stable-service
      trafficRouting:
        managedRoutes:
          - name: mirror-route
        plugins:
          argoproj-labs/gatewayAPI:
            httpRoute: argo-rollouts-http-route
            namespace: default
      steps:
        - setWeight: 0
        - setMirrorRoute:
            name: mirror-route
            percentage: 100
            match:
              - method:
                  exact: GET
                path:
                  prefix: /
        - pause:
            duration: 10m
        - setMirrorRoute:
            name: mirror-route   # no match = remove the mirror rule
        - setWeight: 100
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
          image: <your-image:your-tag>
          ports:
            - name: http
              containerPort: 8080
              protocol: TCP
```

This rollout:

1. Starts with 0% of traffic going to the canary
2. Adds a mirror rule that duplicates every `GET /` request to the canary pods (100% mirroring)
3. Pauses for 10 minutes so you can inspect canary metrics
4. Removes the mirror rule by repeating `setMirrorRoute` with only `name` and no `match`
5. Promotes the canary to 100%

### Controlling the mirrored percentage

The `percentage` field controls what fraction of matched requests are mirrored. Omitting it mirrors 100% of matched traffic (Gateway API default).

```yaml
- setMirrorRoute:
    name: mirror-route
    percentage: 10       # mirror only 10% of matched requests
    match:
      - path:
          prefix: /api
```

### Match criteria

The `match` field narrows which requests are mirrored. All conditions within a single `match` entry use AND semantics; multiple entries in the list use OR semantics.

| Field | Supported match types | Example |
|---|---|---|
| `path` | `exact`, `prefix`, `regex` | `path: {prefix: /api}` |
| `method` | `exact` | `method: {exact: GET}` |
| `headers` | `exact`, `prefix` (converted to regex `prefix.*`), `regex` | `headers: {X-Version: {exact: v2}}` |

```yaml
- setMirrorRoute:
    name: mirror-route
    percentage: 50
    match:
      - method:
          exact: POST
        path:
          exact: /checkout
        headers:
          X-Region:
            exact: eu-west
```

### Removing a mirror rule

Pass only the `name` field (no `match`) to remove a previously created mirror rule:

```yaml
- setMirrorRoute:
    name: mirror-route   # match omitted — removes the rule
```

Mirror rules are also removed automatically when the rollout completes or is aborted, because Argo Rollouts calls `RemoveManagedRoutes` at the end of every rollout.

## Combining mirror routing with header routing

You can use both `setMirrorRoute` and `setHeaderRoute` steps in the same rollout. Declare both route names in `managedRoutes`:

```yaml
trafficRouting:
  managedRoutes:
    - name: mirror-route
    - name: canary-header-route
  plugins:
    argoproj-labs/gatewayAPI:
      httpRoutes:
        - name: argo-rollouts-http-route
          useHeaderRoutes: true
      namespace: default
steps:
  - setWeight: 0
  - setMirrorRoute:
      name: mirror-route
      percentage: 100
      match:
        - path:
            prefix: /
  - pause:
      duration: 5m
  - setMirrorRoute:
      name: mirror-route     # remove mirror before enabling header routing
  - setWeight: 10
  - setHeaderRoute:
      name: canary-header-route
      match:
        - headerName: X-Canary
          headerValue:
            exact: "true"
  - pause: {}
  - setWeight: 100
```

## What the plugin does under the hood

When `setMirrorRoute` is called the plugin:

1. Fetches the named HTTPRoute from the cluster
2. Copies the existing backend refs (stable and canary services with their current weights) from the base rule
3. Appends a new `HTTPRouteRule` containing those backend refs and a `RequestMirror` filter that points to the canary service
4. Updates the HTTPRoute via the Kubernetes API
5. Records the new rule's index in the plugin ConfigMap (default: `argo-gatewayapi-configmap`) under the key `httpMirrorManagedRoutes` so it can be removed cleanly later

The resulting HTTPRoute rule looks like this:

```yaml
rules:
  # existing rule — handles all traffic normally
  - backendRefs:
      - name: argo-rollouts-stable-service
        port: 80
        weight: 100
      - name: argo-rollouts-canary-service
        port: 80
        weight: 0
    matches:
      - path:
          type: PathPrefix
          value: /

  # mirror rule added by the plugin
  - backendRefs:
      - name: argo-rollouts-stable-service
        port: 80
        weight: 100
      - name: argo-rollouts-canary-service
        port: 80
        weight: 0
    matches:
      - method: GET
        path:
          type: PathPrefix
          value: /
    filters:
      - type: RequestMirror
        requestMirror:
          backendRef:
            name: argo-rollouts-canary-service
            port: 80
          percent: 100
```

If you use Argo CD, add an `ignoreDifferences` entry for the HTTPRoute so that Argo CD does not revert the dynamically added mirror rule. See the [Argo Rollouts Istio guide](https://argo-rollouts.readthedocs.io/en/stable/features/traffic-management/istio/#integrating-with-gitops) for an example of this pattern.
