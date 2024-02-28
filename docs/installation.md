# Kubernetes installation

The plugin needs the main Argo Rollouts controller to work. You need to have both installed in order to perform progressive delivery
scenarios using your Kubernetes API Gateway implementation.

## Installing the Argo Rollouts controller

First get the core Argo Rollouts controller in your cluster.

Follow the [official instructions](https://argo-rollouts.readthedocs.io/en/stable/installation/) to install Argo Rollouts.

Optionally instal the [Argo Rollouts CLI](https://argoproj.github.io/argo-rollouts/features/kubectl-plugin/) in order to control Rollouts from your terminal.

## Installing the plugin

To install the plugin create a configmap with the following syntax:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argo-rollouts-config # must be so name
  namespace: argo-rollouts # must be in this namespace
data:
  trafficRouterPlugins: |-
    - name: "argoproj-labs/gatewayAPI"
      location: "https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/releases/download/<version>/gateway-api-plugin-<arch>"
```

You can find the available versions and architectures at the [Releases page](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/releases).

For example for a Linux/x86 cluster save the following in a file of your choosing e.g. `gateway-plugin.yml`.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argo-rollouts-config # must be so name
  namespace: argo-rollouts # must be in this namespace
data:
  trafficRouterPlugins: |-
    - name: "argoproj-labs/gatewayAPI"
      location: "https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/releases/download/v0.2.0/gateway-api-plugin-linux-amd64"
```

Deploy this file with `kubectl -f gateway-plugin.yml -n argo-rollouts`. You can also use [Argo CD](https://argoproj.github.io/cd/) or any other Kubernetes deployment method that you prefer.

## Verifying the installation

Restart the Argo Rollouts controller so that it detects the presence of the plugin.

```
kubectl rollout restart deployment -n argo-rollouts argo-rollouts
```

Then check the controller logs. You should see a line for loading the plugin:

```
time="XXX" level=info msg="Downloading plugin argoproj-labs/gatewayAPI from: https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/releases/download/v0.2.0/gateway-api-plugin-linux-amd64"
time="YYY" level=info msg="Download complete, it took 7.792426599s" 
```

You are now ready to use the Gateway API in your [Rollout definitions](https://argoproj.github.io/argo-rollouts/features/specification/). 
