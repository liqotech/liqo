FROM golang:1.25.5

ARG ENVTEST_VERSION=release-0.21
ARG ENVTEST_K8S_VERSION=1.30.x

WORKDIR /go/src/github.com/liqotech/liqo
COPY go.mod ./go.mod
COPY go.sum ./go.sum
RUN  go mod download

# Install setup-envtest and envtest binaries
RUN go install sigs.k8s.io/controller-runtime/tools/setup-envtest@${ENVTEST_VERSION}
RUN mkdir -p /usr/local/kubebuilder/bin && \
    ASSETS_PATH=$($(go env GOPATH)/bin/setup-envtest use ${ENVTEST_K8S_VERSION} \
      --bin-dir=/usr/local/kubebuilder/bin -p path) && \
    ln -sf ${ASSETS_PATH}/* /usr/local/kubebuilder/bin/

# Install iptables
RUN apt-get update && apt-get install iptables iproute2 -y

# Fix for go version 1.19.5
ENV GOFLAGS=-buildvcs=false

ENTRYPOINT [ "go", "test" ]

# Remove the -count=1 flag to enable caching of successful tests
# Remove the -failfast flag to run all tests even if some fail
# Remove the -shuffle flag to run tests in the order they are defined
# Add the -v flag to increase verbosity
CMD [ "./cmd/...", "./pkg/...", "./apis/...", "./internal/...", "-count=1", "-failfast", "-shuffle=on" ]
