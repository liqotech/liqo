FROM golang:1.21 as goBuilder
WORKDIR /tmp/builder

COPY go.mod ./go.mod
COPY go.sum ./go.sum
RUN  go mod download

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) go build -ldflags="-s -w" ./cmd/gateway/wireguard


FROM alpine:3.18

RUN apk update && \
    apk add nftables bash wireguard-tools tcpdump conntrack-tools curl iputils && \
    rm -rf /var/cache/apk/*

COPY --from=goBuilder /tmp/builder/wireguard /usr/bin/liqo-wireguard

ENTRYPOINT [ "/usr/bin/liqo-wireguard" ]
