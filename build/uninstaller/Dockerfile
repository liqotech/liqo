FROM golang:1.16 as builder
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go
COPY . /go/src/github.com/liqotech/liqo
WORKDIR /go/src/github.com/liqotech/liqo

ARG BUILD_TAGS=""
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) go build ./cmd/uninstaller/
RUN cp uninstaller /usr/bin/uninstaller

FROM alpine:3.13.2
COPY --from=builder /usr/bin/uninstaller /usr/bin/uninstaller
ENTRYPOINT [ "/usr/bin/uninstaller" ]

