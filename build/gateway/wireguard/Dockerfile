FROM golang:1.24 AS gobuilder-wg
WORKDIR /tmp/builder

ARG ref=12269c2761734b15625017d8565745096325392f
RUN git clone https://git.zx2c4.com/wireguard-go && cd wireguard-go && git checkout $ref && \
    CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) go build -ldflags="-s -w" -o wireguard-go


FROM golang:1.24 AS gobuilder
WORKDIR /tmp/builder

COPY go.mod ./go.mod
COPY go.sum ./go.sum
RUN  go mod download

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) go build -ldflags="-s -w" ./cmd/gateway/wireguard


FROM alpine:3.21.3

RUN apk update && \
    apk add iproute2 nftables bash wireguard-tools tcpdump conntrack-tools curl iputils && \
    rm -rf /var/cache/apk/*

COPY --from=gobuilder /tmp/builder/wireguard /usr/bin/liqo-wireguard

COPY --from=gobuilder-wg /tmp/builder/wireguard-go/wireguard-go /usr/bin/wireguard-go

ENTRYPOINT [ "/usr/bin/liqo-wireguard" ]
