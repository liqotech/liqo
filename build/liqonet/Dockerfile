FROM ekidd/rust-musl-builder as rustBuilder
WORKDIR /home/rust/src
RUN git clone https://github.com/cloudflare/boringtun.git
WORKDIR /home/rust/src/boringtun
RUN rustup target add x86_64-unknown-linux-musl
RUN cargo build --bin boringtun-cli --release


FROM golang:1.17 as goBuilder
WORKDIR /tmp/builder

COPY go.mod ./go.mod
COPY go.sum ./go.sum
RUN  go mod download

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) go build -ldflags="-s -w" ./cmd/liqonet


FROM alpine:3.14

RUN apk update && \
    apk add iptables bash wireguard-tools tcpdump && \
    rm -rf /var/cache/apk/*

COPY --from=goBuilder /tmp/builder/liqonet /usr/bin/liqonet
COPY --from=rustBuilder /home/rust/src/boringtun/target/x86_64-unknown-linux-musl/release/boringtun-cli /usr/bin/boringtun

ENTRYPOINT [ "/usr/bin/liqonet" ]
