#!/usr/bin/env bash
set -e          # Fail in case of error
set -o nounset  # Fail if undefined variables are used
set -o pipefail # Fail if one of the piped commands fails

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
#   - INSTALL_AGENT
#     enables the installation of Liqo Desktop Agent.

EXIT_SUCCESS=0
EXIT_FAILURE=1

LIQO_REPO="liqotech/liqo"
LIQO_CHARTS_PATH="deployments/liqo_chart"

LIQO_DASHBOARD_REPO="liqotech/dashboard"

LIQO_NAMESPACE_DEFAULT="liqo"
CLUSTER_NAME_DEFAULT=$(printf "LiqoCluster%04d" $((RANDOM % 10000)))

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
	
		  ${BOLD}INSTALL_AGENT${RESET}:      set it to 'true' to enable the Liqo Desktop Agent installation.
	
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
			--help | -h)
				help
				exit ${EXIT_SUCCESS}
				;;

			--uninstall)
				INSTALL_LIQO=false
				;;
			--purge)
				INSTALL_LIQO=false
				PURGE_LIQO=true
				;;

			--)
				shift
				break
				;;
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

function get_agent_link() {
	# Currently the installer searches for a Liqo Agent binary always from the latest release.
	local latest_url="https://api.github.com/repos/${LIQO_REPO}/releases/latest"
	info "[PRE-FLIGHT] [CONFIGURE]" "Searching for the latest version of Liqo Agent binary..."
	# The function checks first if there is any release version for the current repository to avoid a potential hard failure
	# by directly accessing the 'latest release' resource (if no version exists).
	local tag_flag
	tag_flag=$(get_repo_tags ${LIQO_REPO} | head --lines=1)
	if [[ -z "${tag_flag}" ]]; then
		AGENT_ASSET_URL=""
	else
		AGENT_ASSET_URL=$(download ${latest_url} | grep -Po '"browser_download_url": "\K.*liqo-agent.*(?=")' || echo "")
	fi
}

function setup_agent_environment() {
	# Default directory for XDG_CONFIG_HOME.
	AGENT_XDG_CONFIG_DIR="${HOME}/.config/"
	# Default directory for XDG_DATA_HOME.
	AGENT_XDG_DATA_DIR="${HOME}/.local/share"

	# Directory containing the Agent binary.
	AGENT_BIN_DIR="${HOME}/.local/bin"
	# Liqo subdirectory containing the notifications icons.
	AGENT_ICONS_DIR="${AGENT_XDG_DATA_DIR}/liqo/icons"
	# Directory storing the '.desktop' file.
	AGENT_APP_DIR="${AGENT_XDG_DATA_DIR}/applications"
	# Directory storing the scalable icon for the desktop application.
	AGENT_THEME_DIR="${AGENT_XDG_DATA_DIR}/icons/hicolor/scalable/apps"
	# Directory storing the '.desktop' file to enable the application autostart.
	AGENT_AUTOSTART_DIR="${AGENT_XDG_CONFIG_DIR}/autostart"

	[[ "$1" == "-c" ]] && mkdir -p AGENT_XDG_CONFIG_DIR AGENT_XDG_DATA_DIR \
	AGENT_BIN_DIR AGENT_ICONS_DIR AGENT_APP_DIR AGENT_THEME_DIR AGENT_AUTOSTART_DIR
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
				fatal "[PRE-FLIGHT] [DOWNLOAD]" "Failed downloading $1"
			;;
		wget)
			wget --quiet --output-document=- "$1" ||
				fatal "[PRE-FLIGHT] [DOWNLOAD]" "Failed downloading $1"
			;;
		*)
			fatal "[PRE-FLIGHT] [DOWNLOAD]" "Internal error: incorrect downloader"
			;;
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
	PHASE=$([ "${INSTALL_LIQO}" = true ] && echo "INSTALL" || echo "UNINSTALL")

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
				--output jsonpath="{.items[*].spec.containers[*].command}" 2>/dev/null |
				grep -Po --max-count=1 "(?<=--cluster-cidr=)[0-9.\/]+"
		) ||
			fatal "[INSTALL] [CONFIGURE]" "Failed to automatically retrieve the Pod CIDR." \
				"Please, manually specify it with 'export POD_CIDR=...' before executing again this script"
	fi
	info "[INSTALL] [CONFIGURE]" "Pod CIDR: ${POD_CIDR}"

	# Attempt to retrieve the Service CIDR (in case it has not been specified)
	if [ -z "${SERVICE_CIDR:-}" ]; then
		SERVICE_CIDR=$(
			${KUBECTL} get pods --namespace kube-system \
				--selector component=kube-controller-manager \
				--output jsonpath="{.items[*].spec.containers[*].command} 2>dev/null" |
				grep -Po --max-count=1 "(?<=--service-cluster-ip-range=)[0-9.\/]+"
		) ||
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

	if [ "$(wc --words <<<"${GATEWAY}")" -ge 2 ]; then
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
}

