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
#   - LIQO_REPO
#
#     the repository of Liqo to install. Defaults to "liqotech/liqo", but can be changed in case of forks.
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

LIQO_REPO_DEFAULT="liqotech/liqo"
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
	  ${BOLD}LIQO_REPO${RESET}:          the repository of Liqo to install. Defaults to "liqotech/liqo", but can be changed in case of forks.
	  ${BOLD}LIQO_VERSION${RESET}:       the version of Liqo to install. It can be a released version, a commit SHA or 'master'.

	  ${BOLD}LIQO_NAMESPACE${RESET}:     the Kubernetes namespace where all Liqo components are created (defaults to liqo)
	  ${BOLD}CLUSTER_NAME${RESET}:       the mnemonic name assigned to this Liqo instance. Automatically generated if not specified.
	  ${BOLD}DASHBOARD_HOSTNAME${RESET}: the hostname assigned to the Liqo dashboard (exposed through an Ingress resource).

	  ${BOLD}POD_CIDR${RESET}:           the Pod CIDR of your cluster (e.g.; 10.0.0.0/16). Automatically detected if not configured.
	  ${BOLD}SERVICE_CIDR${RESET}:       the Service CIDR of your cluster (e.g.; 10.96.0.0/12). Automatically detected if not configured.

	  ${BOLD}KUBECONFIG${RESET}:         the KUBECONFIG file used to interact with the cluster (defaults to ~/.kube/config).
	  ${BOLD}KUBECONFIG_CONTEXT${RESET}: the context selected to interact with the cluster (defaults to the current one).

	  ${BOLD}INSTALL_AGENT${RESET}:      set this variable to 'true' to enable Liqo Agent installation on your desktop.
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

function darwin_install_gnu_tool(){
	local PACKAGE=$1
	local BINARY_PATH=$2

	if ! brew list "${PACKAGE}"  > /dev/null 2>&1; then
		info "[PRE-FLIGHT][${OS}]" "package '${PACKAGE}' is not installed. Do you want ot install it ?"
		select yn in "Yes" "No"; do
				case $yn in
						Yes ) brew install "${PACKAGE}";
									info "[PRE-FLIGHT][${OS}]" "package '${PACKAGE}' installed";
									break;;
						No ) fatal "[PRE-FLIGHT][${OS}] package '${PACKAGE}' is required. Abort";;
						* ) warn "[PRE-FLIGHT][${OS}]" "Invalid selected option '${REPLY}'";;
				esac
		# select read input from stdin, if the script is piped (like in demo), the stdin is the pipe. Consequenty the select not works.
		# To avoid this problem we read input from tty
		done < /dev/tty
	fi
	info "[PRE-FLIGHT][${OS}]" "Add gnu tool provided by '${PACKAGE}' package to the PATH"
	export PATH="${BINARY_PATH}:$PATH"


}

function setup_darwin_package(){
	info "[PRE-FLIGHT][${OS}]" "Check necessary gnu-tools (getopts, grep ...) are installed"
	command_exists "brew" || fatal "[PRE-FLIGHT][${OS}]" "please install brew. It need to install package"

	darwin_install_gnu_tool "coreutils" "/usr/local/opt/coreutils/libexec/gnubin"
	darwin_install_gnu_tool "grep" "/usr/local/opt/grep/libexec/gnubin"
	darwin_install_gnu_tool "gnu-getopt" "/usr/local/opt/gnu-getopt/bin"
	darwin_install_gnu_tool "gnu-tar" "/usr/local/opt/gnu-tar/libexec/gnubin"
}


function setup_arch_and_os(){
	ARCH=$(uname -m)
	case $ARCH in
		armv5*) ARCH="armv5";;
		armv6*) ARCH="armv6";;
		armv7*) ARCH="arm";;
		aarch64) ARCH="arm64";;
		x86) ARCH="386";;
		x86_64) ARCH="amd64";;
		i686) ARCH="386";;
		i386) ARCH="386";;
		*) fatal "[PRE-FLIGHT] architecture '${ARCH}' unknown"; return ;;
	esac

	OS=$(uname |tr '[:upper:]' '[:lower:]')
	case "$OS" in
		"darwin"*) setup_darwin_package;;
		# Minimalist GNU for Windows
		"mingw"*) OS='windows';;
	esac

	# borrow to helm install script: https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3
	local supported="darwin-amd64\nlinux-386\nlinux-amd64\nlinux-arm\nlinux-arm64\nlinux-ppc64le\nlinux-s390x\nwindows-amd64"
	if ! echo "${supported}" | grep -q "${OS}-${ARCH}"; then
		fatal "[PRE-FLIGHT] System '${OS}-${ARCH}' not supported."
	fi

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

