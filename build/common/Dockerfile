FROM golang:1.24 AS builder
WORKDIR /tmp/builder

COPY go.mod ./go.mod
COPY go.sum ./go.sum
RUN  go mod download

ARG COMPONENT
RUN test -n "$COMPONENT" || ( echo "The COMPONENT argument is unset. Aborting" && false )

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) go build -ldflags="-s -w" ./cmd/$COMPONENT


FROM alpine:3.21.3

RUN apk update && \
    apk add --no-cache ca-certificates && \
    update-ca-certificates && \
    rm -rf /var/cache/apk/*

ARG COMPONENT
COPY --from=builder /tmp/builder/$COMPONENT /usr/bin/$COMPONENT
RUN ln -s /usr/bin/$COMPONENT /usr/bin/liqo-component

ENTRYPOINT [ "/usr/bin/liqo-component" ]
