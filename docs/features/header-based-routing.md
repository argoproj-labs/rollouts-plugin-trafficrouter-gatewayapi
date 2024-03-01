# Header Based routing

When you want to isolate the behavior of clients that connect to a canary, you can simply use a different http route as explained in [Advanced Deployments](advanced-deployments.md).

An alternative method is to use HTTP headers that distinguish which clients connect to the canary and which do not.

## Using a custom header with a single route

Here is an example of a rollout that uses headers 
for the canary:

```yml
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
        - name: canary-route1
        - name: canary-route2
        plugins:
          argoproj-labs/gatewayAPI:
            httpRoutes:
              - name: argo-rollouts-http-route
                useHeaderRoutes: true
            namespace: default
      steps:
      - setWeight: 10
      - setHeaderRoute:
          name: canary-route1
          match:
            - headerName: X-Canary-start
              headerValue:
                exact: ten-per-cent      
      - pause: {}      
      - setWeight: 50
      - setHeaderRoute:
          name: canary-route2
          match:
            - headerName: X-Canary-middle
              headerValue:
                exact: half-traffic      
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
          image: <your-image:your-tag>
          ports:
            - name: http
              containerPort: 8080
              protocol: TCP
```              

Notice that the route names used for headers in `setHeaderRoute` must also be defined in the `managedRoutes` block as well.

Now when the canary reaches 10% an extra route will be created that uses the `X-Canary-start` header with value `ten-per-cent`

When the canary reaches 50% a different header route will be created. At the end of the canary all header routes are discarded.

## Using multiple routes with headers

It is also possible to combine [multiple routes](multiple-routes.md) with custom headers.

Here is an example

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: rollouts-demo
spec:
  replicas: 4
  revisionHistoryLimit: 2
  selector:
    matchLabels:
      app: rollouts-demo
  strategy:
    canary:
      stableService: stable-service
      canaryService: canary-service
      trafficRouting:
        managedRoutes:
          - name: header-route
          - name: header-route2
        plugins:
          argoproj-labs/gatewayAPI:
            namespace: default                                                       httpRoutes:
              - name: http-route
                useHeaderRoutes: true
              - name: http-route2
                useHeaderRoutes: true
              - name: http-route3
      steps:
      - setWeight: 20
      - pause: {duration: 10}
      - setWeight: 40
      - pause: {duration: 20}
      - setWeight: 60
      - setHeaderRoute:
          name: header-route
          match:
            - headerName: X-Test
              headerValue:
                exact: test
      - pause: {duration: 10}
      - setHeaderRoute:
          name: header-route2
          match:
            - headerName: X-Test2
              headerValue:
                exact: test
      - pause: {}
      - setWeight: 80
      - setHeaderRoute: # remove header route
          name: header-route
      - pause: {}
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

With the `useHeaderRoutes` variable you can decide which routes
will honor the custom headers.