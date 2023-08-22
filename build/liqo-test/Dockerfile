FROM golang:1.21 as builder
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go
ENV K8S_VERSION=1.25.0

# Install required go binaries
RUN go install github.com/ory/go-acc@v0.2.8

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

ENTRYPOINT [ "go-acc" ]
CMD [ "./...", "--ignore", "liqo/test/e2e", "--", "-vet=off" ]
