#!/usr/bin/env bash
#shellcheck disable=SC1091

FILEPATH=$(realpath "$0")
WORKDIR=$(dirname "$FILEPATH")

# shellcheck disable=SC1091
# shellcheck source=./pre-requirements.sh
source "$WORKDIR/pre-requirements.sh"

# shellcheck disable=SC1091
# shellcheck source=../../utils.sh
source "$WORKDIR/../../utils.sh"

DOCKER_PROXY="${DOCKER_PROXY:-docker.io}"

function install_calico() {
  local kubeconfig=$1
  local calico_version="v3.32.1"

  "${KUBECTL}" create -f "https://raw.githubusercontent.com/projectcalico/calico/${calico_version}/manifests/operator-crds.yaml" --kubeconfig "$kubeconfig"
  "${KUBECTL}" create -f "https://raw.githubusercontent.com/projectcalico/calico/${calico_version}/manifests/tigera-operator.yaml" --kubeconfig "$kubeconfig"

  # Wait for the Installation CRD to be available
  if ! waitandretry 5s 12 "${KUBECTL} get crd installations.operator.tigera.io --kubeconfig $kubeconfig"; then
    echo "Failed to wait for Calico Installation CRD to be available"
    exit 1
  fi

  # append a slash to DOCKER_PROXY if not present
  if [[ "${DOCKER_PROXY}" != */ ]]; then
    registry="${DOCKER_PROXY}/"
  else
    registry="${DOCKER_PROXY}"
  fi

  cat <<EOF >custom-resources.yaml
# This section includes base Calico installation configuration.
# For more information, see: https://projectcalico.docs.tigera.io/master/reference/installation/api#operator.tigera.io/v1.Installation
apiVersion: operator.tigera.io/v1
kind: Installation
metadata:
  name: default
spec:
  registry: $registry
  # Configures Calico networking.
  calicoNetwork:
    # Note: The ipPools section cannot be modified post-install.
    ipPools:
    - blockSize: 26
      cidr: $POD_CIDR
      encapsulation: VXLAN
      natOutgoing: Enabled
      nodeSelector: all()
    nodeAddressAutodetectionV4:
      skipInterface: liqo.*

---

# This section configures the Calico API server.
# For more information, see: https://projectcalico.docs.tigera.io/master/reference/installation/api#operator.tigera.io/v1.APIServer
apiVersion: operator.tigera.io/v1
kind: APIServer
metadata:
  name: default
spec: {}
EOF
  "${KUBECTL}" apply -f custom-resources.yaml --kubeconfig "$kubeconfig"
}

function wait_calico() {
  local kubeconfig=$1
  if ! waitandretry 5s 12 "${KUBECTL} wait --for condition=Ready=true -n calico-system pod --all --kubeconfig $kubeconfig --timeout=-1s"; then
    echo "Failed to wait for calico pods to be ready"
    exit 1
  fi
  # set felix to use different port for VXLAN
  if ! waitandretry 5s 12 "${KUBECTL} patch felixconfiguration default --type=merge -p {\"spec\":{\"vxlanPort\":6789}} --kubeconfig $kubeconfig"; then
    echo "Failed to patch felixconfiguration"
    exit 1
  fi
}

function install_cilium() {
  local kubeconfig=$1

  if [ ! -f "${BINDIR}/cilium" ]; then
    setup_arch_and_os
    local CILIUM_CLI_VERSION
    CILIUM_CLI_VERSION="v0.18.8"

    echo "Downloading Cilium CLI ${CILIUM_CLI_VERSION} for ${OS}-${ARCH}"
    curl -L --remote-name-all "https://github.com/cilium/cilium-cli/releases/download/${CILIUM_CLI_VERSION}/cilium-${OS}-${ARCH}.tar.gz{,.sha256sum}"
    sha256sum --check "cilium-${OS}-${ARCH}.tar.gz.sha256sum"
    tar -C "${BINDIR}" -xzvf "cilium-${OS}-${ARCH}.tar.gz"
    rm "cilium-${OS}-${ARCH}.tar.gz"
    rm "cilium-${OS}-${ARCH}.tar.gz.sha256sum"
  fi

  cat <<EOF >cilium-values.yaml
MTU: 1300
devices: "eth0"
ipam:
  operator:
    clusterPoolIPv4PodCIDRList: ${POD_CIDR}
routingMode: tunnel
tunnelProtocol: vxlan
tunnelPort: 8473

affinity:
  nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: liqo.io/type
            operator: DoesNotExist

EOF

  KUBECONFIG="$kubeconfig" "${BINDIR}/cilium" install --values "cilium-values.yaml"
}

function wait_cilium() {
  local kubeconfig=$1
  KUBECONFIG="$kubeconfig" "${BINDIR}/cilium" status --wait
}

function install_flannel() {
  local kubeconfig=$1
  "${KUBECTL}" create ns kube-flannel --kubeconfig "$kubeconfig"
  "${KUBECTL}" label --overwrite ns kube-flannel pod-security.kubernetes.io/enforce=privileged --kubeconfig "$kubeconfig"
  "${HELM}" repo add flannel https://flannel-io.github.io/flannel/
  "${HELM}" install flannel --set podCidr="${POD_CIDR}",flannel.mtu=1400,flannel.backendPort=8473 --namespace kube-flannel flannel/flannel --kubeconfig "$kubeconfig"
}

