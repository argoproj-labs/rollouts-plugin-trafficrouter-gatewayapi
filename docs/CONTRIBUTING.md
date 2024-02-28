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

## Building

`go.mod` is used, so the `go build/test` commands automatically install the needed dependencies

The `make` command will build the plugin.




<!-- ## Running the plugin Locally

It is much easier to run and debug if you run Argo Rollout in your local machine than in the Kubernetes cluster.

```bash
cd ~/go/src/github.com/argoproj/argo-rollouts
go run ./cmd/rollouts-controller/main.go
```

When running locally it will connect to whatever kubernetes cluster you have configured in your kubeconfig. You will need to make sure to install the Argo Rollout CRDs into your local cluster, and have the `argo-rollouts` namespace. -->

## Running Unit Tests

<!-- To run unit tests:

```bash
make test
``` -->

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
