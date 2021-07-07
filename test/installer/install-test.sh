#!/usr/bin/env bats

# Disabling these check, since they are related to the test approach
# shellcheck disable=SC2030 disable=SC2031

load "${BATS_TEST_DIRNAME}/libs/bats-support/load.bash"
load "${BATS_TEST_DIRNAME}/libs/bats-assert/load.bash"

setup() {
	# Sourcing the installer, in order to unit test the functions
	# shellcheck disable=SC1090
	source "${BATS_TEST_DIRNAME}/../../install.sh"

	# Execute the setup_colors function, otherwise the color variables
	# are undefined and other functions may fail
	setup_colors
}

@test "info, warn and fatal correcly output messages" {
	local ID="[TEST]"
	local FIRST_MESSAGE="First message"
	local SECOND_MESSAGE="Second message"

	# Test the info function
	run info "${ID}" "${FIRST_MESSAGE}" "${SECOND_MESSAGE}"
	assert_success
	assert_output "${ID} ${FIRST_MESSAGE} ${SECOND_MESSAGE}"

	# Test the warn function
	run warn "${ID}" "${FIRST_MESSAGE}" "${SECOND_MESSAGE}"
	assert_success
	assert_output "${ID} ${FIRST_MESSAGE} ${SECOND_MESSAGE}"

	# Test the fatal function
	run fatal "${ID}" "${FIRST_MESSAGE}" "${SECOND_MESSAGE}"
	assert_failure
	assert_output "${ID} [FATAL] ${FIRST_MESSAGE} ${SECOND_MESSAGE}"
}


@test "parse_arguments correctly configures the environment on install" {
	parse_arguments
	assert_equal "${INSTALL_LIQO}" true
	assert_equal "${PURGE_LIQO}" false
}

@test "parse_arguments correctly configures the environment on uninstall" {
	parse_arguments --uninstall
	assert_equal "${INSTALL_LIQO}" false
	assert_equal "${PURGE_LIQO}" false
}

@test "parse_arguments correctly configures the environment on purge" {
	parse_arguments --uninstall --purge
	assert_equal "${INSTALL_LIQO}" false
	assert_equal "${PURGE_LIQO}" true

	unset INSTALL_LIQO
	unset PURGE_LIQO

	parse_arguments --purge
	assert_equal "${INSTALL_LIQO}" false
	assert_equal "${PURGE_LIQO}" true
}

@test "parse_arguments prints the help with -h/--help" {
	run parse_arguments -h
	assert_success
	assert_line "Install Liqo on your Kubernetes cluster"

	run parse_arguments --help
	assert_success
	assert_line "Install Liqo on your Kubernetes cluster"
}

@test "parse_arguments fails in case of unrecognized arguments/options" {
	# Unrecognized argument
	run parse_arguments unrecognized
	assert_failure
	assert_line --partial "unrecognized argument 'unrecognized'"
	# Unrecognized long option
	run parse_arguments --unrecognized
	assert_failure
	assert_line --partial "unrecognized option '--unrecognized'"
	# Unrecognized short option
	run parse_arguments -u
	assert_failure
	assert_line --partial "invalid option -- 'u'"
}


@test "command_exists correctly detects whether a command exists or not" {
	# Existing commands should succeed
	command_exists echo
	command_exists cat

	# Non-existing commands should fail
	! command_exists not_existing

	# Ensure the detection of mocked functions works correctly
	! command_exists mocked_command
	function mocked_command() { echo "mocked"; }
	export -f mocked_command
	command_exists mocked_command
}


@test "setup_downloader selects curl in case it is available" {
	# The setup fails if no downloader can be found
	PATH='' run setup_downloader
	assert_failure
	assert_line --partial "Cannot find neither 'curl' nor 'wget' to download files"

	# wget available, should be used
	function wget() { echo "wget"; }
	export -f wget

	PATH='' run setup_downloader
	assert_success
	assert_output --partial "Using wget to download files"

	# curl also available, should be preferred
	function curl() { echo "curl"; }
	export -f curl

	PATH='' run setup_downloader
	assert_success
	assert_output --partial "Using curl to download files"
}

