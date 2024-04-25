CURRENT_DIR=$(shell pwd)
DIST_DIR=${CURRENT_DIR}/dist

.PHONY: release
release:
	make BIN_NAME=gateway-api-plugin-darwin-amd64 GOOS=darwin gateway-api-plugin-build
	make BIN_NAME=gateway-api-plugin-darwin-arm64 GOOS=darwin GOARCH=arm64 gateway-api-plugin-build
	make BIN_NAME=gateway-api-plugin-linux-amd64 GOOS=linux gateway-api-plugin-build
	make BIN_NAME=gateway-api-plugin-linux-arm64 GOOS=linux GOARCH=arm64 gateway-api-plugin-build
	make BIN_NAME=gateway-api-plugin-windows-amd64.exe GOOS=windows gateway-api-plugin-build

.PHONY: gateway-api-plugin-build
gateway-api-plugin-build:
	CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build -v -o ${DIST_DIR}/${BIN_NAME} .

.PHONY: local-build
local-build:
	go build -gcflags=all="-N -l" -o gatewayapi-plugin

.PHONY: lint
lint:
	golangci-lint run --fix

.PHONY: test
	go test -v ./...

# convenience target to run `mkdocs serve` using a docker container
.PHONY: serve-docs
serve-docs:  ## serve docs locally
	docker run --rm -it -p 8000:8000 -v ${CURRENT_DIR}:/docs squidfunk/mkdocs-material serve -a 0.0.0.0:8000


