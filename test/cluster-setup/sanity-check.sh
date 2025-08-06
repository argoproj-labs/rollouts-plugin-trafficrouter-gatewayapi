#!/bin/bash

# Exit immediately if any command fails
set -e

echo ">>> Sanity checks for e2e tests. If these tests fail, your e2e tests will also fail. <<<"
 
echo "Checking e2egateway class traefik with accepted condition=true ..."
kubectl get gatewayclasses traefik -o jsonpath='{.status.conditions[?(@.type=="Accepted")].status}' | grep -q "True"

echo "Checking e2egateway traefik-gateway with programmed condition=true ..."
kubectl get gateway traefik-gateway -o jsonpath='{.status.conditions[?(@.type=="Programmed")].status}' | grep -q "True"

echo "Checking e2e canary service..."
kubectl get svc argo-rollouts-canary-service > /dev/null

echo "Checking e2e stable service..."
kubectl get svc argo-rollouts-stable-service > /dev/null

echo ">>> Sanity checks finished. Your cluster is now ready for e2e tests. <<<"

