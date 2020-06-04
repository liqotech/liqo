
# Image URL to use all building/pushing image targets
IMG ?= aldokcl/dronet:development
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

# Run tests
test: generate fmt vet manifests
	go test ./... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate manifests
	go run ./cmd/liqonet/main.go

# Install CRDs into a cluster
install:
	kubectl kustomize config/liqonet/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall:
	kubectl kustomize config/liqonet/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config/liqonet/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
    #$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/liqonet/crd/bases output:rbac:artifacts:config=config/liqonet/rbac
	#$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./internal/liqonet/route-operator.go" rbac:roleName=manager-role crd:crdVersions=v1beta1 crd:trivialVersions=false output:crd:artifacts:config=config/liqonet/crd/bases output:rbac:artifacts:config=config/liqonet/rbac
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./api/tunnel-endpoint/v1" rbac:roleName=manager-role crd:crdVersions=v1beta1 crd:trivialVersions=false output:crd:artifacts:config=config/liqonet/crd/bases output:rbac:artifacts:config=config/liqonet/rbac
	rm config/dronet.drone.com_tunnelendpoints.yaml

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile=./hack/boilerplate.go.txt paths="./..."

# Build the docker image
docker-build:
	docker build -f build/liqonet/Dockerfile . -t ${IMG}

# Push the docker image
docker-push: docker-build
	docker push ${IMG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.4 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif
