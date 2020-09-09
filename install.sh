#!/usr/bin/env bash
set -e           # Fail in case of error
set -o nounset   # Fail if undefined variables are used
set -o pipefail  # Fail if one of the piped commands fails

#
# Usage:
#   curl ... | ENV_VAR=... bash
#       or
#   ENV_VAR=... ./install.sh
#
# Example:
#   Installing Liqo configuring the cluster name:
#     curl ... | CLUSTER_NAME="MyAwesomeCluster" bash
#   Uninstalling and purging Liqo:
#     curl ... | bash -s -- --uninstall --purge
#
# Arguments:
#   - --uninstall:        uninstall Liqo from your cluster
#   - --purge:            purge all Liqo components from your cluster (i.e. including CRDs)
#
# Environment variables:
#
#   - LIQO_VERSION
#     the version of Liqo to install. It can be a released version, a commit SHA or 'master'.
#     Defaults to the latest released version.
#
#   - LIQO_NAMESPACE
#     the Kubernetes namespace where all Liqo control plane components are created (defaults to liqo).
#   - CLUSTER_NAME
#     the mnemonic name assigned to this Liqo instance. Automatically generated if not specified.
#   - DASHBOARD_HOSTNAME
#     the hostname assigned to the Liqo dashboard (exposed through an Ingress resource).
#
#   - POD_CIDR
#     the Pod CIDR of your cluster (e.g.; 10.0.0.0/16). Automatically detected if not configured.
#   - SERVICE_CIDR
#     the Service CIDR of your cluster (e.g.; 10.96.0.0/12). Automatically detected if not configured.
#
#   - KUBECONFIG
#     the KUBECONFIG file used to interact with the cluster (defaults to ~/.kube/config).
#   - KUBECONFIG_CONTEXT
#     the context selected to interact with the cluster (defaults to the current one).
#
#   - AGENT_INSTALL
#     setting this variable to 'true' will trigger also the installer of the LIQO Desktop Agent"
EXIT_SUCCESS=0
EXIT_FAILURE=1

LIQO_REPO="liqotech/liqo"
LIQO_CHARTS_PATH="deployments/liqo"

LIQO_DASHBOARD_REPO="liqotech/dashboard"

LIQO_NAMESPACE_DEFAULT="liqo"
CLUSTER_NAME_DEFAULT=$(printf "LiqoCluster%04d" $(( RANDOM%10000 )) )

function setup_colors() {
	# Only use colors if connected to a terminal
	if [ -t 1 ]; then
		RED=$(printf '\033[31m')
		GREEN=$(printf '\033[32m')
		YELLOW=$(printf '\033[33m')
		BLUE=$(printf '\033[34m')
		BOLD=$(printf '\033[1m')
		RESET=$(printf '\033[m')
	else
		RED=""
		GREEN=""
		YELLOW=""
		BLUE=""
		BOLD=""
		RESET=""
	fi
}

function print_logo() {
	# ASCII Art: https://patorjk.com/software/taag/#p=display&f=Big%20Money-ne&t=Liqo
	echo -n "${BLUE}${BOLD}"
	cat <<-'EOF'


	     /$$       /$$
	    | $$      |__/
	    | $$       /$$  /$$$$$$   /$$$$$$
	    | $$      | $$ /$$__  $$ /$$__  $$
	    | $$      | $$| $$  \ $$| $$  \ $$
	    | $$      | $$| $$  | $$| $$  | $$
	    | $$$$$$$$| $$|  $$$$$$$|  $$$$$$/
	    |________/|__/ \____  $$ \______/
	                        | $$
	                        | $$
	                        |__/


	EOF
	echo -n "${RESET}"
}

function info() {
	echo "${GREEN}${BOLD}$1${RESET} ${*:2}"
}
function warn() {
	echo "${YELLOW}${BOLD}$1${RESET} ${*:2}" >&2
}
function fatal() {
	echo "${RED}${BOLD}$1 [FATAL]${RESET} ${*:2}" >&2
	exit ${EXIT_FAILURE}
}


