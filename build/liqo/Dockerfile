
# syntax=docker/dockerfile:1
FROM alpine:3.20
ARG COMPONENT
ARG TARGETARCH

RUN if [ "$COMPONENT" = "geneve" ] || [ "$COMPONENT" = "wireguard" ] || [ "$COMPONENT" = "gateway" ]; then \
    set -x; \
    apk add --no-cache iproute2 nftables bash wireguard-tools tcpdump conntrack-tools curl iputils; \
    fi

# Copy the correct binary for the architecture
COPY ./bin/${TARGETARCH}/${COMPONENT}_linux_${TARGETARCH} /usr/bin/${COMPONENT}
RUN chmod +x /usr/bin/${COMPONENT}
RUN ln -s /usr/bin/${COMPONENT} /usr/bin/liqo-component

WORKDIR /workspace

ENTRYPOINT ["/usr/bin/liqo-component"]
