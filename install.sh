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
#
#   - LIQO_INGRESS_CLASS
#     the kubernetes ingress class to be used for the ingresses created by Liqo.
#   - LIQO_APISERVER_ADDR
#     the address where to contact the home API Server (defaults to the master node IP).
#   - LIQO_APISERVER_PORT
#     the address where to contact the home API Server (defaults to 6443).
#   - LIQO_AUTHSERVER_ADDR
#     the hostname assigned to the Liqo AuthService (exposed through an Ingress resource).
#   - LIQO_AUTHSERVER_PORT
#     the address where to contact the home API Server (exposed through an Ingress resource, defaults to 443).
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

EXIT_SUCCESS=0
EXIT_FAILURE=1

LIQO_REPO_DEFAULT="liqotech/liqo"
LIQO_CHARTS_PATH="deployments/liqo"

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

	  ${BOLD}LIQO_INGRESS_CLASS${RESET}:   the kubernetes ingress class to be used for the ingresses created by Liqo.
	  ${BOLD}LIQO_APISERVER_ADDR${RESET}:  the address where to contact the home API Server (defaults to the master node IP).
	  ${BOLD}LIQO_APISERVER_PORT${RESET}:  the address where to contact the home API Server (defaults to 6443).
	  ${BOLD}LIQO_AUTHSERVER_ADDR${RESET}: the hostname assigned to the Liqo AuthService (exposed through an Ingress resource).
	  ${BOLD}LIQO_AUTHSERVER_PORT${RESET}: the address where to contact the home API Server (exposed through an Ingress resource, defaults to 443).

	  ${BOLD}POD_CIDR${RESET}:           the Pod CIDR of your cluster (e.g.; 10.0.0.0/16). Automatically detected if not configured.
	  ${BOLD}SERVICE_CIDR${RESET}:       the Service CIDR of your cluster (e.g.; 10.96.0.0/12). Automatically detected if not configured.

	  ${BOLD}KUBECONFIG${RESET}:         the KUBECONFIG file used to interact with the cluster (defaults to ~/.kube/config).
	  ${BOLD}KUBECONFIG_CONTEXT${RESET}: the context selected to interact with the cluster (defaults to the current one).

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
		info "[PRE-FLIGHT][${OS}]" "package '${PACKAGE}' is not installed. Do you want to install it ?"
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
	command_exists "brew" || fatal "[PRE-FLIGHT][${OS}]" "please install brew. It is needed to install required package"

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

function setup_liqo_version() {
	# Check if LIQO_REPO has been set
	LIQO_REPO=${LIQO_REPO:-${LIQO_REPO_DEFAULT}}
	# Obtain the list of Liqo tags
	local LIQO_TAGS
	LIQO_TAGS=$(get_repo_tags "${LIQO_REPO}")


	# A specific commit has been requested: assuming development version and returning
	if [[ "${LIQO_VERSION:-}" =~ ^[0-9a-f]{40}$ ]]; then
		warn "[PRE-FLIGHT] [DOWNLOAD]" "A Liqo commit has been specified: using the development version"
		LIQO_IMAGE_VERSION=${LIQO_VERSION}
		LIQO_CHART_VERSION=$(printf "%s" "${LIQO_TAGS}" | head --lines=1)+git
		return 0
	fi

	# If no version has been specified, select the latest tag (if available)
	LIQO_VERSION=${LIQO_VERSION:-$(printf "%s" "${LIQO_TAGS}" | head --lines=1)}


	if [ "${LIQO_VERSION:=master}" != "master" ]; then
		# A specific version has been requested: check if the version exists
		printf "%s" "${LIQO_TAGS}" | grep -P --silent "^${LIQO_VERSION}$" ||
			fatal "[PRE-FLIGHT] [DOWNLOAD]" "The requested Liqo version '${LIQO_VERSION}' does not exist"
		LIQO_IMAGE_VERSION=${LIQO_VERSION}
		LIQO_CHART_VERSION=${LIQO_VERSION}
	else
		# Using the version from master
		warn "[PRE-FLIGHT] [DOWNLOAD]" "An unreleased version of Liqo is going to be downloaded"
		LIQO_IMAGE_VERSION=$(get_repo_master_commit "${LIQO_REPO}") ||
			fatal "[PRE-FLIGHT] [DOWNLOAD]" "Failed to retrieve the latest commit of the master branch"
		LIQO_CHART_VERSION=$(printf "%s" "${LIQO_TAGS}" | head --lines=1)+git
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
	local HELM_VERSION=v3.5.3
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

	# Set variables for kubernetes ingress
	if [ -z "${LIQO_AUTHSERVER_ADDR:-}" ]; then
	  LIQO_ENABLE_INGRESS=""
	else
	  LIQO_ENABLE_INGRESS="true"
	fi
	if [ -n "${LIQO_AUTHSERVER_ADDR:-}" ]; then
	  LIQO_AUTHSERVER_PORT="${LIQO_AUTHSERVER_PORT:-"443"}"
	fi
	if [ -n "${LIQO_APISERVER_ADDR:-}" ]; then
	  LIQO_APISERVER_PORT="${LIQO_APISERVER_PORT:-"6443"}"
	fi
	if [ -n "${LIQO_ENABLE_INGRESS:-}" ]; then
	  info "[INSTALL] [CONFIGURE]" "Ingress Class: ${LIQO_INGRESS_CLASS:-"default"}"
	  info "[INSTALL] [CONFIGURE]" "AuthServer: ${LIQO_AUTHSERVER_ADDR}:${LIQO_AUTHSERVER_PORT}"
	  info "[INSTALL] [CONFIGURE]" "APIServer: ${LIQO_APISERVER_ADDR}:${LIQO_APISERVER_PORT}"
	fi
}