function help() {
	cat <<-EOF
	${BLUE}${BOLD}Install Liqo on your Kubernetes cluster${RESET}
	  ${BOLD}Usage: $0 [options]

	${BLUE}${BOLD}Options:${RESET}
	  ${BOLD}--uninstall${RESET}:        uninstall Liqo from your cluster
	  ${BOLD}--purge${RESET}:            purge all Liqo components from your cluster (i.e. including CRDs)

	  ${BOLD}-h, --help${RESET}:         display this help

	${BLUE}${BOLD}Environment variables:${RESET}
	  ${BOLD}LIQO_VERSION${RESET}:       the version of Liqo to install. It can be a released version, a commit SHA or 'master'.

	  ${BOLD}LIQO_NAMESPACE${RESET}:     the Kubernetes namespace where all Liqo components are created (defaults to liqo)
	  ${BOLD}CLUSTER_NAME${RESET}:       the mnemonic name assigned to this Liqo instance. Automatically generated if not specified.
	  ${BOLD}DASHBOARD_HOSTNAME${RESET}: the hostname assigned to the Liqo dashboard (exposed through an Ingress resource).

	  ${BOLD}POD_CIDR${RESET}:           the Pod CIDR of your cluster (e.g.; 10.0.0.0/16). Automatically detected if not configured.
	  ${BOLD}SERVICE_CIDR${RESET}:       the Service CIDR of your cluster (e.g.; 10.96.0.0/12). Automatically detected if not configured.

	  ${BOLD}KUBECONFIG${RESET}:         the KUBECONFIG file used to interact with the cluster (defaults to ~/.kube/config).
	  ${BOLD}KUBECONFIG_CONTEXT${RESET}: the context selected to interact with the cluster (defaults to the current one).

	  ${BOLD}AGENT_INSTALL${RESET}:      set this variable to 'true' to trigger the installer of Liqo Desktop Agent

	EOF
}

function parse_arguments() {
	# Call getopt to validate the provided input.
	local ERROR_STR="${RED}${BOLD}[PRE-FLIGHT] [FATAL]${RESET}"
	OPTIONS=$(getopt --options h --longoptions help,uninstall,purge --name "${ERROR_STR}" -- "$@") ||
		exit ${EXIT_FAILURE}

	INSTALL_LIQO=true
	PURGE_LIQO=false

	eval set -- "$OPTIONS"
	unset OPTIONS

	while true; do
		case "$1" in
		--help|-h)
			help; exit ${EXIT_SUCCESS} ;;

		--uninstall)
			INSTALL_LIQO=false ;;
		--purge)
			INSTALL_LIQO=false; PURGE_LIQO=true; ;;

		--)
			shift; break; ;;
		esac
		shift
	done

	[ $# -eq 0 ] || fatal "[PRE-FLIGHT]" "unrecognized argument '$1'"
}


function command_exists() {
	command -v "$1" >/dev/null 2>&1
}


function setup_downloader() {
	if command_exists "curl"; then
		DOWNLOADER="curl"
	elif command_exists "wget"; then
		DOWNLOADER="wget"
	else
		fatal "[PRE-FLIGHT] [DOWNLOAD]" "Cannot find neither 'curl' nor 'wget' to download files"
	fi

	info "[PRE-FLIGHT] [DOWNLOAD]" "Using ${DOWNLOADER} to download files"
}

