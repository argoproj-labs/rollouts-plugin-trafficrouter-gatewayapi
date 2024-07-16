CURRENT_DIR=$(shell pwd)
DIST_DIR=${CURRENT_DIR}/dist

.PHONY: install-dependencies
install-dependencies:
	go mod download

.PHONY: release
release:
	make BIN_NAME=gatewayapi-plugin-darwin-amd64 GOOS=darwin gateway-api-plugin-build
	make BIN_NAME=gatewayapi-plugin-darwin-arm64 GOOS=darwin GOARCH=arm64 gateway-api-plugin-build
	make BIN_NAME=gatewayapi-plugin-linux-amd64 GOOS=linux gateway-api-plugin-build
	make BIN_NAME=gatewayapi-plugin-linux-arm64 GOOS=linux GOARCH=arm64 gateway-api-plugin-build
	make BIN_NAME=gatewayapi-plugin-windows-amd64.exe GOOS=windows gateway-api-plugin-build

.PHONY: gateway-api-plugin-build
gateway-api-plugin-build:
	CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build -v -o ${DIST_DIR}/${BIN_NAME} .

.PHONY: local-build
local-build:
	go build -gcflags=all="-N -l" -o gatewayapi-plugin

.PHONY: lint
lint:
	golangci-lint run --fix

<<<<<<< Updated upstream
.PHONY: test
test:
	go test -v ./...
=======
.PHONY: unit-tests
unit-tests:
	go test -v ./pkg/...

.PHONY: setup-e2e-cluster
setup-e2e-cluster: release
	kind create cluster --name gatewayapi-plugin-e2e --config ./test/cluster-setup/cluster-config.yml
	helm install argo-rollouts argo/argo-rollouts --values ./test/cluster-setup/argo-rollouts-values.yml
	helm install traefik traefik/traefik --values ./test/cluster-setup/traefik-values.yml
	kubectl apply -f ./examples/traefik/stable.yml
	kubectl apply -f ./examples/traefik/canary.yml
	kubectl apply -f ./examples/traefik/httproute.yml
	kubectl apply -f ./examples/traefik/rollout.yml

.PHONY: e2e-tests
e2e-tests: setup-e2e-cluster
	go test -v ./test/e2e/...

.PHONY: clear-e2e-cluster
clear-e2e-cluster:
	kind delete cluster --name gatewayapi-plugin-e2e
>>>>>>> Stashed changes

# convenience target to run `mkdocs serve` using a docker container
.PHONY: serve-docs
serve-docs:  ## serve docs locally
	docker run --rm -it -p 8000:8000 -v ${CURRENT_DIR}:/docs squidfunk/mkdocs-material serve -a 0.0.0.0:8000


