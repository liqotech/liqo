FROM golang:1.16 as builder
ENV PATH /go/bin:/usr/local/go/bin:/usr/local/kubebuilder/bin:$PATH
ENV GOPATH /go
WORKDIR /go/src/github.com/liqotech/liqo
COPY go.mod ./go.mod
COPY go.sum ./go.sum
RUN  go mod download

# Install kubebuilder
RUN curl -sL https://go.kubebuilder.io/dl/2.3.0/$(go env GOOS)/$(go env GOARCH) | tar -xz -C /tmp/
RUN mv /tmp/kubebuilder_2.3.0_$(go env GOOS)_$(go env GOARCH) /usr/local/kubebuilder

# Install iptables
RUN apt-get update
RUN apt-get install iptables -y

# Install goimports
RUN GO111MODULE="on" go get -u github.com/ory/go-acc

ENTRYPOINT go-acc ./... --ignore liqo/test/e2e
