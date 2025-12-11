CURRENT_DIR=$(shell pwd)
DIST_DIR=${CURRENT_DIR}/dist
E2E_CLUSTER_NAME=gatewayapi-plugin-e2e
IS_E2E_CLUSTER=$(shell kind get clusters | grep -e "^${E2E_CLUSTER_NAME}$$")

# Versions of components used in e2e tests
GATEWAY_API_VERSION=v1.4.0
# See more versions at https://artifacthub.io/packages/helm/argo/argo-rollouts
ARGO_ROLLOUTS_HELM_VERSION=2.37.2 # Contains Argo Rollouts 1.7.1
# See more versions at https://artifacthub.io/packages/helm/traefik/traefik
TRAEFIK_HELM_VERSION=31.0.0 # Contains Traefik proxy v3.1.2



CLUSTER_DELETE ?= true
RUN ?= ''

define add_helm_repo
	helm repo add traefik https://traefik.github.io/charts
	helm repo add argo https://argoproj.github.io/argo-helm
endef

define setup_cluster
	kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/${GATEWAY_API_VERSION}/experimental-install.yaml
	helm install argo-rollouts argo/argo-rollouts --values ./test/cluster-setup/argo-rollouts-values.yml --version ${ARGO_ROLLOUTS_HELM_VERSION} --wait
	helm install traefik traefik/traefik --values ./test/cluster-setup/traefik-values.yml --version ${TRAEFIK_HELM_VERSION} --wait
endef

define install_k8s_resources
	kubectl apply -f ./examples/traefik/stable.yml
	kubectl apply -f ./examples/traefik/canary.yml
endef

.PHONY: install-dependencies
install-dependencies:
	go mod download

.PHONY: release
release:
	make BIN_NAME=gatewayapi-plugin-darwin-amd64 GOOS=darwin GOARCH=amd64 gatewayapi-plugin-build
	make BIN_NAME=gatewayapi-plugin-darwin-arm64 GOOS=darwin GOARCH=arm64 gatewayapi-plugin-build
	make BIN_NAME=gatewayapi-plugin-linux-amd64 GOOS=linux GOARCH=amd64 gatewayapi-plugin-build
	make BIN_NAME=gatewayapi-plugin-linux-arm64 GOOS=linux GOARCH=arm64 gatewayapi-plugin-build
	make BIN_NAME=gatewayapi-plugin-windows-amd64.exe GOOS=windows gatewayapi-plugin-build

.PHONY: gatewayapi-plugin-build
gatewayapi-plugin-build:
	CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build -v -o ${DIST_DIR}/${BIN_NAME} .

.PHONY: local-build
local-build:
	go build -gcflags=all="-N -l" -o gatewayapi-plugin

.PHONY: lint
lint:
	golangci-lint run --fix

.PHONY: unit-tests
unit-tests:
	go test -v -count=1 ./pkg/...

.PHONY: setup-e2e-cluster
setup-e2e-cluster:	
	make BIN_NAME=gatewayapi-plugin-linux-amd64 GOOS=linux GOARCH=amd64 gatewayapi-plugin-build
ifeq (${IS_E2E_CLUSTER},)
	kind create cluster --name ${E2E_CLUSTER_NAME} --config ./test/cluster-setup/cluster-config.yml
	$(call add_helm_repo)
	$(call setup_cluster)
	$(call install_k8s_resources)
endif

.PHONY: e2e-tests
e2e-tests: setup-e2e-cluster run-e2e-tests
ifeq (${CLUSTER_DELETE},true)
	make clear-e2e-cluster
endif

.PHONY: sanity-check-e2e
sanity-check-e2e:
	./test/cluster-setup/sanity-check.sh

.PHONY: run-e2e-tests 
run-e2e-tests: sanity-check-e2e
	go test -v -timeout 5m -count=1 -run ${RUN} ./test/e2e/...

# Flaky tests usually fail with GitHub actions. You should be able to run them locally though.
.PHONY: e2e-tests-flaky
e2e-tests-flaky: setup-e2e-cluster run-e2e-tests-flaky 
ifeq (${CLUSTER_DELETE},true)
	make clear-e2e-cluster
endif

.PHONY: run-e2e-tests-flaky 
run-e2e-tests-flaky: sanity-check-e2e
	go test -tags "flaky" -v -timeout 5m -count=1 -run ${RUN} ./test/e2e/... 

.PHONY: clear-e2e-cluster
clear-e2e-cluster:
	kind delete cluster --name ${E2E_CLUSTER_NAME}

# convenience target to run `mkdocs serve` using a docker container
.PHONY: serve-docs
serve-docs:  ## serve docs locally
	docker run --rm -it -p 8000:8000 -v ${CURRENT_DIR}:/docs squidfunk/mkdocs-material serve -a 0.0.0.0:8000


