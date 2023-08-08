FROM rust:1.70.0 as rustBuilder

ARG VERSION=0.5.2

RUN git clone https://github.com/cloudflare/boringtun.git

WORKDIR /boringtun

RUN cargo build --bin boringtun-cli --release


FROM golang:1.20 as goBuilder
WORKDIR /tmp/builder

COPY go.mod ./go.mod
COPY go.sum ./go.sum
RUN  go mod download

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) go build -ldflags="-s -w" ./cmd/liqonet


FROM debian:11.7-slim

RUN apt-get update && \
    apt-get install -y iproute2 iptables bash wireguard-tools tcpdump conntrack curl iputils-ping && \
    apt-get clean autoclean && \
    apt-get autoremove -y && \
    rm -rf /var/lib/apt/lists/*

COPY --from=goBuilder /tmp/builder/liqonet /usr/bin/liqonet
COPY --from=rustBuilder /boringtun/target/release/boringtun-cli /usr/bin/boringtun-cli

ENTRYPOINT [ "/usr/bin/liqonet" ]
