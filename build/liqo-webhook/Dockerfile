FROM golang:1.14 as builder
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go
COPY . /go/src/github.com/liqotech/liqo
WORKDIR /go/src/github.com/liqotech/liqo
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) go build ./cmd/liqo-webhook/
RUN cp liqo-webhook /usr/bin/liqo-webhook

FROM ubuntu
COPY --from=builder /usr/bin/liqo-webhook /usr/bin/liqo-webhook
ENTRYPOINT [ "/usr/bin/liqo-webhook" ]