function get_liqo_releases() {
	# The Liqo Agent binary resides only inside Liqo releases which are a subset of tagged versions.
	# Only tags returned by this function are valid to download it.
	# The maximum number of retrieved tags is 100, but this should not raise concerns for a while
	local RELEASES_URL="https://api.github.com/repos/${LIQO_REPO}/releases?page=1&per_page=100"
	download "${RELEASES_URL}" | grep -Po '"tag_name": "\K.*?(?=")' || echo ""
}

function get_repo_master_commit() {
	[ $# -eq 1 ] || fatal "[PRE-FLIGHT] [DOWNLOAD]" "Internal error: incorrect parameters"
	# The maximum number of retrieved tags is 100, but this should not raise concerns for a while
	local MASTER_COMMIT_URL="https://api.github.com/repos/$1/commits?page=1&per_page=1"
	download "${MASTER_COMMIT_URL}" | grep -Po --max-count=1 '"sha": "\K.*?(?=")'
}

function setup_installation_platform() {
	# Identify on which kind of platform the installer is running.
	# More specific filters will be added with further support of additional environments.
	local host_system
	host_system=$(uname)
	case "${host_system}" in
		'Linux')
			INSTALLATION_PLATFORM="linux" ;;
		*)
			INSTALLATION_PLATFORM="other" ;;
	esac

}

function setup_agent_version() {
	# Get the latest releases also checking if there is any release available.
	local LIQO_RELEASES
	LIQO_RELEASES=$(get_liqo_releases)
	[[ -z "${LIQO_RELEASES}" ]] && return 0
	local RELEASE_URL
	# The Liqo Agent binary is available for download only from released versions of Liqo.
	# When a specific tagged version of Liqo is chosen, a Liqo Agent binary from the same release will be installed
	# (if requested and available).
	if [[ "${LIQO_VERSION:-}" =~ ^v ]]; then
		info "[AGENT] [PRE-FLIGHT]" "A Liqo Agent app matching requested Liqo version will be installed (${LIQO_VERSION})."
		info "[AGENT] [PRE-FLIGHT]" "Searching for requested version of Liqo Agent binary..."
		printf "%s" "${LIQO_RELEASES}" | grep -P --silent "^${LIQO_VERSION}$" || return 0
		RELEASE_URL="https://api.github.com/repos/${LIQO_REPO}/releases/tags/${LIQO_VERSION}"
	else
		warn "[AGENT] [PRE-FLIGHT]" "No valid release version has been requested. Switching to latest version."
		warn "[AGENT] [PRE-FLIGHT]" "Searching for the latest version of Liqo Agent binary..."
		RELEASE_URL="https://api.github.com/repos/${LIQO_REPO}/releases/latest"
	fi
	AGENT_ASSET_URL=$(download "${RELEASE_URL}" | grep -Po '"browser_download_url": "\K.*liqo-agent.*(?=")' || echo "")
}

