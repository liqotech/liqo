#!/usr/bin/env bash
# Runs the full Liqo E2E test suite on a local kind environment.
# Usage: ./hack/run-e2e-kind.sh [cruise|postinstall|postuninstall|all]
#   cruise        - run only the cruise tests (default)
#   postinstall   - run only the postinstall tests
#   postuninstall - run only the postuninstall tests
#   all           - run the full E2E pipeline (infra setup + all tests + teardown)

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# ── Configurable variables (override via env) ────────────────────────────────
CLUSTER_NUMBER="${CLUSTER_NUMBER:-3}"
K8S_VERSION="${K8S_VERSION:-v1.29.2}"
INFRA="${INFRA:-kind}"
CNI="${CNI:-cilium}"
NAMESPACE="${NAMESPACE:-liqo}"
LIQO_VERSION="${LIQO_VERSION:-$(git -C "${REPO_ROOT}" rev-parse HEAD)}"
POD_CIDR_OVERLAPPING="${POD_CIDR_OVERLAPPING:-false}"
TEMPLATE_FILE="${TEMPLATE_FILE:-cluster-templates.yaml.tmpl}"

TMPDIR="${TMPDIR:-$(mktemp -d)}"
BINDIR="${BINDIR:-${TMPDIR}/bin}"
KUBECONFIGDIR="${KUBECONFIGDIR:-${TMPDIR}/kubeconfigs}"

# ── Use locally installed tools if available, otherwise fall back to BINDIR ──
# The pipeline scripts check `command -v $KUBECTL` etc. before downloading,
# so pointing these to the system binaries skips the download entirely.
_resolve_tool() {
    local name=$1
    local fallback="${BINDIR}/${name}"
    command -v "${name}" 2>/dev/null || echo "${fallback}"
}

export CLUSTER_NUMBER K8S_VERSION INFRA CNI NAMESPACE LIQO_VERSION
export POD_CIDR_OVERLAPPING TEMPLATE_FILE TMPDIR BINDIR KUBECONFIGDIR
export KUBECTL="${KUBECTL:-$(_resolve_tool kubectl)}"
export HELM="${HELM:-$(_resolve_tool helm)}"
export LIQOCTL="${LIQOCTL:-$(_resolve_tool liqoctl)}"
# kind is less commonly installed globally; keep it in BINDIR by default
export KIND="${KIND:-$(_resolve_tool kind)}"
export TEMPLATE_DIR="${REPO_ROOT}/test/e2e/pipeline/infra/kind"
export PWD="${REPO_ROOT}"

MODE="${1:-cruise}"

echo "==> REPO_ROOT:            ${REPO_ROOT}"
echo "==> CLUSTER_NUMBER:       ${CLUSTER_NUMBER}"
echo "==> K8S_VERSION:          ${K8S_VERSION}"
echo "==> INFRA:                ${INFRA}"
echo "==> CNI:                  ${CNI}"
echo "==> LIQO_VERSION:         ${LIQO_VERSION}"
echo "==> TMPDIR:               ${TMPDIR}"
echo "==> BINDIR:               ${BINDIR}"
echo "==> KUBECONFIGDIR:        ${KUBECONFIGDIR}"
echo "==> MODE:                 ${MODE}"
echo ""

mkdir -p "${BINDIR}" "${KUBECONFIGDIR}"

run_infra_setup() {
    echo "==> Setting up kind infrastructure..."
    "${REPO_ROOT}/test/e2e/pipeline/infra/${INFRA}/pre-requirements.sh"
    "${REPO_ROOT}/test/e2e/pipeline/infra/${INFRA}/clean.sh"
    "${REPO_ROOT}/test/e2e/pipeline/infra/${INFRA}/setup.sh"

    # On macOS, Docker Desktop runs in a Linux VM so the kind bridge network
    # (e.g. 192.168.97.0/24) is not reachable from the host. kind already
    # maps each control-plane's 6443 to a random host port on 127.0.0.1.
    # Patch the kubeconfigs to use those host-mapped addresses so that
    # liqoctl (running on the host) can reach the remote API servers.
    if [[ "$(uname)" == "Darwin" ]]; then
        echo "==> Patching kubeconfigs for macOS (replacing internal Docker IPs with host-mapped ports)..."
        for i in $(seq 1 "${CLUSTER_NUMBER}"); do
            local kubeconfig="${KUBECONFIGDIR}/liqo_kubeconf_${i}"
            local container="cluster${i}-control-plane"
            # Get the host port that Docker maps to container port 6443
            local host_port
            host_port=$(docker inspect "${container}" \
                --format '{{(index (index .NetworkSettings.Ports "6443/tcp") 0).HostPort}}')
            echo "    cluster${i}: ${container} -> 127.0.0.1:${host_port}"
            # Replace the internal IP:6443 with 127.0.0.1:<host_port>
            local internal_ip
            internal_ip=$(docker inspect "${container}" \
                --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}')
            sed -i.bak "s|https://${internal_ip}:6443|https://127.0.0.1:${host_port}|g" "${kubeconfig}"
            rm -f "${kubeconfig}.bak"
        done
    fi
}

run_liqo_install() {
    echo "==> Installing Liqo..."
    "${REPO_ROOT}/test/e2e/pipeline/installer/kyverno/install.sh"
    "${REPO_ROOT}/test/e2e/pipeline/installer/liqo/setup.sh"
    "${REPO_ROOT}/test/e2e/pipeline/installer/liqo/peer.sh"
}

run_liqo_uninstall() {
    echo "==> Uninstalling Liqo..."
    "${REPO_ROOT}/test/e2e/pipeline/installer/liqo/unpeer.sh"
    "${REPO_ROOT}/test/e2e/pipeline/installer/liqo/uninstall.sh"
}

run_tests() {
    local suite="$1"
    echo "==> Running E2E tests: ${suite}..."
    go test "${REPO_ROOT}/test/e2e/${suite}/..." -count=1 -timeout=30m -p=1 -v
}

case "${MODE}" in
cruise)
    run_infra_setup
    run_liqo_install
    run_tests "cruise"
    run_liqo_uninstall
    ;;
postinstall)
    run_infra_setup
    run_liqo_install
    run_tests "postinstall"
    run_liqo_uninstall
    ;;
postuninstall)
    run_infra_setup
    run_liqo_install
    run_liqo_uninstall
    run_tests "postuninstall"
    ;;
all)
    run_infra_setup
    run_liqo_install
    run_tests "postinstall"
    run_tests "cruise"
    run_liqo_uninstall
    run_tests "postuninstall"
    ;;
*)
    echo "Unknown mode: ${MODE}. Use: cruise | postinstall | postuninstall | all" >&2
    exit 1
    ;;
esac

echo ""
echo "==> Done."