@test "download correctly uses the selected downloader and returns the output" {
	function wget() { echo "wget"; }
	function curl() { echo "curl"; }
	export -f wget curl

	# Should use curl
	DOWNLOADER="curl" run download "www.example.com"
	assert_success
	assert_output "curl"

	# Should use wget
	DOWNLOADER="wget" run download "www.example.com"
	assert_success
	assert_output "wget"

	# Should fail (DOWNLOADER set to the empty string)
	DOWNLOADER='' run download "www.example.com"
	assert_failure
	assert_output --partial "Internal error: incorrect downloader"

	# Should fail (DOWNLOADER not set)
	run download "www.example.com"
	assert_failure
	assert_output --partial "Internal error: incorrect downloader"
}

@test "download correctly fails if the command fails" {
	function wget() { return 1; }
	function curl() { return 1; }
	export -f wget curl

	DOWNLOADER="curl" run download "www.example.com"
	assert_failure
	assert_output --partial "Failed downloading www.example.com"
	DOWNLOADER="wget" run download "www.example.com"
	assert_failure
	assert_output --partial "Failed downloading www.example.com"
}

@test "download correctly fails if the wrong number of parameters is specified" {
	function curl() { echo "curl"; }
	export -f curl

	DOWNLOADER="curl" run download
	assert_failure
	assert_output --partial "Internal error: incorrect parameters"
	DOWNLOADER="curl" run download www.example.com error
	assert_failure
	assert_output --partial "Internal error: incorrect parameters"
}

@test "setup_liqo_repo correctly configures the environment if a fork repository is specified" {
	LIQO_VERSION="f2de258b07d8b507b461f55f87e17f1bb619f926"
	function get_repo_master_commit() {
			echo "b5de258b07d8b507b461f55f87e17f1bb619f926";
	}

	LIQO_REPO="trololo/liqo"
	setup_liqo_version
	assert_equal "${LIQO_REPO}" "trololo/liqo"

	unset LIQO_REPO
	setup_liqo_version
	assert_equal "${LIQO_REPO}" "liqotech/liqo"
}

@test "setup_liqo_version correctly configures the environment if a commit is specified" {
	LIQO_VERSION="f2de258b07d8b507b461f55f87e17f1bb619f926"
	function get_repo_master_commit() {
		echo "b5de258b07d8b507b461f55f87e17f1bb619f926";
	}

	# Assert that the output is correct
	run setup_liqo_version
	assert_success
	assert_output --partial "A Liqo commit has been specified: using the development version"

	# Run again with side effects, to assert that the variables are correct
	setup_liqo_version
	assert_equal "${LIQO_VERSION}" "f2de258b07d8b507b461f55f87e17f1bb619f926"
	assert_equal "${LIQO_IMAGE_VERSION}" "f2de258b07d8b507b461f55f87e17f1bb619f926"
}

@test "setup_liqo_version correctly configures the environment if master is specified" {
	function get_repo_tags() { echo ""; }
	function get_repo_master_commit() { 
		echo "f2de258b07d8b507b461f55f87e17f1bb619f926";
	}
	declare -f get_repo_tags get_repo_master_commit

	LIQO_VERSION="master"

	# Assert that the output is correct
	run setup_liqo_version
	assert_success
	assert_output --partial "An unreleased version of Liqo is going to be downloaded"

	# Run again with side effects, to assert that the variables are correct
	setup_liqo_version
	assert_equal "${LIQO_VERSION}" "master"
	assert_equal "${LIQO_IMAGE_VERSION}" "f2de258b07d8b507b461f55f87e17f1bb619f926"
}

@test "setup_liqo_version correctly configures the environment if a version is specified" {
	function get_repo_tags() { printf "v1.0.0\nv0.1.0-alpha"; }
	declare -f get_repo_tags

	# Run again with side effects, to assert that the variables are correct
	LIQO_VERSION="v0.1.0-alpha"
	setup_liqo_version
	assert_equal "${LIQO_VERSION}" "v0.1.0-alpha"
	assert_equal "${LIQO_IMAGE_VERSION}" "v0.1.0-alpha"
}

