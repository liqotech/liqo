FROM golang:1.14 as builder

ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go
COPY . /go/src/github.com/liqotech/liqo
WORKDIR /go/src/github.com/liqotech/liqo
ARG BUILD_TAGS=""
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) go build ./cmd/init-virtual-kubelet/
RUN cp init-virtual-kubelet /usr/bin/init-virtual-kubelet


FROM alpine:3.13.2
COPY --from=builder /usr/bin/init-virtual-kubelet /usr/bin/init-virtual-kubelet
ENTRYPOINT [ "/usr/bin/init-vrtual-kubelet" ]