function wait_flannel() {
  local kubeconfig=$1
  if ! waitandretry 5s 12 "${KUBECTL} wait --for condition=Ready=true -n kube-flannel pod --all --timeout=-1s --kubeconfig $kubeconfig"; then
    echo "Failed to wait for flannel pods to be ready"
    exit 1
  fi
}

# CNI_CHECK_NAMESPACE is the namespace hosting the pods used to check the CNI datapath.
CNI_CHECK_NAMESPACE="liqo-cni-check"
# CNI_CHECK_IMAGE is the image used by the pods checking the CNI datapath. It only needs a shell,
# an HTTP server and an HTTP client, so keep it as small as possible to avoid slowing down the pull.
CNI_CHECK_IMAGE="${CNI_CHECK_IMAGE:-${DOCKER_PROXY}/library/busybox:1.36}"

# cni_probe_once issues a single HTTP request from the given pod to the given address.
# It is a standalone function as waitandretry runs its command unquoted.
function cni_probe_once() {
  local kubeconfig=$1
  local pod=$2
  local address=$3

  "${KUBECTL}" exec --kubeconfig "$kubeconfig" -n "${CNI_CHECK_NAMESPACE}" "$pod" -- \
    wget -q -O /dev/null -T 3 "http://${address}:8080/"
}

# check_cni_datapath checks that two pods scheduled on different nodes can reach each other.
#
# A CNI reports its own pods as ready before the routes towards the other nodes are programmed:
# Calico, for instance, has to allocate a block for every node and propagate it to the others.
# Installing Liqo and peering on top of a partially configured datapath makes the network fabric
# sample an incomplete routing table, and the resulting failures do not surface here: they show up
# much later, as unreachable pods in the e2e network tests, and they do not recover on their own.
function check_cni_datapath() {
  local kubeconfig=$1

  if ! "${KUBECTL}" wait --kubeconfig "$kubeconfig" --for=condition=Ready nodes --all --timeout=300s; then
    echo "Failed to wait for the nodes to be ready"
    exit 1
  fi

  local workers
  read -r -a workers <<< "$("${KUBECTL}" get nodes --kubeconfig "$kubeconfig" \
    --selector '!node-role.kubernetes.io/control-plane' -o jsonpath='{.items[*].metadata.name}')"

  if [[ ${#workers[@]} -lt 2 ]]; then
    echo "Skipping the CNI datapath check: it requires at least two worker nodes, found ${#workers[@]}"
    return 0
  fi

  echo "Checking the CNI datapath between nodes ${workers[0]} and ${workers[1]}"

  # The pods are pinned with nodeName to check the datapath between two given nodes without
  # depending on the scheduler.
  cat <<EOF | "${KUBECTL}" apply --kubeconfig "$kubeconfig" -f -
apiVersion: v1
kind: Namespace
metadata:
  name: ${CNI_CHECK_NAMESPACE}
  labels:
    pod-security.kubernetes.io/enforce: privileged
---
apiVersion: v1
kind: Pod
metadata:
  name: cni-check-a
  namespace: ${CNI_CHECK_NAMESPACE}
spec:
  nodeName: ${workers[0]}
  containers:
  - name: probe
    image: ${CNI_CHECK_IMAGE}
    command: ["/bin/sh", "-c"]
    args: ["mkdir -p /www && echo ok > /www/index.html && exec httpd -f -p 8080 -h /www"]
    readinessProbe:
      tcpSocket:
        port: 8080
---
apiVersion: v1
kind: Pod
metadata:
  name: cni-check-b
  namespace: ${CNI_CHECK_NAMESPACE}
spec:
  nodeName: ${workers[1]}
  containers:
  - name: probe
    image: ${CNI_CHECK_IMAGE}
    command: ["/bin/sh", "-c"]
    args: ["mkdir -p /www && echo ok > /www/index.html && exec httpd -f -p 8080 -h /www"]
    readinessProbe:
      tcpSocket:
        port: 8080
EOF

  if ! "${KUBECTL}" wait --kubeconfig "$kubeconfig" -n "${CNI_CHECK_NAMESPACE}" \
    --for=condition=Ready pod --all --timeout=300s; then
    echo "Failed to wait for the CNI check pods to be ready"
    exit 1
  fi

  local ip_a ip_b
  ip_a=$("${KUBECTL}" get pod cni-check-a --kubeconfig "$kubeconfig" \
    -n "${CNI_CHECK_NAMESPACE}" -o jsonpath='{.status.podIP}')
  ip_b=$("${KUBECTL}" get pod cni-check-b --kubeconfig "$kubeconfig" \
    -n "${CNI_CHECK_NAMESPACE}" -o jsonpath='{.status.podIP}')

  # Check both directions, as the routes of the two nodes are programmed independently.
  if ! waitandretry 2s 60 "cni_probe_once $kubeconfig cni-check-a $ip_b"; then
    echo "Failed to reach ${ip_b} on ${workers[1]} from ${workers[0]}: the CNI datapath is not ready"
    exit 1
  fi
  if ! waitandretry 2s 60 "cni_probe_once $kubeconfig cni-check-b $ip_a"; then
    echo "Failed to reach ${ip_a} on ${workers[0]} from ${workers[1]}: the CNI datapath is not ready"
    exit 1
  fi

  echo "The CNI datapath is ready"

  "${KUBECTL}" delete namespace "${CNI_CHECK_NAMESPACE}" --kubeconfig "$kubeconfig" \
    --wait=false --ignore-not-found
}
