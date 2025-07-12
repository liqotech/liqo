#!/usr/bin/env bash

set -e
set -o nounset
set -o pipefail

usage() {
    echo "Usage: $0 [-m] [-p] <component-folder>"
    echo "  -p    Push the built image to the registry"
    echo "  -h    Show this help message"
}

if [ $# -ne 1 ]; then
    usage
    exit 1
fi

componentdir="$1"
if [ ! -d "$componentdir" ]; then
    echo "Error: $componentdir is not a directory" >&2
    exit 1
fi

component=$(basename "$componentdir")

# Set registry/org/tag if not already defined
DOCKER_REGISTRY="${DOCKER_REGISTRY:-ghcr.io}"
DOCKER_ORGANIZATION="${DOCKER_ORGANIZATION:-liqotech}"
DOCKER_TAG="${DOCKER_TAG:-latest}"
DOCKER_PUSH="${DOCKER_PUSH:-true}"
ARCHS="${ARCHS:-linux/amd64,linux/arm64}"

echo "Downloading Go modules..."
start_time=$(date +%s)
go mod download
end_time=$(date +%s)
elapsed=$((end_time - start_time))
echo "Go modules downloaded in ${elapsed} seconds."

IFS=',' read -ra arch_array <<<"$ARCHS"
for arch in "${arch_array[@]}"; do
    arch_no_os=${arch#linux/} # Remove 'linux/' prefix
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
    if CGO_ENABLED=0 go build -ldflags="-s -w" -o "./bin/${arch_no_os}/${component}_${os}_${arch_no_os}" "$componentdir"; then
        echo "  Built $component for $arch."
    else
        echo "  Failed to build $component for $arch" >&2
    fi
done

if [[ "$component" == "geneve" || "$component" == "wireguard" ]]; then
    image_component="gateway/${component}"
else
    image_component="${component}"
fi

if [[ "$DOCKER_TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+ ]]; then
    image_tag="${DOCKER_REGISTRY}/$DOCKER_ORGANIZATION/${image_component}:${DOCKER_TAG}"
else
    image_tag="${DOCKER_REGISTRY}/${DOCKER_ORGANIZATION}/${image_component}-ci:${DOCKER_TAG}"
fi
echo "Building container image $image_tag for architectures $ARCHS..."
docker buildx build --platform "${ARCHS}" \
    --build-arg COMPONENT="$component" \
    -t "$image_tag" -f ./build/liqo/Dockerfile . \
    "$(if $DOCKER_PUSH; then echo --push; else echo --load; fi)"