@test "setup_liqo_version correctly detects whether a version is available or not" {
	function get_repo_tags() { printf "v1.0.0\nv0.1.0-alpha"; }
	declare -f get_repo_tags

	# This version is returned by get_repo_tags, hence it should succeed
	LIQO_VERSION="v1.0.0"
	run setup_liqo_version
	assert_success

	# This version is returned by get_repo_tags, hence it should succeed
	LIQO_VERSION="v0.1.0-alpha"
	run setup_liqo_version
	assert_success

	# This version is not returned by get_repo_tags, hence it should fail
	LIQO_VERSION="v0.1.0"
	run setup_liqo_version
	assert_failure
	assert_output --partial "The requested Liqo version '${LIQO_VERSION}' does not exist"
}

@test "setup_liqo_version correctly uses the latest tag or master if no version is specified" {
	function get_repo_tags() { printf "v1.0.0\nv0.1.0-alpha"; }
	declare -f get_repo_tags

	# get_repo_tags returns a list of tags, hence the latest should be used (assumed to be sorted)
	unset LIQO_VERSION
	setup_liqo_version
	assert_equal "${LIQO_VERSION}" "v1.0.0"
	assert_equal "${LIQO_IMAGE_VERSION}" "v1.0.0"

	function get_repo_tags() { echo ""; }
	function get_repo_master_commit() { 
		echo "f2de258b07d8b507b461f55f87e17f1bb619f926";
	}
	declare -f get_repo_tags get_repo_master_commit

	# get_repo_tags returns no tags, hence should fallback to master
	unset LIQO_VERSION
	setup_liqo_version
	assert_equal "${LIQO_VERSION}" "master"
	assert_equal "${LIQO_IMAGE_VERSION}" "f2de258b07d8b507b461f55f87e17f1bb619f926"
}

@test "setup_kubectl correctly detects whether kubectl is available or not" {
	PATH='' run setup_kubectl
	assert_failure
	assert_output --partial "Cannot find 'kubectl'"

	function kubectl() { return 0; }
	export -f kubectl

	PATH='' run setup_kubectl
	assert_success
	assert_line --partial "Kubectl correctly found"
}

@test "setup_kubectl correctly detects the KUBECONFIG" {
	function kubectl() { return 0; }
	export -f kubectl

	# Should use the default value
	run setup_kubectl
	assert_success
	assert_line --partial "Using KUBECONFIG: ~/.kube/config"

	# Should use the specified value
	KUBECONFIG=/test/kubeconfig
	run setup_kubectl
	assert_success
	assert_line --partial "Using KUBECONFIG: ${KUBECONFIG}"
}

@test "setup_kubectl correctly detects the KUBECONFIG context" {
	function kubectl() { [ "$*" == "config current-context" ] && echo "mocked"; }
	export -f kubectl

	# Should use the value returned by kubectl
	run setup_kubectl
	assert_success
	assert_line --partial "Using context: mocked"

	# Should correctly configure the KUBECTL variable
	# Executing again since run does not allow side-effectss
	unset KUBECTL
	assert setup_kubectl
	assert_equal "${KUBECTL}" "kubectl --context mocked"

	# Should use the specified value
	KUBECONFIG_CONTEXT="custom"
	run setup_kubectl
	assert_success
	assert_line --partial "Using context: ${KUBECONFIG_CONTEXT}"

	# Should correctly configure the KUBECTL variable
	# Executing again since run does not allow side-effects
	unset KUBECTL
	assert setup_kubectl
	assert_equal "${KUBECTL}" "kubectl --context ${KUBECONFIG_CONTEXT}"

	# kubectl which fails when invoked
	function kubectl() { return 1; }
	export -f kubectl

	# Should fail, since kubectl fails
	run setup_kubectl
	assert_failure
	assert_line --partial "Failed to retrieve the current context"
}


@test "setup_tmpdir correctly creates the temporary directory" {
	setup_tmpdir
	assert [ -d "${TMPDIR}" ]
	assert [ -d "${BINDIR}" ]
}

@test "configure_namespace correctly reads the environment variable" {
	LIQO_NAMESPACE=custom

	# Assert that the output is correct
	INSTALL_LIQO=true run configure_namespace
	assert_success
	assert_output --partial "Using namespace: ${LIQO_NAMESPACE}"

	# Run again with side effects, to assert that the variable is not modified
	INSTALL_LIQO=true configure_namespace
	assert_equal "${LIQO_NAMESPACE}" "custom"
}


