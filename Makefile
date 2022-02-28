# Image URL to use all building/pushing image targets
TAG ?= v0.7.0-alpha
REPO ?= openyurt
MANAGER_IMG ?= ${REPO}/yurtcluster-operator-manager:${TAG}
AGENT_IMG ?= ${REPO}/yurtcluster-operator-agent:${TAG}

# Go ldflags for version vars
GO_LD_FLAGS ?= $(shell hack/lib/version.sh yurt::version::ldflags)

# Build linux/amd64 arch with `make release-artifacts DOCKER_BUILD_PLATFORMS=linux/amd64`
DOCKER_BUILD_PLATFORMS ?= linux/amd64,linux/arm64,linux/arm/v7
DOCKER_BUILD_GO_PROXY_ARG ?= GO_PROXY=https://goproxy.cn,direct
DOCKER_BUILD_GO_LD_FLAGS_ARG ?= GO_LD_FLAGS="${GO_LD_FLAGS}"

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager agent edgectl

# Run tests
test: generate fmt vet manifests
	go test ./... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build --ldflags "${GO_LD_FLAGS}" -o bin/manager cmd/manager/manager.go

# Build agent binary
agent: generate fmt vet
	go build --ldflags "${GO_LD_FLAGS}" -o bin/agent cmd/agent/agent.go

# Build edgectl binary
edgectl: generate fmt vet
	go build --ldflags "${GO_LD_FLAGS}" -o bin/edgectl cmd/edgectl/edgectl.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run --ldflags "${GO_LD_FLAGS}" ./cmd/manager/manager.go

# Install CRDs into a cluster
install: manifests
	kustomize build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	cd config/manager && kustomize edit set image yurtcluster-operator-manager=${MANAGER_IMG}
	cd config/agent && kustomize edit set image yurtcluster-operator-agent=${AGENT_IMG}
	kustomize build config/default | kubectl apply -f -

# Release manifests into docs/manifests and push docker image to dockerhub
release-artifacts: docker-push release-manifests

# Release manifests into docs/manifests
release-manifests: manifests
	cd config/manager && kustomize edit set image yurtcluster-operator-manager=${MANAGER_IMG}
	cd config/agent && kustomize edit set image yurtcluster-operator-agent=${AGENT_IMG}
	kustomize build config/default > docs/manifests/deploy.yaml

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." \
		output:crd:artifacts:config=config/crd/bases
	cp config/crd/bases/* charts/yurtcluster-operator/crds

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Lint codebase
lint: golangci-lint
	$(GOLANGCI_LINT) run

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image (only linux/amd64 arch)
docker-build: docker-build-manager docker-build-agent

docker-build-manager:
	docker buildx build --load --platform linux/amd64 -f Dockerfile . -t ${MANAGER_IMG} \
		--build-arg ${DOCKER_BUILD_GO_PROXY_ARG} --build-arg ${DOCKER_BUILD_GO_LD_FLAGS_ARG}
docker-build-agent:
	docker buildx build --load --platform linux/amd64 -f Dockerfile.agent . -t ${AGENT_IMG} \
		--build-arg ${DOCKER_BUILD_GO_PROXY_ARG} --build-arg ${DOCKER_BUILD_GO_LD_FLAGS_ARG}

# Push the docker images with multi-arch
docker-push: docker-push-manager docker-push-agent

docker-push-manager:
	docker buildx build --push --platform ${DOCKER_BUILD_PLATFORMS} -f Dockerfile . -t ${MANAGER_IMG} \
		--build-arg ${DOCKER_BUILD_GO_PROXY_ARG} --build-arg ${DOCKER_BUILD_GO_LD_FLAGS_ARG}
docker-push-agent:
	docker buildx build --push --platform ${DOCKER_BUILD_PLATFORMS} -f Dockerfile.agent . -t ${AGENT_IMG} \
		--build-arg ${DOCKER_BUILD_GO_PROXY_ARG} --build-arg ${DOCKER_BUILD_GO_LD_FLAGS_ARG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

# find or download golangci-lint
golangci-lint:
ifeq (, $(shell which golangci-lint))
	@{ \
	set -e ;\
	GOLANGCI_LINT_TMP_DIR=$$(mktemp -d) ;\
	cd $$GOLANGCI_LINT_TMP_DIR ;\
	go mod init tmp ;\
	go get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.42.1 ;\
	rm -rf $$GOLANGCI_LINT_TMP_DIR ;\
	}
GOLANGCI_LINT=$(GOBIN)/golangci-lint
else
GOLANGCI_LINT=$(shell which golangci-lint)
endif

update-helm-package: # update helm repo
	chmod a+x ./hack/update-helm-package.sh && ./hack/update-helm-package.sh

verify-helm-package: # verify helm repo
	chmod a+x ./hack/verify-helm-package.sh && ./hack/verify-helm-package.sh