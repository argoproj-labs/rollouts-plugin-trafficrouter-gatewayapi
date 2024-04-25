# Contributing

!!! warning
    Page under construction.

## Before You Start
The Gateway Plugin for Argo Rollouts is written in Golang. If you do not have a good grounding in Go, try out [the tutorial](https://tour.golang.org/).

## Pre-requisites

Install:

* [docker](https://docs.docker.com/install/#supported-platforms)
* [golang](https://golang.org/)
* [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl)
* [kustomize](https://github.com/kubernetes-sigs/kustomize/releases) >= 4.5.5
* [k3d](https://k3d.io/) recommended


Checkout the code:

```bash
git clone https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-gatewayapi.git
cd rollouts-plugin-trafficrouter-gatewayapi
```

## Argo Rollouts plugin system architecture

## Project dependecies

`go.mod` is used, so the `go build/test` commands automatically install the needed dependencies

## Building

We have 2 targets in /Makefile:
1. **local-build** It is recommended to use this target "*make local-build*" to make not optimized build for local testing as debugger can't link optimized binary code with its go code to debug it correctly

2. **gateway-api-plugin-build** It is recommended to use this target "*make gateway-api-plugin-build*" to make optimized build for production when you are sure it is ready for it. We use it to create releases


## Running the plugin Locally

1. Create ConfigMap **argo-rollouts-config** in namespace of Argo Rollouts controller. We will run it locally so its namespace will be default
2. Run **make local-build** to make not optimized local build of plugin. Specify the path to this local build in the ConfigMap that we created before
```
file://<path to the local build>
```
3. Install needing CRDs for Argo Rollouts and apply its needing manifest. For that you can run
```bash
kubectl create namespace argo-rollouts
kubectl apply -n argo-rollouts -f https://github.com/argoproj/argo-rollouts/releases/latest/download/install.yaml
```
After delete in cluster Argo Rollouts controller deployment as we will run controller locally
4. Run locally Argo Rollouts controller
```bash
cd ~/go/src/github.com/argoproj/argo-rollouts
go run ./cmd/rollouts-controller/main.go
```
5. If you did all right, Argo Rollouts controller will find your local build of plugin and will run it as RPC server locally. You have ability to debug plugin. Debugger of go has ability to attach to the local process and as we built our plugin without optimizations it also can map binary code with text code of plugin correctly so you can use breakpoints and it will work

## Making releases

1. Write in **/RELEASE_NOTES.md** the description of the future release
2. On needing commit in **main** branch create locally tag
```bash
git tag release-v[0-9]+.[0-9]+.[0-9]+
```
If you would like to make pre-release run
```bash
git tag release-v[0-9]+.[0-9]+.[0-9]+-rc[0-9]+
```
3. Push tag to the remote repository
4. Pushed tag will trigger needing workflow that will create corresponding tag **v[0-9]+.[0-9]+.[0-9]+** or **v[0-9]+.[0-9]+.[0-9]+-rc[0-9]+** and will delete your tag so after pushing tag to the remote repository you need to delete it locally. When workflow will finish its work you can run **git pull** and you will see new tag

## Running Unit Tests

To run unit tests:
```bash
make test
```

## Running E2E tests

<!-- The end-to-end tests need to run against a kubernetes cluster with the Argo Rollouts controller
running. The rollout controller can be started with the command:

```
make start-e2e
```

Start and prepare your cluster for e2e tests:

```
k3d cluster create
kubectl create ns argo-rollouts
kubectl apply -k manifests/crds
kubectl apply -f test/e2e/crds
```

Then run the e2e tests:

```
make test-e2e
```

To run a subset of e2e tests, you need to specify the suite with `-run`, and the specific test regex with `-testify.m`.

```
E2E_TEST_OPTIONS="-run 'TestCanarySuite' -testify.m 'TestCanaryScaleDownOnAbortNoTrafficRouting'" make test-e2e 
``` -->


## Documentation Changes

Install Docker locally.
Modify contents in `docs/` directory.

Preview changes in your browser by visiting http://localhost:8000 after running:

```shell
make serve-docs
```

<!-- To publish changes, run:

```shell
make release-docs
``` -->