@test "configure_namespace correctly uses the default value" {
	# Should use the default value (liqo)
	unset LIQO_NAMESPACE

	# Assert that the output is correct
	INSTALL_LIQO=true run configure_namespace
	assert_success
	assert_output --partial "Using namespace: liqo"

	# Run again with side effects, to assert that the variable is correctly set
	INSTALL_LIQO=true configure_namespace
	assert_equal "${LIQO_NAMESPACE}" "liqo"
}

@test "configure_installation_variables correctly reads the environment variables" {
	CLUSTER_NAME="Liqo Cluster"
	POD_CIDR="10.10.0.0/16"
	SERVICE_CIDR="20.20.0.0/16"

	# Assert that the output is correct
	run configure_installation_variables
	assert_success
	assert_line --partial "Cluster name: ${CLUSTER_NAME}"
	assert_line --partial "Pod CIDR: ${POD_CIDR}"
	assert_line --partial "Service CIDR: ${SERVICE_CIDR}"

	# Run again with side effects, to assert that the variables are not modified
	configure_installation_variables
	assert_equal "${CLUSTER_NAME}" "Liqo Cluster"
	assert_equal "${POD_CIDR}" "10.10.0.0/16"
	assert_equal "${SERVICE_CIDR}" "20.20.0.0/16"
}

@test "configure_installation_variables correctly retrieves the default values" {
	# Mocked implementation of kubectl
	KUBECTL_ARG="get pods --namespace kube-system --selector component=kube-controller-manager"
	KUBECTL_OUT='["kube-controller-manager","--allocate-node-cidrs=true","--bind-address=0.0.0.0",...,'
	KUBECTL_OUT=${KUBECTL_OUT}'"--cluster-cidr=172.16.0.0/16",...,"--service-cluster-ip-range=10.96.0.0/12"]'
	function kubectl() { [[ "$*" =~ ^${KUBECTL_ARG} ]] && echo "${KUBECTL_OUT}"; }
	export -f kubectl

	# Ensure the variables are not set
	unset CLUSTER_NAME
	unset POD_CIDR
	unset SERVICE_CIDR

	# Should be set by setup_kubectl
	KUBECTL=kubectl

	# Assert that the output is correct
	run configure_installation_variables
	assert_success
	assert_line --regexp "Cluster name: LiqoCluster[0-9]{4}"
	assert_line --partial "Pod CIDR: 172.16.0.0/16"
	assert_line --partial "Service CIDR: 10.96.0.0/12"

	# Run again with side effects, to assert that the variables are not modified
	configure_installation_variables
	[[ "${CLUSTER_NAME}" =~ ^LiqoCluster[0-9]{4}$ ]]
	assert_equal "${POD_CIDR}" "172.16.0.0/16"
	assert_equal "${SERVICE_CIDR}" "10.96.0.0/12"
}

@test "configure_installation_variables correctly fails if kubectl fails" {
	function kubectl() { return 1; }
	export -f kubectl

	# Ensure the variables are not set
	unset CLUSTER_NAME
	unset POD_CIDR
	unset SERVICE_CIDR

	# Should be set by setup_kubectl
	KUBECTL=kubectl

	run configure_installation_variables
	assert_failure
	assert_line --partial "Failed to automatically retrieve the Pod CIDR."
}


@test "setup_arch_and_os fails if ARCH is unknown" {
    function uname() { echo 'does_not_exist'; }
    export -f uname

    run setup_arch_and_os
    assert_failure 
    assert_line --partial "[PRE-FLIGHT] architecture 'does_not_exist' unknown [FATAL]"
}

@test "setup_arch_and_os fails if combination arch and os is not supported" {
    function uname() { if [ "$*" == "-m" ] ; then echo "armv7"; else echo "mingw"; fi; }
    export -f uname

    run setup_arch_and_os
    assert_failure
    assert_line --partial "[PRE-FLIGHT] System 'windows-arm' not supported."
}

@test "setup_arch_and_os success if combination arch and os is supported" {
    function uname() { if [ "$*" == "-m" ] ; then echo "x86_64"; else echo "linux"; fi; }
    export -f uname

    run setup_arch_and_os
    assert_success
}