function setup_agent_environment() {
	[[ -z ${HOME:-} ]] && HOME=~
	# Default directory for XDG_CONFIG_HOME.
	AGENT_XDG_CONFIG_DIR="${XDG_CONFIG_HOME:-${HOME}/.config/}"
	# Default directory for XDG_DATA_HOME.
	AGENT_XDG_DATA_DIR="${XDG_DATA_HOME:-${HOME}/.local/share}"

	# Directory containing the Agent binary.
	AGENT_BIN_DIR="${HOME}/.local/bin"
	# Liqo Agent root directory containing all resources related to Liqo.
	AGENT_LIQO_DIR="${AGENT_XDG_DATA_DIR}/liqo"
	# Name of the Liqo Agent config file.
	AGENT_CONFIG_FILE_NAME="agent_conf.yaml"
	# Filepath of the Liqo Agent config file.
	AGENT_CONF_FILE_PATH="${AGENT_LIQO_DIR}/${AGENT_CONFIG_FILE_NAME}"
	# Liqo subdirectory containing the notifications icons.
	AGENT_ICONS_DIR="${AGENT_LIQO_DIR}/icons"
	# Directory storing the '.desktop' file.
	AGENT_APP_DIR="${AGENT_XDG_DATA_DIR}/applications"
	# Directory storing the scalable icon for the desktop application.
	AGENT_THEME_DIR="${AGENT_XDG_DATA_DIR}/icons/hicolor/scalable/apps"
	# Directory storing the '.desktop' file to enable the application autostart.
	AGENT_AUTOSTART_DIR="${AGENT_XDG_CONFIG_DIR}/autostart"

	if [[ "${1:-}" == "--create" ]]; then
		mkdir --parent "${AGENT_XDG_CONFIG_DIR}" "${AGENT_XDG_DATA_DIR}" \
		"${AGENT_BIN_DIR}" "${AGENT_ICONS_DIR}" "${AGENT_APP_DIR}" \
		"${AGENT_THEME_DIR}" "${AGENT_AUTOSTART_DIR}"
	fi
}

function setup_liqo_version() {
	# Check if LIQO_REPO has been set
	LIQO_REPO=${LIQO_REPO:-${LIQO_REPO_DEFAULT}}

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
	LIQO_TAGS=$(get_repo_tags "${LIQO_REPO}")

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
		LIQO_IMAGE_VERSION=$(get_repo_master_commit "${LIQO_REPO}") ||
			fatal "[PRE-FLIGHT] [DOWNLOAD]" "Failed to retrieve the latest commit of the master branch"
		LIQO_SUFFIX="-ci"

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
	local HELM_ARCHIVE=helm-${HELM_VERSION}-${OS}-${ARCH}.tar.gz
	local HELM_URL=https://get.helm.sh/${HELM_ARCHIVE}

	info "[PRE-FLIGHT] [DOWNLOAD]" "Downloading Helm ${HELM_VERSION}"
	command_exists tar || fatal "[PRE-FLIGHT] [DOWNLOAD]" "'tar' is not available"
	download "${HELM_URL}" | tar zxf - --directory="${BINDIR}" 2>/dev/null ||
		fatal "[PRE-FLIGHT] [DOWNLOAD]" "Something went wrong while extracting the Helm archive"
	HELM="${BINDIR}/$OS-$ARCH/helm"
}

function download_liqo() {
	info "[PRE-FLIGHT] [DOWNLOAD]" "Downloading Liqo (version: ${LIQO_VERSION})"
	command_exists tar || fatal "[PRE-FLIGHT] [DOWNLOAD]" "'tar' is not available"
	local LIQO_DOWNLOAD_URL="https://github.com/${LIQO_REPO}/archive/${LIQO_VERSION}.tar.gz"
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
}

