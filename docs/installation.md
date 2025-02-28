# Kubernetes installation

The plugin needs the main Argo Rollouts controller to work. You need to have both installed in order to perform progressive delivery
scenarios using your Kubernetes API Gateway implementation.

## Installing the Argo Rollouts controller

First get the core Argo Rollouts controller in your cluster.

Follow the [official instructions](https://argo-rollouts.readthedocs.io/en/stable/installation/) to install Argo Rollouts.

Optionally install the [Argo Rollouts CLI](https://argoproj.github.io/argo-rollouts/features/kubectl-plugin/) in order to control Rollouts from your terminal.

## Installing the plugin via HTTP(S)

To install the plugin create a configmap with the following syntax:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argo-rollouts-config # must be named like this
  namespace: argo-rollouts # must be in this namespace
data:
  trafficRouterPlugins: |-
    - name: "argoproj-labs/gatewayAPI"
      location: "https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/releases/download/<version>/gatewayapi-plugin-<arch>"
```

You can find the available versions and architectures at the [Releases page](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/releases).

For example for a Linux/x86 cluster save the following in a file of your choosing e.g. `gateway-plugin.yml`.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argo-rollouts-config # must be named like this
  namespace: argo-rollouts # must be in this namespace
data:
  trafficRouterPlugins: |-
    - name: "argoproj-labs/gatewayAPI"
      location: "https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/releases/download/v0.4.0/gatewayapi-plugin-linux-amd64"
```

Deploy this file with `kubectl apply -f gateway-plugin.yml -n argo-rollouts`. You can also use [Argo CD](https://argoproj.github.io/cd/) or any other Kubernetes deployment method that you prefer.

## Installing the plugin via init containers

Use the [Argo Rollouts Helm chart](https://argoproj.github.io/argo-helm/) and change the [default values](https://artifacthub.io/packages/helm/argo/argo-rollouts):

```yaml
controller:
    initContainers:                                   
      - name: copy-gwapi-plugin
        image: ghcr.io/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi:v0.5.0
        command: ["/bin/sh", "-c"]                    
        args:
          - cp /bin/rollouts-plugin-trafficrouter-gatewayapi /plugins
        volumeMounts:                                 
          - name: gwapi-plugin
            mountPath: /plugins
    trafficRouterPlugins:                             
      trafficRouterPlugins: |-
        - name: argoproj-labs/gatewayAPI
          location: "file:///plugins/rollouts-plugin-trafficrouter-gatewayapi"  
    volumes:                                           
      - name: gwapi-plugin
        emptyDir: {}
    volumeMounts:                                      
      - name: gwapi-plugin
        mountPath: /plugins
```        

We publish [container images](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/pkgs/container/rollouts-plugin-trafficrouter-gatewayapi) for both ARM and x86.

For more installation options see the [Plugin documentation](https://argoproj.github.io/argo-rollouts/features/traffic-management/plugins/) at the main Argo Rollouts site.

## Verifying the installation

Restart the Argo Rollouts controller so that it detects the presence of the plugin.

```
kubectl rollout restart deployment -n argo-rollouts argo-rollouts
```

Then check the controller logs. You should see a line for loading the plugin:

```
time="XXX" level=info msg="Downloading plugin argoproj-labs/gatewayAPI from: https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/releases/download/v0.4.0/gatewayapi-plugin-linux-amd64"
time="YYY" level=info msg="Download complete, it took 7.792426599s" 
```

You are now ready to use the Gateway API in your [Rollout definitions](https://argoproj.github.io/argo-rollouts/features/specification/). See also our [Quick Start Guide](quick-start.md).

## Configuration 

The `kubeClientQPS` and `kubeClientBurst` options configure the behavior of the Kubernetes client. These
values may need to be increased if you operate Argo Rollouts in a large cluster.  These values can be specified
using the `args` block of the plugin configuration:

```yaml
  trafficRouterPlugins:
    trafficRouterPlugins: |-
      - name: "argoproj-labs/gatewayAPI"
        location: "https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/releases/download/vX.X.X/gatewayapi-plugin-linux-amd64"
        args:
        - "-kubeClientQPS=40"
        - "-kubeClientBurst=80"
```

Notice that this setting applies **only** to the plugin process. The main Argo Rollouts controller is not affected (or any other additional plugins you might have already).