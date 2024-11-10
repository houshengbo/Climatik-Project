# Docker image variables
DOCKER_REGISTRY ?= quay.io/climatik-project
IMAGE_NAME ?= freqtuning-recommender
IMAGE_TAG ?= latest
IMG ?= $(DOCKER_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)

# Setting SHELL to bash allows bash commands to be executed by recipes.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

##@ General

# The help target prints out all targets with their descriptions organized
.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development
.PHONY: fmt
fmt: ## Auto-format Go code
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: tidy
tidy: ## Run go mod tidy to ensure dependencies are up to date
	go mod tidy

##@ Build
.PHONY: build
build: fmt vet tidy ## Build manager binary.
	go build -o bin/manager cmd/main.go

.PHONY: run
run: fmt vet ## Run the controller from your host.
	go run ./cmd/main.go

# Basic clean target
.PHONY: clean
clean: ## Clean up build artifacts and temporary files
	rm -rf bin/
	rm -f coverage.out

##@ Docker
.PHONY: docker-build
docker-build: tidy ## Build docker image with the manager.
	cd .. && docker build -t ${IMG} -f freqtuning-recommender/Dockerfile .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker push ${IMG}

##@ Deployment
.PHONY: deploy
deploy: docker-build docker-push ## Deploy controller to the K8s cluster
	kubectl apply -f config/rbac
	kubectl apply -f config/manager

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster
	kubectl delete -f config/manager
	kubectl delete -f config/rbac 