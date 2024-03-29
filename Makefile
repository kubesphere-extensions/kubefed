GOPATH ?= $(shell go env GOPATH)
BIN_DIR := $(GOPATH)/bin
GOLANGCI_LINT := $(BIN_DIR)/golangci-lint

IMG ?= iawia002/kubefed-extension:latest

.PHONY: lint test

lint: $(GOLANGCI_LINT)
	@$(GOLANGCI_LINT) run

$(GOLANGCI_LINT):
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(BIN_DIR) v1.55.2

test:
	@go test ./... -coverprofile=coverage.out
	@go tool cover -func coverage.out | tail -n 1 | awk '{ print "total: " $$3 }'

controller:
	docker build -f build/controller/Dockerfile . -t ${IMG}

controller-push:
	docker buildx build --platform linux/amd64,linux/arm64 -f build/controller/Dockerfile . -t ${IMG} --push