function get_repo_tags() {
	[ $# -eq 1 ] || fatal "[PRE-FLIGHT] [DOWNLOAD]" "Internal error: incorrect parameters"
	# The maximum number of retrieved tags is 100, but this should not raise concerns for a while
	local TAGS_URL="https://api.github.com/repos/$1/tags?page=1&per_page=100"
	download "${TAGS_URL}" | grep -Po '"name": "\K.*?(?=")' || echo ""
}

function get_repo_master_commit() {
	[ $# -eq 1 ] || fatal "[PRE-FLIGHT] [DOWNLOAD]" "Internal error: incorrect parameters"
	# The maximum number of retrieved tags is 100, but this should not raise concerns for a while
	local MASTER_COMMIT_URL="https://api.github.com/repos/$1/commits?page=1&per_page=1"
	download "${MASTER_COMMIT_URL}" | grep -Po --max-count=1 '"sha": "\K.*?(?=")'
}

function setup_liqo_version() {
	# A specific commit has been requested: assuming development version and returning
	if [[ "${LIQO_VERSION:-}" =~ ^[0-9a-f]{40}$ ]]; then
		warn "[PRE-FLIGHT] [DOWNLOAD]" "A Liqo commit has been specified: using the development version"
		LIQO_IMAGE_VERSION=${LIQO_VERSION}
		LIQO_SUFFIX="-ci"

		# Using the Liqo Dashboard version from master
		warn "[PRE-FLIGHT] [DOWNLOAD]" "An unreleased version of Liqo Dashboard is going to be downloaded"
		LIQO_DASHBOARD_IMAGE_VERSION=$(get_repo_master_commit ${LIQO_DASHBOARD_REPO}) ||
			fatal "[PRE-FLIGHT] [DOWNLOAD]" "Failed to retrieve the latest commit of the master branch"
		return 0
	fi

	# Obtain the list of Liqo tags
	local LIQO_TAGS
	LIQO_TAGS=$(get_repo_tags ${LIQO_REPO})

	#Obtain the list of Liqo Dashboard tags
	local LIQO_DASHBOARD_TAGS
	LIQO_DASHBOARD_TAGS=$(get_repo_tags ${LIQO_DASHBOARD_REPO})

	# If no version has been specified, select the latest tag (if available)
	LIQO_VERSION=${LIQO_VERSION:-$(printf "%s" "${LIQO_TAGS}" | head --lines=1)}

	if [ "${LIQO_VERSION:=master}" != "master" ]; then
		# A specific version has been requested: check if the version exists
		printf "%s" "${LIQO_TAGS}" | grep -P --silent "^${LIQO_VERSION}$" ||
			fatal "[PRE-FLIGHT] [DOWNLOAD]" "The requested Liqo version '${LIQO_VERSION}' does not exist"
		LIQO_IMAGE_VERSION=${LIQO_VERSION}
		
		# Also check if the relative dashboard version exists
		printf "%s" "${LIQO_DASHBOARD_TAGS}" | grep -P --silent "^${LIQO_VERSION}$" ||
			fatal "[PRE-FLIGHT] [DOWNLOAD]" "The requested Liqo Dashboard version '${LIQO_VERSION}' does not exist"
		LIQO_DASHBOARD_IMAGE_VERSION=${LIQO_VERSION}

	else
		# Using the version from master
		warn "[PRE-FLIGHT] [DOWNLOAD]" "An unreleased version of Liqo is going to be downloaded"
		LIQO_IMAGE_VERSION=$(get_repo_master_commit ${LIQO_REPO}) ||
			fatal "[PRE-FLIGHT] [DOWNLOAD]" "Failed to retrieve the latest commit of the master branch"

		# Using the Liqo Dashboard version from master
		warn "[PRE-FLIGHT] [DOWNLOAD]" "An unreleased version of Liqo Dashboard is going to be downloaded"
		LIQO_DASHBOARD_IMAGE_VERSION=$(get_repo_master_commit ${LIQO_DASHBOARD_REPO}) ||
			fatal "[PRE-FLIGHT] [DOWNLOAD]" "Failed to retrieve the latest commit of the master branch"
	fi
}

function download() {
	[ $# -eq 1 ] || fatal "[PRE-FLIGHT] [DOWNLOAD]" "Internal error: incorrect parameters"

	case ${DOWNLOADER:-} in
		curl)
			curl --output - --silent --fail --location "$1" ||
				fatal "[PRE-FLIGHT] [DOWNLOAD]" "Failed downloading $1" ;;
		wget)
			wget --quiet --output-document=- "$1" ||
				fatal "[PRE-FLIGHT] [DOWNLOAD]" "Failed downloading $1" ;;
		*)
			fatal "[PRE-FLIGHT] [DOWNLOAD]" "Internal error: incorrect downloader" ;;
	esac
}

