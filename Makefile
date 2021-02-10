
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

gen: generate fmt vet manifests

#run all tests
test: unit e2e

# Run unit tests
unit: gen
	go test $(shell go list ./... | grep -v "e2e")

# Run e2e tests
e2e: gen
	go test $(shell go list ./... | grep "e2e")

# Install LIQO into a cluster
install: manifests
	./install.sh

# Uninstall LIQO from a cluster
uninstall: manifests
	./install.sh --uninstall

# Uninstall LIQO from a cluster with purge flag
purge: manifests
	./install.sh --uninstall --purge

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./..." output:crd:artifacts:config=deployments/liqo/crds

#Generate RBAC for each controller
rbacs: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./internal/advertisement-operator" rbac:roleName=liqo-advertisement output:rbac:stdout | sed -n '/rules/,$$p' > deployments/liqo/files/liqo-advertisement-rbac.yaml
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./internal/liqonet/route-operator" rbac:roleName=liqo-route output:rbac:stdout | sed -n '/rules/,$$p' > deployments/liqo/files/liqo-route-rbac.yaml
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./internal/liqonet/tunnel-operator" rbac:roleName=liqo-gateway output:rbac:stdout | sed -n '/rules/,$$p' > deployments/liqo/files/liqo-gateway-rbac.yaml
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./internal/crdReplicator" rbac:roleName=liqo-crd-replicator output:rbac:stdout | sed -n '/rules/,$$p' > deployments/liqo/files/liqo-crd-replicator-rbac.yaml

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."


# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif