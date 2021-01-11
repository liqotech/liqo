FROM ekidd/rust-musl-builder as rustBuilder
WORKDIR /home/rust/src
RUN git clone https://github.com/cloudflare/boringtun.git
WORKDIR boringtun
RUN rustup target add x86_64-unknown-linux-musl
RUN cargo build --bin boringtun --release

FROM golang:1.14-alpine AS goBuilder
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go
RUN apk update && apk add git make
COPY . /go/src/github.com/liqotech/liqo
WORKDIR /go/src/github.com/liqotech/liqo
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/liqonet/
RUN cp liqonet /usr/bin/liqonet

FROM alpine
RUN apk update && apk add iptables bash wireguard-tools tcpdump
COPY --from=goBuilder /usr/bin/liqonet /usr/bin/liqonet
COPY --from=rustBuilder /home/rust/src/boringtun/target/x86_64-unknown-linux-musl/release/boringtun /usr/bin/boringtun
ENTRYPOINT [ "/usr/bin/liqonet" ]