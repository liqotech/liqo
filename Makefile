
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif


gen: generate fmt vet manifests rbacs docs

#generate helm documentation
docs: helm-docs
	$(HELM_DOCS) -t deployments/liqo/README.gotmpl deployments/liqo

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
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./internal/liqonet/route-operator" rbac:roleName=liqo-route output:rbac:stdout | awk -v RS="---\n" 'NR>1{f="./deployments/liqo/files/liqo-route-" $$4 ".yaml";printf "%s",$$0 > f; close(f)}' &&  sed -i -n '/rules/,$$p' deployments/liqo/files/liqo-route-ClusterRole.yaml deployments/liqo/files/liqo-route-Role.yaml
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./internal/liqonet/tunnel-operator" rbac:roleName=liqo-gateway output:rbac:stdout | awk -v RS="---\n" 'NR>1{f="./deployments/liqo/files/liqo-gateway-" $$4 ".yaml";printf "%s",$$0 > f; close(f)}' &&  sed -i -n '/rules/,$$p' deployments/liqo/files/liqo-gateway-ClusterRole.yaml deployments/liqo/files/liqo-gateway-Role.yaml
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./internal/liqonet/tunnelEndpointCreator" rbac:roleName=liqo-network-manager output:rbac:stdout | awk -v RS="---\n" 'NR>1{f="./deployments/liqo/files/liqo-network-manager-" $$4 ".yaml";printf "%s",$$0 > f; close(f)}' &&  sed -i -n '/rules/,$$p' deployments/liqo/files/liqo-network-manager-ClusterRole.yaml deployments/liqo/files/liqo-network-manager-Role.yaml
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./internal/crdReplicator" rbac:roleName=liqo-crd-replicator output:rbac:stdout | awk -v RS="---\n" 'NR>1{f="./deployments/liqo/files/liqo-crd-replicator-" $$4 ".yaml";printf "%s",$$0 > f; close(f)}' &&  sed -i -n '/rules/,$$p' deployments/liqo/files/liqo-crd-replicator-ClusterRole.yaml deployments/liqo/files/liqo-crd-replicator-Role.yaml
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./internal/peering-request-operator" rbac:roleName=liqo-peering-request output:rbac:stdout | awk -v RS="---\n" 'NR>1{f="./deployments/liqo/files/liqo-peering-request-" $$4 ".yaml";printf "%s",$$0 > f; close(f)}' &&  sed -i -n '/rules/,$$p' deployments/liqo/files/liqo-peering-request-ClusterRole.yaml deployments/liqo/files/liqo-peering-request-Role.yaml
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./internal/discovery/foreign-cluster-operator" rbac:roleName=liqo-discovery output:rbac:stdout | awk -v RS="---\n" 'NR>1{f="./deployments/liqo/files/liqo-discovery-" $$4 ".yaml";printf "%s",$$0 > f; close(f)}' &&  sed -i -n '/rules/,$$p' deployments/liqo/files/liqo-discovery-ClusterRole.yaml deployments/liqo/files/liqo-discovery-Role.yaml
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./internal/auth-service" rbac:roleName=liqo-auth-service output:rbac:stdout | awk -v RS="---\n" 'NR>1{f="./deployments/liqo/files/liqo-auth-" $$4 ".yaml";printf "%s",$$0 > f; close(f)}' &&  sed -i -n '/rules/,$$p' deployments/liqo/files/liqo-auth-ClusterRole.yaml deployments/liqo/files/liqo-auth-Role.yaml
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths="./pkg/mutate" rbac:roleName=liqo-webhook output:rbac:stdout | awk -v RS="---\n" 'NR>1{f="./deployments/liqo/files/liqo-webhook-" $$4 ".yaml";printf "%s",$$0 > f; close(f)}' &&  sed -i -n '/rules/,$$p' deployments/liqo/files/liqo-webhook-ClusterRole.yaml deployments/liqo/files/liqo-webhook-Role.yaml

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