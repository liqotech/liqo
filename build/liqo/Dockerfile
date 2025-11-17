
# syntax=docker/dockerfile:1
FROM alpine:3.20
ARG COMPONENT
ARG TARGETARCH

RUN if [ "$COMPONENT" = "geneve" ] || [ "$COMPONENT" = "wireguard" ] || [ "$COMPONENT" = "gateway" ]; then \
    set -x; \
    apk add --no-cache iproute2 nftables bash wireguard-tools tcpdump conntrack-tools curl iputils; \
    fi

RUN if [ "$COMPONENT" = "wireguard" ]; then \
    set -e; \
    apk add --no-cache git curl; \
    GO_VERSION=1.22.4; \
    # Map Docker TARGETARCH to Go architecture names \
    case "$TARGETARCH" in \
        amd64) GO_ARCH=amd64 ;; \
        arm64) GO_ARCH=arm64 ;; \
        arm) GO_ARCH=armv6l ;; \
        *) echo "Unsupported architecture: $TARGETARCH" && exit 1 ;; \
    esac; \
    curl -L https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz | tar -C /usr/local -xz; \
    export PATH="/usr/local/go/bin:$PATH"; \
    git clone https://git.zx2c4.com/wireguard-go; cd wireguard-go; git checkout f333402bd9cbe0f3eeb02507bd14e23d7d639280; \
    CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH /usr/local/go/bin/go build -ldflags="-s -w" -o /usr/bin/wireguard-go; \
    cd .. && rm -rf wireguard-go; \
    rm -rf /usr/local/go; \
    apk del git curl; \
    fi

# Copy the correct binary for the architecture
COPY ./bin/${TARGETARCH}/${COMPONENT}_linux_${TARGETARCH} /usr/bin/${COMPONENT}
RUN chmod +x /usr/bin/${COMPONENT}
RUN ln -s /usr/bin/${COMPONENT} /usr/bin/liqo-component

WORKDIR /workspace

ENTRYPOINT ["/usr/bin/liqo-component"]