function install_liqo() {
	info "[INSTALL]" "Installing Liqo on your cluster..."

	# Ignore errors that may occur if the namespace already exists
	${KUBECTL} create namespace "${LIQO_NAMESPACE}" 1>/dev/null 2>&1 || true
	local LIQO_CHART="${TMPDIR}/${LIQO_CHARTS_PATH}"
	info "[INSTALL]" "Packaging the chart..."
	${HELM} package --version="${LIQO_CHART_VERSION}" --app-version="${LIQO_CHART_VERSION}" "${LIQO_CHART}" --dependency-update >/dev/null ||
			fatal "[INSTALL]" "Something went wrong while installing Liqo"

	${HELM} install liqo --kube-context "${KUBECONFIG_CONTEXT}" --namespace "${LIQO_NAMESPACE}" "liqo-${LIQO_CHART_VERSION}.tgz" \
		--set tag="${LIQO_IMAGE_VERSION}"  --set discovery.Config.clusterName="${CLUSTER_NAME}" \
		--set networkManager.config.podCIDR="${POD_CIDR}" --set networkManager.config.serviceCIDR="${SERVICE_CIDR}" \
		--set auth.config.allowEmptyToken=true \
		--set auth.ingress.enable="${LIQO_ENABLE_INGRESS:-}" \
		--set auth.ingress.host="${LIQO_AUTHSERVER_ADDR:-}" \
		--set auth.ingress.class="${LIQO_INGRESS_CLASS:-}" \
		--set apiServer.address="${LIQO_APISERVER_ADDR:-}" \
		--set auth.ingress.host="${LIQO_AUTHSERVER_ADDR:-}" \
		--set auth.portOverride="${LIQO_AUTHSERVER_PORT:-}" >/dev/null ||
			fatal "[INSTALL]" "Something went wrong while installing Liqo"

	info "[INSTALL]" "Hooray! Liqo is now installed on your cluster"
}

function uninstall_liqo() {
	# Do not fail in case of errors, to avoid exiting if Liqo had already been (partially) uninstalled
	set +e

	info "[UNINSTALL]" "Uninstalling Liqo from your cluster..."
	${HELM} uninstall liqo --kube-context "${KUBECONFIG_CONTEXT}" --namespace "${LIQO_NAMESPACE}" 1>/dev/null 2>&1

	info "[UNINSTALL]" "Waiting for all Liqo pods to terminate..."
	${KUBECTL} wait pods --timeout=120s --namespace liqo --all --for=delete 1>/dev/null 2>&1

	info "[UNINSTALL]" "Liqo has been correctly uninstalled from your cluster"
	set -e
}

function purge_liqo() {
	[ "${PURGE_LIQO}" = true ] || return 0

	# Do not fail in case of errors, to avoid exiting if Liqo had already been (partially) uninstalled
	set +e

	info "[UNINSTALL]" "Purging all remaining Liqo resources from your cluster..."
	${KUBECTL} delete --filename="${TMPDIR}/${LIQO_CHARTS_PATH}/crds" 1>/dev/null 2>&1
	${KUBECTL} delete namespace "${LIQO_NAMESPACE}" 1>/dev/null 2>&1
	info "[UNINSTALL]" "All Liqo resources have been successfully purged"

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
		install_liqo
	else
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
