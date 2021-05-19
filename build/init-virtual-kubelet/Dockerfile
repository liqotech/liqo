FROM golang:1.16 as builder
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go
WORKDIR /go/src/github.com/liqotech/liqo
COPY go.mod ./go.mod
COPY go.sum ./go.sum
RUN  go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) go build ./cmd/init-virtual-kubelet/
RUN cp init-virtual-kubelet /usr/bin/init-virtual-kubelet


FROM alpine:3.13.2
COPY --from=builder /usr/bin/init-virtual-kubelet /usr/bin/init-virtual-kubelet
ENTRYPOINT [ "/usr/bin/init-vrtual-kubelet" ]
