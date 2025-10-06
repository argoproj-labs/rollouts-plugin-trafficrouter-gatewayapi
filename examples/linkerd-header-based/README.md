# Using Linkerd with Argo Rollouts for header based traffic split

[Linkerd](https://linkerd.io/) is a service mesh for Kubernetes. It makes running services easier and safer by giving you runtime debugging, observability, reliability, and security—all without requiring any changes to your code.

## Prerequisites

A Kubernetes cluster. If you do not have one, you can create one using [kind](https://kind.sigs.k8s.io/), [minikube](https://minikube.sigs.k8s.io/), or any other Kubernetes cluster. This guide will use Kind.

## Step 1 - Create a Kind cluster by running the following command

```shell
kind create cluster --config ./kind-cluster.yaml
```

## Step 2 - Install Linkerd and Linkerd Viz by running the following commands

I will use the Linkerd CLI to install Linkerd in the cluster. You can also install Linkerd using Helm or kubectl.
I tested this guide with Linkerd version `edge-25.9.4`.

> [!IMPORTANT]
> Linkerd version `edge-25.9.4` uses `v1` GatewayAPI apiVersion and the [plugin](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi) `v0.8.0` expects that. It wouldn't work if `v1beta1` GatewayAPI apiVersion CRDs would be installed (like in the case of an older Linkerd `stable-2.14.10`)

```shell
export LINKERD2_VERSION=edge-25.9.4; curl --proto '=https' --tlsv1.2 -sSfL https://run.linkerd.io/install-edge | sh
export PATH=$PATH:$HOME/.linkerd2/bin
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.1/standard-install.yaml # Gateway API CRDs must be installed prior to installing Linkerd
linkerd install --crds | kubectl apply -f -
linkerd install | kubectl apply -f - && linkerd check
linkerd viz install | kubectl apply -f - && linkerd check
```

## Step 3 - Install Argo Rollouts and Argo Rollouts plugin to instruct Linkerd to manage the traffic

```shell
kubectl create namespace argo-rollouts
kubectl apply -n argo-rollouts -f https://github.com/argoproj/argo-rollouts/releases/latest/download/install.yaml
kubectl apply -k https://github.com/argoproj/argo-rollouts/manifests/crds\?ref\=stable
kubectl apply -f argo-rollouts-plugin.yaml
kubectl rollout restart deploy -n argo-rollouts
```

## Step 4 - Grant Argo Rollouts SA access to the HTTPRoute
```shell
kubectl apply -f cluster-role.yaml
```
__Note:__ These permission are very permissive. You should lock them down according to your needs.

With the following role we allow Argo Rollouts to have Admin access to HTTPRoutes and Gateways.

```shell
kubectl apply -f cluster-role-binding.yaml
```
## Step 5 - Create HTTPRoute that defines a traffic split between two services

Create a GAMMA [producer `HTTPRoute`](https://gateway-api.sigs.k8s.io/concepts/glossary/#producer-route) resource and connect it to a parent K8s service (using a canary and stable K8s services as backends)

```shell
kubectl apply -f httproute.yaml
```

## Step 6 - Create the services required for traffic split

Create three Services required for canary based rollout strategy

```shell
kubectl apply -f service.yaml
```

## Step 7 - Add `linkerd.io/inject: enabled` annotation to namespace

Add Linkerd annotation to the namespace where the pods are deployed to enable [Automatic Proxy Injection](https://linkerd.io/2-edge/features/proxy-injection/)

```shell
kubectl apply -f namespace.yaml
```

## Step 8 - Create an example Rollout

Deploy a rollout to get the initial version.

```shell
kubectl apply -f rollout.yaml
```

## Step 9 - Watch the rollout

Monitor the HTTPRoute configuration to see how traffic is split and header-based routing is configured

```shell
watch "kubectl get httproute.gateway.networking.k8s.io/argo-rollouts-http-route -o jsonpath='{\" HEADERS: \"}{.spec.rules[*].matches[*]}'"
```

## Step 10 - Patch the rollout to see the canary deployment
```shell
kubectl patch rollout rollouts-demo --type='json' -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/env/0/value", "value": "1.1.0"}]'
```

## Step 11 - Observe the rollout and HTTPRoute rule addition of [canary header matching rule](https://gateway-api.sigs.k8s.io/guides/traffic-splitting/#canary-traffic-rollout)

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
Images:          hashicorp/http-echo (canary, stable)
Replicas:
  Desired:       5
  Current:       6
  Updated:       1
  Ready:         6
  Available:     6

NAME                                       KIND        STATUS     AGE    INFO
⟳ rollouts-demo                            Rollout     ॥ Paused   4m53s
├──# revision:2
│  └──⧉ rollouts-demo-8598f766fd           ReplicaSet  ✔ Healthy  67s    canary
│     └──□ rollouts-demo-8598f766fd-2cvt4  Pod         ✔ Running  31s    ready:2/2
└──# revision:1
   └──⧉ rollouts-demo-5d78c448f9           ReplicaSet  ✔ Healthy  4m53s  stable
      ├──□ rollouts-demo-5d78c448f9-9zttx  Pod         ✔ Running  4m53s  ready:2/2
      ├──□ rollouts-demo-5d78c448f9-hpcfg  Pod         ✔ Running  4m53s  ready:2/2
      ├──□ rollouts-demo-5d78c448f9-l757w  Pod         ✔ Running  4m53s  ready:2/2
      ├──□ rollouts-demo-5d78c448f9-xl72c  Pod         ✔ Running  4m53s  ready:2/2
      └──□ rollouts-demo-5d78c448f9-zk5pd  Pod         ✔ Running  4m53s  ready:2/2
$
$ kubectl get httproute argo-rollouts-http-route -o yaml | yq .spec.rules
- backendRefs:
    - group: core
      kind: Service
      name: argo-rollouts-stable-service
      port: 80
      weight: 100
    - group: core
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
$ kubectl exec -it network-test -c network-test -- sh
~ # # stable K8s service targets any of the 5 stable pods
~ # curl http://argo-rollouts-stable-service/
Hello from rollouts-demo-5d78c448f9-l757w
~ #
~ # # canary K8s service targets the one canary pod, created for the `setCanaryScale` step in the Rollout
~ # curl http://argo-rollouts-canary-service/
Hello from rollouts-demo-8598f766fd-2cvt4
~ #
~ # # GAMMA-type HTTPRoute's `.spec.parentRefs` K8s service only targets stable pods since no `setWeight` step is used in the Rollout
~ # seq 1 100 | xargs -P 10 -I {} bash -c 'curl -s http://argo-rollouts-service' > pods.txt
~ # sort pods.txt | uniq -c | sort -rn
     25 Hello from rollouts-demo-5d78c448f9-hpcfg
     21 Hello from rollouts-demo-5d78c448f9-xl72c
     19 Hello from rollouts-demo-5d78c448f9-zk5pd
     19 Hello from rollouts-demo-5d78c448f9-l757w
     16 Hello from rollouts-demo-5d78c448f9-9zttx
~ #
```

## Step 12 - Test the header-based routing with curl

You can test the header-based routing by sending requests with the specified header.
With header it always goes to canary:

```shell
$ kubectl exec -it network-test -c network-test -- sh
~ # seq 1 100 | xargs -P 10 -I {} bash -c 'curl -s -H "X-Test: test" http://argo-rollouts-service' > pods.txt
~ # sort pods.txt | uniq -c | sort -rn
    100 Hello from rollouts-demo-8598f766fd-2cvt4
~ #
```
