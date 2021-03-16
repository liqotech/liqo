FROM golang:1.14 as builder
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go
COPY . /go/src/github.com/liqotech/liqo
WORKDIR /go/src/github.com/liqotech/liqo
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) go build ./cmd/advertisement-broadcaster/
RUN cp advertisement-broadcaster /usr/bin/advertisement-broadcaster

FROM alpine:3.13.2
COPY --from=builder /usr/bin/advertisement-broadcaster /usr/bin/advertisement-broadcaster
ENTRYPOINT [ "/usr/bin/advertisement-broadcaster" ]
