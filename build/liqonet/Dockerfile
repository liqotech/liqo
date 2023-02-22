FROM ekidd/rust-musl-builder as rustBuilder

ARG VERSION=0.5.2

RUN cargo install --version $VERSION boringtun-cli


FROM golang:1.20 as goBuilder
WORKDIR /tmp/builder

COPY go.mod ./go.mod
COPY go.sum ./go.sum
RUN  go mod download

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) go build -ldflags="-s -w" ./cmd/liqonet


FROM alpine:3.15

RUN apk update && \
    apk add iptables bash wireguard-tools tcpdump conntrack-tools curl && \
    rm -rf /var/cache/apk/*

COPY --from=goBuilder /tmp/builder/liqonet /usr/bin/liqonet
COPY --from=rustBuilder /home/rust/.cargo/bin/boringtun-cli /usr/bin/boringtun-cli

ENTRYPOINT [ "/usr/bin/liqonet" ]