function download_helm() {
	local HELM_VERSION=v3.3.4
	local HELM_ARCHIVE=helm-${HELM_VERSION}-linux-amd64.tar.gz
	local HELM_URL=https://get.helm.sh/${HELM_ARCHIVE}

	info "[PRE-FLIGHT] [DOWNLOAD]" "Downloading Helm ${HELM_VERSION}"
	command_exists tar || fatal "[PRE-FLIGHT] [DOWNLOAD]" "'tar' is not available"
	download "${HELM_URL}" | tar zxf - --directory="${BINDIR}" --wildcards '*/helm' --strip 1 2>/dev/null ||
		fatal "[PRE-FLIGHT] [DOWNLOAD]" "Something went wrong while extracting the Helm archive"
	HELM="${BINDIR}/helm"
}

function download_liqo() {
	info "[PRE-FLIGHT] [DOWNLOAD]" "Downloading Liqo (version: ${LIQO_VERSION})"
	command_exists tar || fatal "[PRE-FLIGHT] [DOWNLOAD]" "'tar' is not available"
	local LIQO_DOWNLOAD_URL=https://github.com/liqotech/liqo/archive/${LIQO_VERSION}.tar.gz
	download "${LIQO_DOWNLOAD_URL}" | tar zxf - --directory="${TMPDIR}" --strip 1 2>/dev/null ||
		fatal "[PRE-FLIGHT] [DOWNLOAD]" "Something went wrong while extracting the Liqo archive"
}


function setup_kubectl() {
	command_exists "kubectl" ||
		fatal "[PRE-FLIGHT]" "Cannot find 'kubectl'"

	info "[PRE-FLIGHT] [KUBECTL]" "Kubectl correctly found"
	info "[PRE-FLIGHT] [KUBECTL]" "Using KUBECONFIG: ${KUBECONFIG:-~/.kube/config}"

	local CURRENT_CONTEXT
	CURRENT_CONTEXT=$(kubectl config current-context) ||
		fatal "[PRE-FLIGHT] [KUBECTL]" "Failed to retrieve the current context"
	info "[PRE-FLIGHT] [KUBECTL]" "Using context: ${KUBECONFIG_CONTEXT:=${CURRENT_CONTEXT}}"
	KUBECTL="kubectl --context ${KUBECONFIG_CONTEXT}"
}

function setup_tmpdir() {
	TMPDIR=$(mktemp -d -t liqo-install.XXXXXXXXXX)
	BINDIR="${TMPDIR}/bin"
	mkdir --parent "${BINDIR}"

	cleanup() {
		local CODE=$?

		# Do not trigger errors again if something goes wrong
		set +e
		trap - EXIT

		rm -rf "${TMPDIR}"
		exit ${CODE}
	}
	trap cleanup INT EXIT
}


function configure_namespace() {
	local PHASE
	PHASE=$([ "${INSTALL_LIQO}" = true ] && echo "INSTALL" || echo "UNINSTALL" )

	LIQO_NAMESPACE=${LIQO_NAMESPACE:-${LIQO_NAMESPACE_DEFAULT}}
	info "[${PHASE}] [CONFIGURE]" "Using namespace: ${LIQO_NAMESPACE}"
}

function configure_installation_variables() {
	CLUSTER_NAME=${CLUSTER_NAME:-${CLUSTER_NAME_DEFAULT}}
	info "[INSTALL] [CONFIGURE]" "Cluster name: ${CLUSTER_NAME}"

	# Attempt to retrieve the Pod CIDR (in case it has not been specified)
	if [ -z "${POD_CIDR:-}" ]; then
		POD_CIDR=$(
			${KUBECTL} get pods --namespace kube-system \
				--selector component=kube-controller-manager \
				--output jsonpath="{.items[*].spec.containers[*].command}" 2>/dev/null | \
					grep -Po --max-count=1 "(?<=--cluster-cidr=)[0-9.\/]+") ||
			fatal "[INSTALL] [CONFIGURE]" "Failed to automatically retrieve the Pod CIDR." \
				"Please, manually specify it with 'export POD_CIDR=...' before executing again this script"
	fi
	info "[INSTALL] [CONFIGURE]" "Pod CIDR: ${POD_CIDR}"

	# Attempt to retrieve the Service CIDR (in case it has not been specified)
	if [ -z "${SERVICE_CIDR:-}" ]; then
		SERVICE_CIDR=$(
			${KUBECTL} get pods --namespace kube-system \
				--selector component=kube-controller-manager \
				--output jsonpath="{.items[*].spec.containers[*].command} 2>dev/null" | \
					grep -Po --max-count=1 "(?<=--service-cluster-ip-range=)[0-9.\/]+") ||
			fatal "[INSTALL] [CONFIGURE]" "Failed to automatically retrieve the Service CIDR." \
				"Please, manually specify it with 'export SERVICE_CIDR=...' before executing again this script"
	fi
	info "[INSTALL] [CONFIGURE]" "Service CIDR: ${SERVICE_CIDR}"
}

