# Using kgateway with Argo Rollouts

This guide will describe how to use kgateway Kubernetes Gateway as an implementation
for the Gateway API in order to do split traffic with Argo Rollouts.

Versions used in this guide:
- Kubernetes Gateway API: v1.4.0
- kgateway: v2.3.0
- argo-rollouts: v1.8.4
- rollouts-plugin-trafficrouter-gatewayapi: v0.11.0

Dependency:
- [argo rollouts](https://argo-rollouts.readthedocs.io/en/stable/installation/#kubectl-plugin-installation) kubectl plugin

## Step 1 - Install kgateway and Argo Rollouts

### 1 - Install kgateway
This installation creates a `kgateway` GatewayClass, which you will use later. You can follow the instructions below to install via Helm, or refer to the [installation instructions](https://kgateway.dev/docs/envoy/main/install/) for other methods.

1. Install the custom resources of the Kubernetes Gateway API version 1.4.0.
   ```shell
   kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.4.0/standard-install.yaml
   ```

   Example output:
   ```shell
   customresourcedefinition.apiextensions.k8s.io/gatewayclasses.gateway.networking.k8s.io created
   customresourcedefinition.apiextensions.k8s.io/gateways.gateway.networking.k8s.io created
   customresourcedefinition.apiextensions.k8s.io/httproutes.gateway.networking.k8s.io created
   customresourcedefinition.apiextensions.k8s.io/referencegrants.gateway.networking.k8s.io created
   customresourcedefinition.apiextensions.k8s.io/grpcroutes.gateway.networking.k8s.io created
   ```

2. Deploy the kgateway CRDs by using Helm. This command creates the kgateway-system namespace and creates the kgateway CRDs in the cluster.
   ```shell
   helm upgrade -i --create-namespace \
     --namespace kgateway-system \
     --version v2.2.2 kgateway-crds oci://cr.kgateway.dev/kgateway-dev/charts/kgateway-crds 
   ```

3. Install the kgateway Helm chart.
   ```shell
   helm upgrade -i -n kgateway-system kgateway oci://cr.kgateway.dev/kgateway-dev/charts/kgateway \
   --version v2.2.2
   ```

   Example output:
   ```shell
   NAME: kgateway
   LAST DEPLOYED: Thu Feb 13 14:03:51 2025
   NAMESPACE: kgateway-system
   STATUS: deployed
   REVISION: 1
   TEST SUITE: None
   ```

4. Verify that the control plane is up and running.
   ```shell
   kubectl get pods -n kgateway-system
   ```
   
   Example output:
   ```shell
   NAME                                  READY   STATUS    RESTARTS   AGE
   kgateway-78658959cd-cz6jt             1/1     Running   0          12s
   ```

5. Verify that the kgateway GatewayClass is created.
   ```shell
   kubectl get gatewayclass kgateway
   ```
   
   Example output:
   ```shell
   NAME         CONTROLLER               ACCEPTED   AGE   
   kgateway     kgateway.dev/kgateway    True       6m36s
   ```

### 2 - Install Argo Rollouts
Make sure you also install Argo Rollouts. Follow the [kgateway Argo Rollouts integration guide](https://kgateway.dev/docs/envoy/main/integrations/argo/#install-argo-rollouts) to install Argo Rollouts.

## Step 2 - Split traffic with Gateway resources

Create a dedicated gateway that splits traffic across your Argo Rollouts resources.

1. Create a Gateway.
   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: gateway.networking.k8s.io/v1beta1
   kind: Gateway
   metadata:
     name: argo-rollouts-gateway
     namespace: argo-rollouts
   spec:
     gatewayClassName: kgateway
     listeners:
       - protocol: HTTP
         name: web
         port: 80
   EOF
   ```

2. Create and attach an HTTPRoute to the Gateway.
   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: gateway.networking.k8s.io/v1beta1
   kind: HTTPRoute
   metadata:
     name: argo-rollouts-http-route
     namespace: argo-rollouts
   spec:
     parentRefs:
       - name: argo-rollouts-gateway
         namespace: argo-rollouts
     rules:
       - matches:
         backendRefs:
           - name: argo-rollouts-stable-service
             port: 80
           - name: argo-rollouts-canary-service
             port: 80
   EOF
   ```

## Step 3 - Create Services for your application

1. Create a canary service.
   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: v1
   kind: Service
   metadata:
     name: argo-rollouts-canary-service
     namespace: argo-rollouts
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

2. Create a stable service.
   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: v1
   kind: Service
   metadata:
     name: argo-rollouts-stable-service
     namespace: argo-rollouts
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

## Step 4 - Grant argo-rollouts permissions

Grant argo-rollouts permissions to view and modify Gateway HTTPRoute resources. The argo-rollouts service account needs the ability to be able to view and modify HTTPRoutes and Configmaps as well as its existing permissions. Edit the `argo-rollouts` cluster role to add the following permissions or use the RBAC example provided in the [kgateway documentation](https://kgateway.dev/docs/envoy/main/integrations/argo/#create-rbac-rules-for-argo):

1. Create a ClusterRole.
   ```yaml
   kubectl apply -f- <<EOF
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: gateway-controller-role
     namespace: argo-rollouts
   rules:
     - apiGroups:
         - "gateway.networking.k8s.io"
       resources:
         - "httproutes"
       verbs:
         - get
         - list
         - watch
         - update
         - patch
     - apiGroups:
         - ""
       resources:
         - "configmaps"
       verbs:
         - get
         - list
         - watch
         - create
         - update
         - patch
         - delete
   EOF
   ```

2. Create a ClusterRoleBinding.
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

## Step 5 - Create argo-rollouts resources

1. Create the argo-rollouts resources.
   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: argoproj.io/v1alpha1
   kind: Rollout
   metadata:
     name: rollouts-demo
     namespace: argo-rollouts
   spec:
     replicas: 5
     strategy:
       canary:
         canaryService: argo-rollouts-canary-service # our created canary service
         stableService: argo-rollouts-stable-service # our created stable service
         trafficRouting:
           plugins:
             argoproj-labs/gatewayAPI:
               httpRoute: argo-rollouts-http-route # our created httproute
               namespace: argo-rollouts # namespace where this rollout resides
         steps:
           - setWeight: 30
           - pause: {}
           - setWeight: 40
           - pause: { duration: 10 }
           - setWeight: 60
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
             image: argoproj/rollouts-demo:red
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

## Step 6 - Test Application

1. Port-forward the Gateway service.
   ```
   kubectl port-forward svc/argo-rollouts-gateway 8080:80 -n argo-rollouts
   ```

2. You can now test the application. An argo application shows up with red dots.
   - Via browser:
   ```shell
   http://localhost:8080
   ```

   - Or via terminal:
   ```shell
   curl http://localhost:8080/
   curl http://localhost:8080/color
   ```

## Step 7 - Test the promotion

1. Update the Image of the rollout to blue or a different color.
   ```shell
   kubectl argo rollouts set image rollouts-demo rollouts-demo=argoproj/rollouts-demo:blue -n argo-rollouts
   ```

   Or update the image under containers in rollout.yml to blue or a different color (such as `image: argoproj/rollouts-demo:blue`) and apply the `rollout.yaml` again.
   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: argoproj.io/v1alpha1
   kind: Rollout
   metadata:
     name: rollouts-demo
     namespace: argo-rollouts
   spec:
     replicas: 5
     strategy:
       canary:
         canaryService: argo-rollouts-canary-service # our created canary service
         stableService: argo-rollouts-stable-service # our created stable service
         trafficRouting:
           plugins:
             argoproj-labs/gatewayAPI:
               httpRoute: argo-rollouts-http-route # our created httproute
               namespace: argo-rollouts # namespace where this rollout resides
         steps:
           - setWeight: 30
           - pause: {}
           - setWeight: 40
           - pause: { duration: 10 }
           - setWeight: 60
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
             image: argoproj/rollouts-demo:blue # Change the image here.
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

2. Check the weight difference between the stable and canary service.
   ```shell
   kubectl get httproute argo-rollouts-http-route -o yaml -n argo-rollouts
   ```
   
   Or use the following command to watch the promotion progress in real time. Make sure you have installed the [argo-rollouts](https://argo-rollouts.readthedocs.io/en/stable/installation/#kubectl-plugin-installation) extension for kubectl.
   ```shell
   kubectl argo rollouts get rollout rollouts-demo -n argo-rollouts --watch
   ```

3. Promote the rollout. Make sure you have installed the [argo-rollouts](https://argo-rollouts.readthedocs.io/en/stable/installation/#kubectl-plugin-installation) extension for kubectl.
   ```shell
   kubectl argo rollouts promote rollouts-demo -n argo-rollouts
   ```

4. Check again the weight difference between the stable and canary service.
   ```shell
   kubectl get httproute argo-rollouts-http-route -o yaml -n argo-rollouts
   ```

5. Verify changes. Now blue dots appear in the web GUI instead of red ones.
   - Via browser:
   ```shell
   http://localhost:8080
   ```

   - Or via terminal:
   ```shell
   curl http://localhost:8080/color
   ```