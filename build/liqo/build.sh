#!/usr/bin/env bash

# Example usage
# DOCKER_ORGANIZATION=<your-org> DOCKER_PUSH=false DOCKER_TAG=test-build-1 ARCHS=linux/amd64 ./build/liqo/build.sh all

set -e
set -o nounset
set -o pipefail

# -----------------------------------------------------------------------------
# Configuration
# -----------------------------------------------------------------------------

DOCKER_REGISTRY="${DOCKER_REGISTRY:-ghcr.io}"
DOCKER_ORGANIZATION="${DOCKER_ORGANIZATION:-liqotech}"
DOCKER_TAG="${DOCKER_TAG:-latest}"
DOCKER_PUSH="${DOCKER_PUSH:-true}"
ARCHS="${ARCHS:-linux/amd64,linux/arm64}"

# All components built by this script, matching the build-go matrix in
# .github/workflows/integration.yml.
ALL_COMPONENTS=(
	./cmd/crd-replicator
	./cmd/ipam
	./cmd/liqo-controller-manager
	./cmd/webhook
	./cmd/uninstaller
	./cmd/virtual-kubelet
	./cmd/metric-agent
	./cmd/telemetry
	./cmd/proxy
	./cmd/gateway
	./cmd/gateway/wireguard
	./cmd/gateway/geneve
	./cmd/fabric
)

usage() {
	echo "Usage: $0 <component-folder> [<component-folder> ...]"
	echo "       Each argument must be a path to a cmd subdirectory (e.g. ./cmd/liqo-controller-manager),"
	echo "       or the special keyword 'all' to build every component."
	echo ""
	echo "Environment variables:"
	echo "  DOCKER_REGISTRY    Container registry (default: ghcr.io)"
	echo "  DOCKER_ORGANIZATION  Organization in the registry (default: liqotech)"
	echo "  DOCKER_TAG         Image tag (default: latest)"
	echo "  DOCKER_PUSH        Push the image after build (default: true)"
	echo "  ARCHS              Comma-separated list of linux architectures (default: linux/amd64,linux/arm64)"
	exit 1
}

# Build Go binaries for a single component across all target architectures.
# Args: $1 = component directory (e.g. ./cmd/webhook)
build_binaries() {
	local componentdir="$1"
	local component
	component=$(basename "$componentdir")

	local arch_array
	IFS=',' read -ra arch_array <<<"$ARCHS"

	for arch in "${arch_array[@]}"; do
		local arch_no_os=${arch#linux/} # Remove 'linux/' prefix
		local os
		os=$(echo "$arch" | cut -d'/' -f1)

		if [ "$os" != "linux" ]; then
			echo "Error: Only 'linux' OS is supported. Found: $os" >&2
			exit 1
		fi

		export GOOS="${os}"
		export GOARCH="${arch_no_os}"

		if [ "${arch_no_os}" == "arm/v7" ]; then
			export GOARM=7
			export GOARCH=arm
			arch_no_os="arm"
		fi

		mkdir -p "./bin/$arch_no_os"
		echo "Building $component for $arch..."
		if CGO_ENABLED=0 go build -ldflags="-s -w" \
			-o "./bin/${arch_no_os}/${component}_${os}_${arch_no_os}" "$componentdir"; then
			echo "  Built $component for $arch."
		else
			echo "  Failed to build $component for $arch" >&2
			return 1
		fi
	done
}

# Compute the full image tag for a component.
# Release tags (v1.2.3) skip the -ci suffix; everything else gets -ci.
# Args: $1 = component name (e.g. webhook, geneve, wireguard)
# Echoes: <registry>/<org>/<image_component>[-ci]:<tag>
compute_image_tag() {
	local component="$1"
	local image_component

	if [[ "$component" == "geneve" || "$component" == "wireguard" ]]; then
		image_component="gateway/${component}"
	else
		image_component="${component}"
	fi

	if [[ "$DOCKER_TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+ ]]; then
		echo "${DOCKER_REGISTRY}/${DOCKER_ORGANIZATION}/${image_component}:${DOCKER_TAG}"
	else
		echo "${DOCKER_REGISTRY}/${DOCKER_ORGANIZATION}/${image_component}-ci:${DOCKER_TAG}"
	fi
}

# Build the container image for a component using docker buildx.
# Args: $1 = component name (used as the COMPONENT build arg)
build_container_image() {
	local component="$1"
	local image_tag
	image_tag=$(compute_image_tag "$component")

	echo "Building container image $image_tag for architectures $ARCHS..."
	docker buildx build --platform "${ARCHS}" \
		--build-arg COMPONENT="$component" \
		-t "$image_tag" -f ./build/liqo/Dockerfile . \
		"$(if $DOCKER_PUSH; then echo --push; else echo --load; fi)"
}

# Build Go binaries and container image for a single component.
# Args: $1 = component directory (e.g. ./cmd/liqo-controller-manager)
build_component() {
	local componentdir="$1"
	local component
	component=$(basename "$componentdir")

	if [ ! -d "$componentdir" ]; then
		echo "Error: $componentdir is not a directory" >&2
		return 1
	fi

	build_binaries "$componentdir"
	build_container_image "$component"
}

# Expand 'all' (in any form: all, ./all, etc.) into the full component list
# matching the build-go matrix in .github/workflows/integration.yml.
expand_components() {
	local -n _input=$1
	local -n _output=$2
	_output=()
	for arg in "${_input[@]}"; do
		local basename_arg
		basename_arg=$(basename "$arg")
		if [ "$basename_arg" == "all" ]; then
			_output+=("${ALL_COMPONENTS[@]}")
		else
			_output+=("$arg")
		fi
	done
}

# -----------------------------------------------------------------------------
# Main
# -----------------------------------------------------------------------------

if [ $# -lt 1 ]; then
	usage
fi

# Expand special 'all' keyword into the full set of components.
declare -a component_dirs
input_args=("$@")
expand_components input_args component_dirs

echo "Downloading Go modules..."
start_time=$(date +%s)
go mod download
end_time=$(date +%s)
echo "Go modules downloaded in $((end_time - start_time)) seconds."

failed=0
for componentdir in "${component_dirs[@]}"; do
	echo ""
	echo "=============================================="
	echo "  Building: $componentdir"
	echo "=============================================="
	if ! build_component "$componentdir"; then
		failed=1
	fi
done

if [ "$failed" -ne 0 ]; then
	echo ""
	echo "One or more components failed to build." >&2
	exit 1
fi

echo ""
echo "All components built successfully."
