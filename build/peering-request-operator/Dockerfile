FROM golang:1.14 as builder
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go
COPY . /go/src/github.com/liqotech/liqo
WORKDIR /go/src/github.com/liqotech/liqo
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) go build ./cmd/peering-request-operator/
RUN cp peering-request-operator /usr/bin/peering-request-operator

FROM alpine:3.13.2
COPY --from=builder /usr/bin/peering-request-operator /usr/bin/peering-request-operator
ENTRYPOINT [ "/usr/bin/peering-request-operator" ]
