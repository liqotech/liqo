
# syntax=docker/dockerfile:1
FROM alpine:3.20
ARG COMPONENT
ARG TARGETARCH

# Copy the correct binary for the architecture
COPY ./bin/${TARGETARCH}/${COMPONENT}_linux_${TARGETARCH} /usr/bin/${COMPONENT}
RUN chmod +x /usr/bin/${COMPONENT}
RUN ln -s /usr/bin/${COMPONENT} /usr/bin/liqo-component

RUN if [ "$COMPONENT" = "geneve" ] || [ "$COMPONENT" = "wireguard" ] || [ "$COMPONENT" = "gateway" ]; then \
    apk add --no-cache iproute2 nftables bash wireguard-tools tcpdump conntrack-tools curl iputils; \
    fi

WORKDIR /workspace

ENTRYPOINT ["/usr/bin/liqo-component"]
