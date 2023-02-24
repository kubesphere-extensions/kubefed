IMG ?= iawia002/kubefed:latest

controller:
	docker build -f build/controller/Dockerfile . -t ${IMG}

controller-push:
	docker buildx build --platform linux/amd64,linux/arm64 -f build/controller/Dockerfile . -t ${IMG} --push