function install_agent() {
	[[ "${INSTALL_AGENT:-}" != "true" ]] && return 0
	info "[AGENT] [PRE-FLIGHT]" "Liqo desktop agent installation"
	# Currently Liqo only supports the Linux-based version of Liqo Agent.
	# This check prevents useless and bad installations of the Liqo Agent on desktop environments that are only
	# partially compatible, e.g. MacOs or cygwin for Windows.
	setup_installation_platform
	if [[ "${INSTALLATION_PLATFORM}" != "linux" ]]; then
		warn "[AGENT] [PRE-FLIGHT]" "Sorry, Liqo Agent does not support your desktop OS for now! Skipping Agent installation"
		return 0
	fi

	setup_agent_version
	if [[ -z "${AGENT_ASSET_URL:-}" ]]; then
		warn "[AGENT] [PRE-FLIGHT]" "No Liqo Agent binary found! Skipping Agent installation"
		return 0
	fi
	info "[AGENT] [PRE-FLIGHT]" "Liqo Agent binary found!"

	local AGENT_ASSETS_DIR="${TMPDIR}/assets/tray-agent"
	local AGENT_INSTALL_LINUX_DIR="${AGENT_ASSETS_DIR}/install/linux"

	info "[AGENT] [INSTALL] [1/4]" "Downloading binary"
	download "${AGENT_ASSET_URL}" | tar xpzf - --directory="${BINDIR}" ||
		fatal "[AGENT] [INSTALL]" "Something went wrong while extracting the Agent archive"

	info "[AGENT] [INSTALL] [2/4]" "Preparing environment"
	setup_agent_environment --create

	info "[AGENT] [INSTALL] [3/4]" "Installing binary"
	# moving binary
	mv -f "${BINDIR}/liqo-agent" "${AGENT_BIN_DIR}"
	# moving notifications icons
	mv -f "${AGENT_ASSETS_DIR}"/icons/desktop/* "${AGENT_ICONS_DIR}" ||
		fatal "[AGENT] [INSTALL]" "Something went wrong while copying files"

	info "[AGENT] [INSTALL] [4/4]" "Installing Desktop Application"
	# INSTALL AGENT AS A DESKTOP APPLICATION
	# a) Inject binary path into '.desktop' file.
	echo Exec='"'"${AGENT_BIN_DIR}/liqo-agent"'"' >> "${AGENT_INSTALL_LINUX_DIR}/io.liqo.Agent.desktop"
	# The x permission is required to let the system trust the application to autostart.
	chmod +x "${AGENT_INSTALL_LINUX_DIR}/io.liqo.Agent.desktop"
	# b) The '.desktop' file is installed in one of the XDG_DATA_* directories to let the
	# system recognize Liqo Agent as a desktop application.
	cp -f "${AGENT_INSTALL_LINUX_DIR}/io.liqo.Agent.desktop" "${AGENT_APP_DIR}"
	# c) The '.desktop' file is installed in one of the XDG_CONFIG_* directories to enable autostart.
	# Having the file in both directories allows an easier management of a "don't start at boot" option.
	mv -f "${AGENT_INSTALL_LINUX_DIR}/io.liqo.Agent.desktop" "${AGENT_AUTOSTART_DIR}"
	# d) The Liqo Agent desktop icon is exported in 'scalable' format for the default theme to one of the
	# $XDG_DATA_*/icons/hicolor/scalable/apps directories.
	mv -f "${AGENT_INSTALL_LINUX_DIR}/io.liqo.Agent.svg" "${AGENT_THEME_DIR}"
	# e) In order to automatically trust the application, the '.desktop' file copies' metadata
	# are trusted using gio after they are moved in their respective location.
	if command_exists gio; then
		gio set "${AGENT_APP_DIR}/io.liqo.Agent.desktop" "metadata::trusted" yes
		gio set "${AGENT_AUTOSTART_DIR}/io.liqo.Agent.desktop" "metadata::trusted" yes
	fi
	# f) If there are specific parameters needed by the Agent, these are written to a config file.
	write_agent_config_file
	info "[AGENT] [INSTALL]" "Installation complete!"
	command_exists gtk-launch && gtk-launch io.liqo.Agent.desktop
}

function write_agent_config_file() {
	# If there are configuration parameters whose value differs from default, write them down to the agent
	# configuration file, creating or truncating the file if already present.
	# Currently the only considered information is the kubeconfig file path (KUBECONFIG env var).
	if [[ "${KUBECONFIG}" != "${HOME}/.kube/config" ]]; then
		info "[AGENT] [INSTALL]" "writing Liqo Agent configuration file"
		echo "kubeconfig: ${KUBECONFIG}" > "${AGENT_CONF_FILE_PATH}"
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
	set -e
}

function uninstall_agent() {
	setup_installation_platform
	[[ "${INSTALLATION_PLATFORM}" != "linux" ]] && return 0
	setup_agent_environment
	info "[AGENT] [UNINSTALL]" "Uninstalling Liqo Agent components"
	# Uninstalling main components.
	rm -f "${AGENT_BIN_DIR}/liqo-agent"
	rm -rf "${AGENT_XDG_DATA_DIR}/liqo"
	# Uninstalling desktop application files.
	rm -f "${AGENT_APP_DIR}/io.liqo.Agent.desktop"
	rm -f "${AGENT_AUTOSTART_DIR}/io.liqo.Agent.desktop"
	rm -f "${AGENT_THEME_DIR}/io.liqo.Agent.svg"
	info "[AGENT] [UNINSTALL]" "Liqo Agent was correctly uninstalled"
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

	setup_arch_and_os
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