function install_agent() {
	[[ "${INSTALL_AGENT}" != "true" ]] && return 0
	info "[INSTALL]" "LIQO DESKTOP AGENT INSTALLATION"
	get_agent_link
	if [[ -z "${AGENT_ASSET_URL}" ]]; then
		warn "[PRE-FLIGHT] [CONFIGURE]" "No Liqo Agent binary found! Skipping Agent installation"
		return 0
	fi
	info "[PRE-FLIGHT] [CONFIGURE]" "Liqo Agent binary found!"
	local AGENT_INSTALL_DIR="${TMPDIR}/scripts/tray-agent"
	cd "${AGENT_INSTALL_DIR}"

	info "[AGENT INSTALL] [1/4]" "Downloading binary"
	download "${AGENT_ASSET_URL}" | tar xpzf - || fatal "[DOWNLOAD]" "Something went wrong while extracting \
	the Agent archive"

	info "[AGENT INSTALL] [2/4]" "Preparing environment"
	setup_agent_environment -c

	info "[AGENT INSTALL] [3/4]" "Installing binary"
	# moving binary
	mv -f "liqo-agent" "${AGENT_BIN_DIR}"
	# moving notifications icons
	tar xzf io.liqo.Agent_icons.tar.gz -C "${AGENT_ICONS_DIR}" --strip 1 --overwrite || \
	fatal "[INSTALL]" "Something went wrong while extracting files"

	info "[AGENT INSTALL] [4/4]" "Installing Desktop Application"
	# INSTALL AGENT AS A DESKTOP APPLICATION
	# a) Inject binary path into '.desktop' file.
	echo Exec='"'"${AGENT_BIN_DIR}/liqo-agent"'"' >> io.liqo.Agent.desktop
	# The x permission is required to let the system trust the application to autostart.
	chmod +x io.liqo.Agent.desktop
	# b) The '.desktop' file is installed in one of the XDG_DATA_* directories to let the
	# system recognize Liqo Agent as a desktop application.
	cp -f io.liqo.Agent.desktop "${AGENT_APP_DIR}"
	# c) The '.desktop' file is installed in one of the XDG_CONFIG_* directories to enable autostart.
	# Having the file in both directories allows an easier management of a "don't start at boot" option.
	mv -f io.liqo.Agent.desktop "${AGENT_AUTOSTART_DIR}"
	# d) The Liqo Agent desktop icon is exported in 'scalable' format for the default theme to one of the
	# $XDG_DATA_*/icons/hicolor/scalable/apps directories.
	mv -f io.liqo.Agent.svg "${AGENT_THEME_DIR}"
	# e) In order to automatically trust the application, the '.desktop' file copies' metadata
	# are trusted using gio after they are moved in their respective location.
	if command_exists gio; then
		gio set "${AGENT_APP_DIR}/io.liqo.Agent.desktop" "metadata::trusted" yes
		gio set "${AGENT_AUTOSTART_DIR}/io.liqo.Agent.desktop" "metadata::trusted" yes
	fi
	# f) If a '.profile' shell config file is present, export the two AGENT_XDG_* directories
	# to ensure they will be scanned by the display manager even if the user would set the
	# XDG_DATA_HOME and XDG_CONFIG_HOME variables with values different from their respective
	# default.
	if [[ -s "${HOME}/.profile" ]]; then
		# shellcheck disable=SC2016
		echo 'export XDG_CONFIG_DIRS="'"${AGENT_XDG_CONFIG_DIR}:"'$XDG_CONFIG_DIRS"' >> "${HOME}/.profile"
		# shellcheck disable=SC2016
		echo 'export XDG_DATA_DIRS="'"${AGENT_XDG_DATA_DIR}:"'$XDG_DATA_DIRS"' >> "${HOME}/.profile"
	fi
	info "[AGENT INSTALL]" "Installation complete!"
	info "[AGENT INSTALL]" "Please restart your computer for the applied changes to make effect"
	cd -
	"${AGENT_BIN_DIR}"/liqo-agent &
}

function all_clusters_unjoined() {
	local JSON_PATH="{.items[*].spec.join} {.items[*].status.incoming.joined} {.items[*].status.outgoing.joined} {.items[*].status.network.localNetworkConfig.available} {.items[*].status.network.remoteNetworkConfig.available} {.items[*].status.network.tunnelEndpoint.available}"
	(${KUBECTL} get foreignclusters --output jsonpath="${JSON_PATH}" 2>/dev/null || echo "") |
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
		RETRIES=$((RETRIES - 1))
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
	set -e
}

function uninstall_agent() {
	setup_agent_environment
	# Do not fail in case of errors, to avoid exiting if Liqo Agent had already been (partially) uninstalled.
	info "[UNINSTALL] [AGENT]" "Uninstalling Liqo Agent components"
	set +e
	# Uninstalling main components.
	rm -f "${AGENT_BIN_DIR}/liqo-agent"
	rm -rf "${AGENT_XDG_DATA_DIR}/liqo"
	# Uninstalling desktop application files.
	rm -f "${AGENT_APP_DIR}/io.liqo.Agent.desktop"
	rm -f "${AGENT_AUTOSTART_DIR}/io.liqo.Agent.desktop"
	rm -f "${AGENT_THEME_DIR}/io.liqo.Agent.svg"
	if [[ -s "${HOME}/.profile" ]] && command_exists sed; then
		# shellcheck disable=SC2016
		local str1='export XDG_CONFIG_DIRS="'"${AGENT_XDG_CONFIG_DIR}:"'$XDG_CONFIG_DIRS"'
		# shellcheck disable=SC2016
		local str2='export XDG_DATA_DIRS="'"${AGENT_XDG_DATA_DIR}:"'$XDG_DATA_DIRS"'
		# remove the exact added exports, trying not to compromise user edits.
		sed -i "/$str1/d" "${HOME}/.profile"
		sed -i "/$str2/d" "${HOME}/.profile"
	fi
	info "[UNINSTALL] [AGENT]" "Liqo Agent was correctly uninstalled"
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

	if [[ ${INSTALL_LIQO} == true ]]; then
		configure_installation_variables
		configure_gateway_node
		install_liqo
		install_agent
	else
		unjoin_clusters
		uninstall_liqo
		purge_liqo
		uninstall_agent
	fi
}

# This check prevents the script from being executed when sourced,
# hence enabling the possibility to perform unit testing
if ! (return 0 2>/dev/null); then
	main "$@"
	exit ${EXIT_SUCCESS}
fi
