FROM golang:1.21 as goBuilder
WORKDIR /tmp/builder

COPY go.mod ./go.mod
COPY go.sum ./go.sum
RUN  go mod download

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) go build -ldflags="-s -w" ./cmd/gateway


FROM alpine:3.18

RUN apk update && \
    apk add nftables bash tcpdump conntrack-tools curl iputils && \
    rm -rf /var/cache/apk/*
    
# Alpine do not have /etc/iproute2/rt_tables by default.
# We need to create it to associate the custom routing tables IDs with names.
RUN mkdir /etc/iproute2 && touch /etc/iproute2/rt_tables

COPY --from=goBuilder /tmp/builder/gateway /usr/bin/liqo-gateway

ENTRYPOINT [ "/usr/bin/liqo-gateway" ]