function configure_gateway_node() {
	local GATEWAY_LABEL="net.liqo.io/gateway=true"

	# Check whether there is already a node labeled as gateway
	GATEWAY=$(${KUBECTL} get node --selector "${GATEWAY_LABEL}" --output jsonpath="{.items[*].metadata.name}" 2>/dev/null) ||
		fatal "[INSTALL] [CONFIGURE]" "Failed to detect whether a gateway node is already configured"

	if [ "$(wc --words <<< "${GATEWAY}")" -ge 2 ]; then
		fatal "[INSTALL] [CONFIGURE]" "More than one cluster node is labeled as gateway"
	fi

	if [ -z "${GATEWAY}" ]; then
		# If not, select one node as gateway and label it accordingly
		GATEWAY=$(${KUBECTL} get node --output jsonpath="{.items[-1].metadata.name}" 2>/dev/null) ||
			fatal "[INSTALL] [CONFIGURE]" "Failed to retrieve a candidate gateway node"
		${KUBECTL} label node "${GATEWAY}" "${GATEWAY_LABEL}" >/dev/null ||
			fatal "[INSTALL] [CONFIGURE]" "Failed to label node ${GATEWAY} as gateway"
	fi

	GATEWAY_IP=$(${KUBECTL} get node "${GATEWAY}" -o jsonpath="{.status.addresses[0].address}" 2>/dev/null) ||
		fatal "[INSTALL] [CONFIGURE]" "Failed to retrieve the IP address of the gateway node"
	info "[INSTALL] [CONFIGURE]" "Gateway node: ${GATEWAY} (${GATEWAY_IP})"
}

function install_liqo() {
	info "[INSTALL]" "Installing Liqo on your cluster..."

	# Ignore errors that may occur if the namespace already exists
	${KUBECTL} create namespace "${LIQO_NAMESPACE}" 1>/dev/null 2>&1 || true

	local LIQO_CHART="${TMPDIR}/${LIQO_CHARTS_PATH}"
	${HELM} dependency update "${LIQO_CHART}" >/dev/null ||
		fatal "[INSTALL]" "Something went wrong while installing Liqo"
	${HELM} install liqo --kube-context "${KUBECONFIG_CONTEXT}" --namespace "${LIQO_NAMESPACE}" "${LIQO_CHART}" \
		--set global.version="${LIQO_IMAGE_VERSION}" --set global.suffix="${LIQO_SUFFIX:-}" --set clusterName="${CLUSTER_NAME}" \
		--set podCIDR="${POD_CIDR}" --set serviceCIDR="${SERVICE_CIDR}" --set gatewayIP="${GATEWAY_IP}" \
		--set global.dashboard_version="${LIQO_DASHBOARD_IMAGE_VERSION}" \
		--set global.dashboard_ingress="${DASHBOARD_INGRESS:-}" >/dev/null ||
			fatal "[INSTALL]" "Something went wrong while installing Liqo"

	info "[INSTALL]" "Hooray! Liqo is now installed on your cluster"
	 if [ -n "${AGENT_INSTALL}" ] && [ "$AGENT_INSTALL" == "true" ] ; then
      agent_install
  fi
}

