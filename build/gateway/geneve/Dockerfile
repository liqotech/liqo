FROM golang:1.24 AS gobuilder
WORKDIR /tmp/builder

COPY go.mod ./go.mod
COPY go.sum ./go.sum
RUN  go mod download

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) go build -ldflags="-s -w" ./cmd/gateway/geneve


FROM alpine:3.21.3

RUN apk update && \
    apk add iproute2 nftables bash tcpdump conntrack-tools curl iputils && \
    rm -rf /var/cache/apk/*

COPY --from=gobuilder /tmp/builder/geneve /usr/bin/liqo-geneve

ENTRYPOINT [ "/usr/bin/liqo-geneve" ]
