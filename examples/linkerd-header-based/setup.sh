#!/bin/bash
kind delete cluster &>/dev/null
kind create cluster --config manifests/kind-cluster.yaml
kubectl ns default

linkerd install --crds | kubectl apply -f -

linkerd install | kubectl apply -f - && linkerd check

linkerd viz install | kubectl apply -f - && linkerd check

kubectl create namespace argo-rollouts
kubectl apply -n argo-rollouts -f https://github.com/argoproj/argo-rollouts/releases/latest/download/install.yaml
kubectl apply -k https://github.com/argoproj/argo-rollouts/manifests/crds\?ref\=stable

kubectl apply -k manifests/
kubectl rollout restart deploy -n argo-rollouts

