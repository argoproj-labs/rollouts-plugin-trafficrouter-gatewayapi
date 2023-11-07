# Using Gloo Gateway with Argo Rollouts

[Gloo Gateway](https://docs.solo.io/gloo-gateway/v2/) is a powerful Kubernetes-native ingress controller and API gateway that is based on the [Kubernetes Gateway API](https://gateway-api.sigs.k8s.io/). It excels in function-level routing, supports legacy apps, microservices and serverless, offers robust discovery capabilities, integrates seamlessly with open-source projects, and is designed to support hybrid applications with various technologies, architectures, protocols, and clouds. 

## Prerequisites

* Kubernetes cluster with minimum version 1.23

## Step 1: Install the Kubernetes Gateway API and Gloo Gateway

1. Install the Kubernetes Gateway API CRDs. 
   ```shell
   kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.0.0/standard-install.yaml
   ```
   
2. Install Gloo Gateway. 
   ```shell
   helm install default -n gloo-system --create-namespace \
       oci://ghcr.io/solo-io/helm-charts/gloo-gateway \
       --version 2.0.0-beta1 \
       --wait --timeout 1m
   ```

3. Verify that the Gloo Gateway control plane is up and running.
   ```shell
   kubectl get pods -n gloo-system
   ```

4. Verify that the default `GatewayClass` resource is created.
   ```shell
   kubectl wait --timeout=1m -n gloo-system gatewayclass/gloo-gateway --for=condition=Accepted
   ```

   During the Helm installation, a `GatewayClass` resource is automatically created for you with the following configuration
   ```yaml
   apiVersion: gateway.networking.k8s.io/v1beta1
   kind: GatewayClass
   metadata:
     name: gloo-gateway
   spec:
     controllerName: solo.io/gloo-gateway
   ```

   You can use this `GatewayClass` to define `Gateway` resources that dynamically provision and configure Envoy proxies to handle incoming traffic.


## Step 2: Set up Argo Rollouts

1. Install Argo Rollouts. 
   ```shell
   kubectl create namespace argo-rollouts
   kubectl apply -n argo-rollouts -f https://github.com/argoproj/argo-rollouts/releases/latest/download/install.yaml
   ```
   
   See the [installation docs](https://argo-rollouts.readthedocs.io/en/stable/installation) for more detail.

2. Change the Argo Rollouts config map to install the Argo Rollout Gateway API Plugin. For more information, see the [project README](/README.md#installing-the-plugin).
   ```yaml
   cat <<EOF | kubectl apply -f -
   apiVersion: v1
   kind: ConfigMap
   metadata:
     name: argo-rollouts-config # must be so name
     namespace: argo-rollouts # must be in this namespace
   data:
     trafficRouterPlugins: |-
       - name: "argoproj-labs/gatewayAPI"
         location: "https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/releases/download/v0.0.0-rc1/gateway-api-plugin-linux-amd64"
   EOF
   ```

3. Restart the Argo Rollouts pod for the plug-in to take effect. 
   ```shell
   kubectl rollout restart deployment -n argo-rollouts argo-rollouts
   ```

4. Create a cluster role to allow the Argo Rollouts pod to manage HTTPRoute resources. 
   __Note:__ This `ClusterRole` is overly permissive and is provided __only for demo purposes__.
   
   ```yaml
   kubectl apply -f- <<EOF
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: gateway-controller-role
     namespace: argo-rollouts
   rules:
     - apiGroups:
         - "*"
       resources:
         - "*"
       verbs:
         - "*"
   EOF
   ```

5. Create a cluster role binding to give the Argo Rollouts service account the permissions from the cluster role. 
   ```yaml
   kubectl apply -f- <<EOF
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRoleBinding
   metadata:
     name: gateway-admin
   roleRef:
     apiGroup: rbac.authorization.k8s.io
     kind: ClusterRole
     name: gateway-controller-role
   subjects:
     - namespace: argo-rollouts
       kind: ServiceAccount
       name: argo-rollouts
   EOF
   ```


## Step 3: Configure a rollout for a sample app

1. Create a stable and canary service for the `rollouts-demo` pod that you deploy in the next step.  
   ```yaml
   kubectl apply -f- <<EOF
   apiVersion: v1
   kind: Service
   metadata:
     name: argo-rollouts-stable-service
     namespace: default
   spec:
     ports:
       - port: 80
         targetPort: http
         protocol: TCP
         name: http
     selector:
       app: rollouts-demo
   ---
   apiVersion: v1
   kind: Service
   metadata:
     name: argo-rollouts-canary-service
     namespace: default
   spec:
     ports:
       - port: 80
         targetPort: http
         protocol: TCP
         name: http
     selector:
       app: rollouts-demo
   EOF
   ```

2. Create an Argo Rollout that deploys the `rollouts-demo` pod. Add your stable and canary services to the `spec.strategy.canary` section. 
   ```yaml
   kubectl apply -f- <<EOF
   apiVersion: argoproj.io/v1alpha1
   kind: Rollout
   metadata:
     name: rollouts-demo
     namespace: default
   spec:
     replicas: 3
     strategy:
       canary:
         canaryService: argo-rollouts-canary-service # our created canary service
         stableService: argo-rollouts-stable-service # our created stable service
         trafficRouting:
           plugins:
             argoproj-labs/gatewayAPI:
               httpRoute: argo-rollouts-http-route # our created httproute
               namespace: default
         steps:
         - setWeight: 30
         - pause: { duration: 30s }
         - setWeight: 60
         - pause: { duration: 30s }
         - setWeight: 100
         - pause: { duration: 30s }
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
             image: kostiscodefresh/summer-of-k8s-app:v1
             ports:
               - name: http
                 containerPort: 8080
                 protocol: TCP
             resources:
               requests:
                 memory: 32Mi
                 cpu: 5m
   EOF
   ```

3. Create an HTTP Gateway that that is managed by Argo Rollouts. 
   ```yaml
   kubectl apply -f- <<EOF
   apiVersion: gateway.networking.k8s.io/v1beta1
   kind: Gateway
   metadata:
     name: gloo
     namespace: default
   spec:
     gatewayClassName: gloo-gateway
     listeners:
       - name: http
         protocol: HTTP
         port: 80
   EOF
   ```

4. Create an HTTPRoute that is associated with the `Gateway` managed by Argo Rollouts and that can route to the stable and canary services that you set up earlier. 
   ```yaml
   kubectl apply -f- <<EOF
   kind: HTTPRoute
   apiVersion: gateway.networking.k8s.io/v1beta1
   metadata:
     name: argo-rollouts-http-route
     namespace: default
   spec:
     parentRefs:
       - name: gloo
     hostnames:
     - "demo.example.com"
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
   EOF
   ```

## Step 4: Test a sample rollout

1. Get the IP address of the gateway. 
   ```shell
   export GATEWAY_IP=$(kubectl get gateway gloo -o=jsonpath="{.status.addresses[0].value}")
   ```

   If you try out this guide in a test setup, such as kind, you must port-forward the gateway pod instead. 

2. Send a request to the `rollouts-demo` app.
   ```
   curl -H "host: demo.example.com" $GATEWAY_IP/callme
   ```

   Example output: 
   ```shell
   <div class='pod' style='background:#44B3C2'> ver: 1.0
    </div>%
   ```

3. Change the manifest to use the `v2` tag to start a rollout of your app. Argo Rollouts automatically starts splitting traffic between version 1 and version 2 of the app for the duration of the rollout.
   ```shell
   kubectl patch rollout rollouts-demo -n default \
     --type='json' -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/image", "value":"kostiscodefresh/summer-of-k8s-app:v2"}]'
   ```

4. Send a few more requests to your app. Because traffic is split between version 1 and version 2 of the app, you see responses from both app versions until the rollout is completed.
   ```shell
   while true; do curl -H "host: demo.example.com" $GATEWAY_IP/callme; done
   ```

   Example output:
   ```
   <div class='pod' style='background:#F1A94E'> ver: 2.0
   </div><div class='pod' style='background:#F1A94E'> ver: 2.0
   </div><div class='pod' style='background:#44B3C2'> ver: 1.0
   </div><div class='pod' style='background:#44B3C2'> ver: 1.0
   </div><div class='pod' style='background:#F1A94E'> ver: 2.0
   ```
