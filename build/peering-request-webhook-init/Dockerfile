FROM alpine:3.9.5
RUN apk add --update curl bash
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/v1.17.0/bin/linux/amd64/kubectl && chmod +x ./kubectl && cp kubectl /usr/bin/kubectl

COPY scripts/peering-request-webhook-init/peering-request-webhook-init.sh /usr/bin/local/peering-request-webhook-init.sh
RUN mkdir /root/.kube
RUN chmod +x "/usr/bin/local/peering-request-webhook-init.sh"

ENTRYPOINT [ "/usr/bin/local/peering-request-webhook-init.sh" ]
