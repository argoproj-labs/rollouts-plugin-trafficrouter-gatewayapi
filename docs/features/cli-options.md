# CLI Options

## Increasing Kubernetes API Server Request QPS and Burst
The `kubeClientQPS` and `kubeClientBurst` options configure the behavior of the Kubernetes client. These
values may need to be increased if you operate Argo Rollouts in a large cluster.  These values can be specified
using the `args` block of the plugin configuration:

```yaml
  trafficRouterPlugins:
    trafficRouterPlugins: |-
      - name: "argoproj-labs/gatewayAPI"
        location: "https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/releases/download/v0.4.0/gatewayapi-plugin-linux-amd64"
        args:
        - "-kubeClientQPS=40"
        - "-kubeClientBurst=80"
```
