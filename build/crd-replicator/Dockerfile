FROM golang:1.13 as builder
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go
COPY . /go/src/github.com/liqotech/liqo
WORKDIR /go/src/github.com/liqotech/liqo
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/crd-replicator/
RUN cp crd-replicator /usr/bin/crd-replicator

FROM scratch
COPY --from=builder /usr/bin/crd-replicator /usr/bin/crd-replicator
ENTRYPOINT [ "/usr/bin/crd-replicator" ]