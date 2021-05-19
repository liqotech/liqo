FROM golang:1.16 as builder
ENV PATH /go/bin:/usr/local/go/bin:/usr/local/kubebuilder/bin:$PATH
ENV GOPATH /go

# Install kubebuilder
RUN curl -sL https://go.kubebuilder.io/dl/2.3.0/$(go env GOOS)/$(go env GOARCH) | tar -xz -C /tmp/
RUN mv /tmp/kubebuilder_2.3.0_$(go env GOOS)_$(go env GOARCH) /usr/local/kubebuilder

# Install goimports
RUN GO111MODULE="on" go get golang.org/x/tools/cmd/goimports@v0.1.1

ENTRYPOINT go test -cover $(go list ./... | grep -v "e2e")