function all_clusters_unjoined() {
	local JSON_PATH="{.items[*].spec.join} {.items[*].status.incoming.joined} {.items[*].status.outgoing.joined} {.items[*].status.network.localNetworkConfig.available} {.items[*].status.network.remoteNetworkConfig.available} {.items[*].status.network.tunnelEndpoint.available}"
	( ${KUBECTL} get foreignclusters --output jsonpath="${JSON_PATH}" 2>/dev/null || echo "" ) | \
		grep --invert-match --silent "true"
}

function unjoin_clusters() {
	# Do not fail in case of errors, to avoid exiting if Liqo had already been (partially) uninstalled
	set +e

	info "[UNINSTALL] [UNJOIN]" "Unjoining from all peers"

	# Globally disable the broadcaster
	local CLUSTER_CONFIG_PATCH='{"spec":{"advertisementConfig":{"outgoingConfig":{"enableBroadcaster":false}}}}'
	${KUBECTL} patch clusterconfig configuration --patch "${CLUSTER_CONFIG_PATCH}" --type 'merge' >/dev/null 2>&1

	# Set join=false to all ForeignCluster resources
	FOREIGN_CLUSTERS=$(${KUBECTL} get foreignclusters --output jsonpath="{.items[*].metadata.name}") 2>/dev/null
	for FOREIGN_CLUSTER in ${FOREIGN_CLUSTERS}; do
		${KUBECTL} patch foreignclusters "${FOREIGN_CLUSTER}" --patch '{"spec":{"join":false}}' --type 'merge' >/dev/null 2>&1
	done

	info "[UNINSTALL] [UNJOIN]" "Waiting for the unjoining process to complete..."

	local RETRIES=600
	while ! all_clusters_unjoined; do
		RETRIES=$(( RETRIES-1 ))
		[ "${RETRIES}" -gt 0 ] ||
			fatal "[UNINSTALL] [UNJOIN]" "Timeout: impossible to unpeer from all clusters"
		sleep 1
	done

	info "[UNINSTALL] [UNJOIN]" "Waiting for the network operators to reconcile..."

	${KUBECTL} wait tunnelendpoints.net.liqo.io --timeout=30s --all --for=delete >/dev/null 2>&1
	${KUBECTL} wait networkconfigs.net.liqo.io --timeout=30s --all --for=delete >/dev/null 2>&1
	set -e
}

function uninstall_liqo() {
	# Do not fail in case of errors, to avoid exiting if Liqo had already been (partially) uninstalled
	set +e

	info "[UNINSTALL]" "Uninstalling Liqo from your cluster..."
	${HELM} uninstall liqo --kube-context "${KUBECONFIG_CONTEXT}" --namespace "${LIQO_NAMESPACE}" 1>/dev/null 2>&1

	info "[UNINSTALL]" "Waiting for all Liqo pods to terminate..."
	${KUBECTL} wait pods --timeout=120s --namespace liqo --all --for=delete 1>/dev/null 2>&1

 	${KUBECTL} delete MutatingWebhookConfiguration mutatepodtoleration 1>/dev/null 2>&1
	${KUBECTL} delete ValidatingWebhookConfiguration peering-request-operator 1>/dev/null 2>&1

	${KUBECTL} delete certificatesigningrequest "peering-request-operator.${LIQO_NAMESPACE}" 1>/dev/null 2>&1
	${KUBECTL} delete certificatesigningrequest "mutatepodtoleration.${LIQO_NAMESPACE}" 1>/dev/null 2>&1

	local GATEWAY_LABEL="net.liqo.io/gateway"
	${KUBECTL} label nodes --selector ${GATEWAY_LABEL} ${GATEWAY_LABEL}- 1>/dev/null 2>&1

	info "[UNINSTALL]" "Liqo has been correctly uninstalled from your cluster"
	agent_uninstall
	set -e
}

