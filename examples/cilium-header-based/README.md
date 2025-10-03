# Using Cilium with Argo Rollouts for header based traffic split

## Prerequisites

A Kubernetes cluster. If you do not have one, you can create one using [kind](https://kind.sigs.k8s.io/), [minikube](https://minikube.sigs.k8s.io/), or any other Kubernetes cluster. This guide will use Kind.

## Step 1 - Create a Kind cluster by running the following command

```shell
kind create cluster --config ./kind-cluster.yaml
```

## Step 2 - Install Cilium

I will use helm to install Cilium in the cluster, but before that we'll need to install Gateway API CRDs. You can also install Cilium using [cilium CLI](https://docs.cilium.io/en/stable/gettingstarted/k8s-install-default/#install-the-cilium-cli).

> [!NOTE]
> Cilium `v1.18.2` supports Gateway API v1.2.0, per [docs](https://docs.cilium.io/en/stable/network/servicemesh/gateway-api/gateway-api/).

```shell
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/gateway-api/v1.2.0/config/crd/standard/gateway.networking.k8s.io_gatewayclasses.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/gateway-api/v1.2.0/config/crd/standard/gateway.networking.k8s.io_gateways.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/gateway-api/v1.2.0/config/crd/standard/gateway.networking.k8s.io_httproutes.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/gateway-api/v1.2.0/config/crd/standard/gateway.networking.k8s.io_referencegrants.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/gateway-api/v1.2.0/config/crd/standard/gateway.networking.k8s.io_grpcroutes.yaml
```
```shell
helm repo add cilium https://helm.cilium.io/
helm repo update
helm install cilium cilium/cilium --version 1.18.2 \
     --namespace kube-system \
     --set image.pullPolicy=IfNotPresent \
     --set ipam.mode=kubernetes \
     --set cni.exclusive=false \
     --set kubeProxyReplacement=true \
     --set gatewayAPI.enabled=true \
     --wait
cilium status --wait
```

## Step 3 - Install Argo Rollouts and Argo Rollouts plugin to instruct Cilium to manage the traffic

```shell
helm repo add argo https://argoproj.github.io/argo-helm
helm repo update
helm install argo-rollouts argo/argo-rollouts --version 2.40.4 \
  --namespace argo-rollouts \
  --create-namespace \
  --set 'controller.trafficRouterPlugins[0].location=https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi/releases/download/v0.8.0/gatewayapi-plugin-linux-amd64' \
  --set 'controller.trafficRouterPlugins[0].name=argoproj-labs/gatewayAPI'
```

## Step 4 - Create the services required for traffic split

Create three Services required for canary based rollout strategy

```shell
kubectl apply -f service.yaml
```

## Step 5 - Create HTTPRoute that defines a traffic split between two services

> [!IMPORTANT]
> For Cilium the K8s Services refs need to use `group: ""`. This is different than Linkerd, where `group: "core"` could be used.
  ```yaml
  apiVersion: gateway.networking.k8s.io/v1beta1
  kind: HTTPRoute
  spec:
    parentRefs:
      - group: ""
        name:
        kind: Service
        port:
    rules:
      - backendRefs:
          - group: ""
            name:
            kind: Service
            port:
  ```

Create a GAMMA [producer `HTTPRoute`](https://gateway-api.sigs.k8s.io/concepts/glossary/#producer-route) resource and connect it to a parent K8s service (using a canary and stable K8s services as backends)

```shell
kubectl apply -f httproute.yaml
```

## Step 6 - Create an example Rollout

Deploy a rollout to get the initial version

```shell
kubectl apply -f rollout.yaml
```

## Step 7 - Patch the rollout to see the canary deployment
```shell
kubectl patch rollout rollouts-demo --type='json' -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/env/0/value", "value": "1.1.0"}]'
```

## Step 8 - Observe the rollout and HTTPRoute rule addition of [canary header matching rule](https://gateway-api.sigs.k8s.io/guides/traffic-splitting/#canary-traffic-rollout)

```shell
$ kubectl argo rollouts promote rollouts-demo  # promote to Rollout step 1
$ kubectl argo rollouts get rollout rollouts-demo
Name:            rollouts-demo
Namespace:       default
Status:          ॥ Paused
Message:         CanaryPauseStep
Strategy:        Canary
  Step:          3/5
  SetWeight:     0
  ActualWeight:  0
Images:          hashicorp/http-echo:1.0 (canary, stable)
Replicas:
  Desired:       5
  Current:       6
  Updated:       1
  Ready:         6
  Available:     6

NAME                                       KIND        STATUS     AGE   INFO
⟳ rollouts-demo                            Rollout     ॥ Paused   114s
├──# revision:2
│  └──⧉ rollouts-demo-7bd564d79f           ReplicaSet  ✔ Healthy  23s   canary
│     └──□ rollouts-demo-7bd564d79f-tshpg  Pod         ✔ Running  6s    ready:1/1
└──# revision:1
   └──⧉ rollouts-demo-784858d6db           ReplicaSet  ✔ Healthy  114s  stable
      ├──□ rollouts-demo-784858d6db-d799l  Pod         ✔ Running  114s  ready:1/1
      ├──□ rollouts-demo-784858d6db-hh44q  Pod         ✔ Running  114s  ready:1/1
      ├──□ rollouts-demo-784858d6db-nf2wh  Pod         ✔ Running  114s  ready:1/1
      ├──□ rollouts-demo-784858d6db-qn7dc  Pod         ✔ Running  114s  ready:1/1
      └──□ rollouts-demo-784858d6db-ww2q5  Pod         ✔ Running  114s  ready:1/1
$
$ kubectl get httproute argo-rollouts-http-route -o yaml | yq .spec.rules
- backendRefs:
    - group: ""
      kind: Service
      name: argo-rollouts-stable-service
      port: 80
      weight: 100
    - group: ""
      kind: Service
      name: argo-rollouts-canary-service
      port: 80
      weight: 0
  matches:
    - path:
        type: PathPrefix
        value: /
- backendRefs:
    - group: ""
      kind: Service
      name: argo-rollouts-canary-service
      port: 80
      weight: 0
  matches:
    - headers:
        - name: X-Test
          type: Exact
          value: test
      path:
        type: PathPrefix
        value: /
```
```shell
$ kubectl run -it --image nicolaka/netshoot:v0.13 network-test -- sh  # run a pod to source curl tests
~ #
~ # # stable K8s service targets any of the 5 stable pods
~ # seq 1 100 | xargs -P 10 -I {} bash -c 'curl -s http://argo-rollouts-stable-service' > pods.txt
~ # sort pods.txt | uniq -c | sort -rn
     26 Hello from rollouts-demo-784858d6db-qn7dc
     20 Hello from rollouts-demo-784858d6db-hh44q
     19 Hello from rollouts-demo-784858d6db-ww2q5
     18 Hello from rollouts-demo-784858d6db-d799l
     17 Hello from rollouts-demo-784858d6db-nf2wh
~ #
~ #
~ # # canary K8s service targets the one canary pod, created for the `setCanaryScale` step in the Rollout
~ # seq 1 100 | xargs -P 10 -I {} bash -c 'curl -s http://argo-rollouts-canary-service' > pods.txt
~ # sort pods.txt | uniq -c | sort -rn
    100 Hello from rollouts-demo-7bd564d79f-tshpg
~ #
~ #
~ # # GAMMA-type HTTPRoute's `.spec.parentRefs` K8s service only targets stable pods since no `setWeight` step is used in the Rollout
~ # seq 1 100 | xargs -P 10 -I {} bash -c 'curl -s http://argo-rollouts-service' > pods.txt
~ # sort pods.txt | uniq -c | sort -rn
     22 Hello from rollouts-demo-784858d6db-d799l
     21 Hello from rollouts-demo-784858d6db-hh44q
     20 Hello from rollouts-demo-784858d6db-qn7dc
     19 Hello from rollouts-demo-784858d6db-ww2q5
     18 Hello from rollouts-demo-784858d6db-nf2wh
~ #
```

## Step 12 - Test the header-based routing with curl

You can test the header-based routing by sending requests with the specified header.
With header it always goes to canary:

```shell
$ kubectl exec -it network-test -- sh
~ # seq 1 100 | xargs -P 10 -I {} bash -c 'curl -s -H "X-Test: test" http://argo-rollouts-service' > pods.txt
~ # sort pods.txt | uniq -c | sort -rn
    100 Hello from rollouts-demo-7bd564d79f-tshpg
~ #
```
