
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Set the capsule version to use
CAPSULE_VERSION = v0.1.0-rc2

gen: generate fmt vet manifests rbacs docs

#generate helm documentation
docs: helm-docs
	$(HELM_DOCS) -t deployments/liqo/README.gotmpl deployments/liqo

#run all tests
test: unit e2e

# Check if test image exists
test-container:
ifeq (, $(shell docker image ls | grep liqo-test))
	@{ \
	docker build -t liqo-test -f build/liqo-test/Dockerfile . ; \
	}
endif

# Fetch external CRDs
fetch-external-crds:
	mkdir -p externalcrds
	curl -s -o externalcrds/tenant-crd.yaml https://raw.githubusercontent.com/clastix/capsule/$(CAPSULE_VERSION)/charts/capsule/crds/tenant-crd.yaml
	curl -s -o externalcrds/capsuleconfiguration-crd.yaml https://raw.githubusercontent.com/clastix/capsule/$(CAPSULE_VERSION)/charts/capsule/crds/capsuleconfiguration-crd.yaml

# Run unit tests
unit: test-container fetch-external-crds
	docker run --privileged=true --mount type=bind,src=$(shell pwd),dst=/go/src/liqo -w /go/src/liqo --rm liqo-test

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
	rm --force deployments/liqo/files/*
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./internal/liqonet/route-operator" rbac:roleName=liqo-route output:rbac:stdout | awk -v RS="---\n" 'NR>1{f="./deployments/liqo/files/liqo-route-" $$4 ".yaml";printf "%s",$$0 > f; close(f)}' &&  sed -i -n '/rules/,$$p' deployments/liqo/files/liqo-route-ClusterRole.yaml deployments/liqo/files/liqo-route-Role.yaml
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./internal/liqonet/tunnel-operator" rbac:roleName=liqo-gateway output:rbac:stdout | awk -v RS="---\n" 'NR>1{f="./deployments/liqo/files/liqo-gateway-" $$4 ".yaml";printf "%s",$$0 > f; close(f)}' &&  sed -i -n '/rules/,$$p' deployments/liqo/files/liqo-gateway-ClusterRole.yaml deployments/liqo/files/liqo-gateway-Role.yaml
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./internal/liqonet/tunnelEndpointCreator" rbac:roleName=liqo-network-manager output:rbac:stdout | awk -v RS="---\n" 'NR>1{f="./deployments/liqo/files/liqo-network-manager-" $$4 ".yaml";printf "%s",$$0 > f; close(f)}' &&  sed -i -n '/rules/,$$p' deployments/liqo/files/liqo-network-manager-ClusterRole.yaml deployments/liqo/files/liqo-network-manager-Role.yaml
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./internal/crdReplicator" rbac:roleName=liqo-crd-replicator output:rbac:stdout | awk -v RS="---\n" 'NR>1{f="./deployments/liqo/files/liqo-crd-replicator-" $$4 ".yaml";printf "%s",$$0 > f; close(f)}' &&  sed -i -n '/rules/,$$p' deployments/liqo/files/liqo-crd-replicator-ClusterRole.yaml deployments/liqo/files/liqo-crd-replicator-Role.yaml
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./internal/discovery/foreign-cluster-operator" rbac:roleName=liqo-discovery output:rbac:stdout | awk -v RS="---\n" 'NR>1{f="./deployments/liqo/files/liqo-discovery-" $$4 ".yaml";printf "%s",$$0 > f; close(f)}' &&  sed -i -n '/rules/,$$p' deployments/liqo/files/liqo-discovery-ClusterRole.yaml deployments/liqo/files/liqo-discovery-Role.yaml
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./internal/auth-service" rbac:roleName=liqo-auth-service output:rbac:stdout | awk -v RS="---\n" 'NR>1{f="./deployments/liqo/files/liqo-auth-" $$4 ".yaml";printf "%s",$$0 > f; close(f)}' &&  sed -i -n '/rules/,$$p' deployments/liqo/files/liqo-auth-ClusterRole.yaml deployments/liqo/files/liqo-auth-Role.yaml
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./pkg/mutate" rbac:roleName=liqo-webhook output:rbac:stdout | awk -v RS="---\n" 'NR>1{f="./deployments/liqo/files/liqo-webhook-" $$4 ".yaml";printf "%s",$$0 > f; close(f)}' &&  sed -i -n '/rules/,$$p' deployments/liqo/files/liqo-webhook-ClusterRole.yaml deployments/liqo/files/liqo-webhook-Role.yaml
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./pkg/peering-roles/basic" rbac:roleName=liqo-remote-peering-basic output:rbac:stdout | awk -v RS="---\n" 'NR>1{f="./deployments/liqo/files/liqo-remote-peering-basic-" $$4 ".yaml";printf "%s",$$0 > f; close(f)}' &&  sed -i -n '/rules/,$$p' deployments/liqo/files/liqo-remote-peering-basic-ClusterRole.yaml
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./pkg/peering-roles/incoming" rbac:roleName=liqo-remote-peering-incoming output:rbac:stdout | awk -v RS="---\n" 'NR>1{f="./deployments/liqo/files/liqo-remote-peering-incoming-" $$4 ".yaml";printf "%s",$$0 > f; close(f)}' &&  sed -i -n '/rules/,$$p' deployments/liqo/files/liqo-remote-peering-incoming-ClusterRole.yaml
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./pkg/peering-roles/outgoing" rbac:roleName=liqo-remote-peering-outgoing output:rbac:stdout | awk -v RS="---\n" 'NR>1{f="./deployments/liqo/files/liqo-remote-peering-outgoing-" $$4 ".yaml";printf "%s",$$0 > f; close(f)}' &&  sed -i -n '/rules/,$$p' deployments/liqo/files/liqo-remote-peering-outgoing-ClusterRole.yaml
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./pkg/liqo-controller-manager/..." rbac:roleName=liqo-controller-manager output:rbac:stdout | awk -v RS="---\n" 'NR>1{f="./deployments/liqo/files/liqo-controller-manager-" $$4 ".yaml";printf "%s",$$0 > f; close(f)}' &&  sed -i -n '/rules/,$$p' deployments/liqo/files/liqo-controller-manager-ClusterRole.yaml deployments/liqo/files/liqo-controller-manager-Role.yaml
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./pkg/virtualKubelet/roles/local" rbac:roleName=liqo-virtual-kubelet-local output:rbac:stdout | awk -v RS="---\n" 'NR>1{f="./deployments/liqo/files/liqo-virtual-kubelet-local-" $$4 ".yaml";printf "%s",$$0 > f; close(f)}' &&  sed -i -n '/rules/,$$p' deployments/liqo/files/liqo-virtual-kubelet-local-ClusterRole.yaml
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./pkg/virtualKubelet/roles/remote" rbac:roleName=liqo-virtual-kubelet-remote output:rbac:stdout | awk -v RS="---\n" 'NR>1{f="./deployments/liqo/files/liqo-virtual-kubelet-remote-" $$4 ".yaml";printf "%s",$$0 > f; close(f)}' &&  sed -i -n '/rules/,$$p' deployments/liqo/files/liqo-virtual-kubelet-remote-ClusterRole.yaml
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./cmd/uninstaller" rbac:roleName=liqo-pre-delete output:rbac:stdout | awk -v RS="---\n" 'NR>1{f="./deployments/liqo/files/liqo-pre-delete-" $$4 ".yaml";printf "%s",$$0 > f; close(f)}' &&  sed -i -n '/rules/,$$p' deployments/liqo/files/liqo-pre-delete-ClusterRole.yaml

# Install gci if not available
gci:
ifeq (, $(shell which gci))
	@{ \
	go get github.com/daixiang0/gci@v0.2.9 ;\
	}
endif

# Run go fmt against code
fmt: gci
	go mod tidy
	go fmt ./...
	find $(pwd) -type f -name '*.go' -a ! -name '*zz_generated*' -exec gci -local github.com/liqotech/liqo -w {} \;

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Generate gRPC files
grpc: protoc
	$(PROTOC) --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative pkg/liqonet/ipam/ipam.proto

protoc:
ifeq (, $(shell which protoc))
	@{ \
	PB_REL="https://github.com/protocolbuffers/protobuf/releases" ;\
	version=3.15.5 ;\
	arch=x86_64 ;\
	curl -LO $${PB_REL}/download/v$${version}/protoc-$${version}-linux-$${arch}.zip ;\
	unzip protoc-$${version}-linux-$${arch}.zip -d $${HOME}/.local ;\
	rm protoc-$${version}-linux-$${arch}.zip ;\
	PROTOC_TMP_DIR=$$(mktemp -d) ;\
	cd $$PROTOC_TMP_DIR ;\
	go mod init tmp ;\
	go get google.golang.org/protobuf/cmd/protoc-gen-go ;\
	go get google.golang.org/grpc/cmd/protoc-gen-go-grpc ;\
	rm -rf $$PROTOC_TMP_DIR ;\
	}
endif
PROTOC=$(shell which protoc)

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

helm-docs:
ifeq (, $(shell which helm-docs))
	@{ \
	set -e ;\
	HELM_DOCS_TMP_DIR=$$(mktemp -d) ;\
	cd $$HELM_DOCS_TMP_DIR ;\
	version=1.5.0 ;\
    arch=x86_64 ;\
    echo  $$HELM_DOCS_PATH ;\
    echo https://github.com/norwoodj/helm-docs/releases/download/v$${version}/helm-docs_$${version}_linux_$${arch}.tar.gz ;\
    curl -LO https://github.com/norwoodj/helm-docs/releases/download/v$${version}/helm-docs_$${version}_linux_$${arch}.tar.gz ;\
    tar -zxvf helm-docs_$${version}_linux_$${arch}.tar.gz ;\
    mv helm-docs $(GOBIN)/helm-docs ;\
	rm -rf $$HELM_DOCS_TMP_DIR ;\
	}
HELM_DOCS=$(GOBIN)/helm-docs
else
HELM_DOCS=$(shell which helm-docs)
endif