function purge_liqo() {
	[ "${PURGE_LIQO}" = true ] || return 0

	# Do not fail in case of errors, to avoid exiting if Liqo had already been (partially) uninstalled
	set +e

	info "[UNINSTALL]" "Purging all remaining Liqo resources from your cluster..."
	${KUBECTL} delete --filename="${TMPDIR}/${LIQO_CHARTS_PATH}/crds" 1>/dev/null 2>&1
	${KUBECTL} delete namespace "${LIQO_NAMESPACE}" 1>/dev/null 2>&1
	info "[UNINSTALL]" "All Liqo resources have been succesfully purged"

	set -e
}


function main() {
	setup_colors
	print_logo

	parse_arguments "$@"

	setup_tmpdir
	setup_kubectl
	setup_downloader

	setup_liqo_version
	download_liqo
	download_helm

	configure_namespace

	if [[ ${INSTALL_LIQO} = true ]]; then
		configure_installation_variables
		configure_gateway_node
		install_liqo
	else
		unjoin_clusters
		uninstall_liqo
		purge_liqo
	fi
}

# This check prevents the script from being executed when sourced,
# hence enabling the possibility to perform unit testing
if ! (return 0 2>/dev/null); then
	main "$@"
	exit ${EXIT_SUCCESS}
fi

function agent_install() {
    echo "[INSTALL]: Installing LIQO Desktop Agent..."
    AGENT_INSTALL_DIR="$TMPDIR/liqo/scripts/tray-agent"
    cd "$AGENT_INSTALL_DIR"
    #1- DOWNLOAD AND INSTALL AGENT BINARY WITH RELATED FILES
    # ----> TO DEFINE AGENT LINK FROM GITHUB ACTIONS
    AGENT_URL=""
    curl --fail -L ${AGENT_URL} | tar xpzf -
    # /usr/bin is one of the directories always present in PATH
    sudo mv -f "liqo-agent" /usr/bin
    AGENT_DIR="$HOME/.liqo"
    mkdir -p "$AGENT_DIR/icons"
    rm -rf "$AGENT_DIR/icons/*"
    tar xzf io.liqo.Agent_icons.tar.gz -C "$AGENT_DIR/icons" --strip 1
    #2- INSTALL AGENT AS DESKTOP APPLICATION
    # a) Installing '.desktop' file in one of $XDG_DATA_DIRS/applications/ to let the system
    # recognize the binary as full application.
    # Using /usr/share since it is one of the default/fallback directories for $XDG_DATA_DIRS.
    XDG_APP_DIR="/usr/share/applications"
    sudo mkdir -p "$XDG_APP_DIR"
    sudo cp -f io.liqo.Agent.desktop "$XDG_APP_DIR"
    # b) Installing '.desktop' in one of $XDG_CONFIG_DIRS/autostart to enable autostart.
    # Having the file in both directories will let an easier implementation of a "don't start at boot" option.
    # Using /etc/xdg since it is the default dir for $XDG_CONFIG_DIRS.
    AUTOSTART_DIR="/etc/xdg/autostart"
    sudo mkdir -p "$AUTOSTART_DIR"
    sudo mv -f io.liqo.Agent.desktop "$AUTOSTART_DIR"
    # c) exporting Agent icon in 'scalable' format to the default theme in one of $XDG_DATA_DIRS/icons/hicolor/scalable/apps.
    # Using /usr/share since it is one of the default dirs for $XDG_DATA_DIRS.
    ICONS_DIR="/usr/share/icons/hicolor/scalable/apps"
    sudo mkdir -p "$ICONS_DIR"
    sudo mv -f io.liqo.Agent.svg "$ICONS_DIR"
    echo "...OK"
    cd -
}

function agent_uninstall() {
  if [ -f "/usr/bin/liqo-agent" ]; then
      echo "[UNINSTALL]: uninstalling Liqo Agent..."
      #uninstalling binary
      sudo rm -f /usr/bin/liqo-agent
      rm -rf "$HOME/.liqo"
      #uninstalling desktop application
      sudo rm -f /usr/share/applications/io.liqo.Agent.desktop
      sudo rm -f /etc/xdg/autostart/io.liqo.Agent.desktop
      sudo rm -f /usr/share/icons/hicolor/scalable/apps/io.liqo.Agent.svg
      echo "...OK"
  fi
}