FROM golang:1.16 as builder
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go
COPY . /go/src/github.com/liqotech/liqo
WORKDIR /go/src/github.com/liqotech/liqo
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) go build ./cmd/crd-replicator/
RUN cp crd-replicator /usr/bin/crd-replicator

FROM alpine:3.13.2
COPY --from=builder /usr/bin/crd-replicator /usr/bin/crd-replicator
ENTRYPOINT [ "/usr/bin/crd-replicator" ]