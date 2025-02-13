FROM golang:1.24

ENV K8S_VERSION=1.30.0

WORKDIR /go/src/github.com/liqotech/liqo
COPY go.mod ./go.mod
COPY go.sum ./go.sum
RUN  go mod download

# Install kubebuilder
RUN curl --fail -sSLo envtest-bins.tar.gz "https://go.kubebuilder.io/test-tools/${K8S_VERSION}/$(go env GOOS)/$(go env GOARCH)"
RUN mkdir /usr/local/kubebuilder/
RUN tar -C /usr/local/kubebuilder/ --strip-components=1 -zvxf envtest-bins.tar.gz

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
