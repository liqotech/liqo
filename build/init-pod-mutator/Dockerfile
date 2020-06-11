FROM alpine:3.9.5
RUN apk add --update curl bash
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/v1.17.0/bin/linux/amd64/kubectl && chmod +x ./kubectl && cp kubectl /usr/bin/kubectl

COPY scripts/pod-mutator-webhook/pod-mutator-webhook-deployment.sh /usr/bin/local/pod-mutator-webhook-deployment.sh
RUN mkdir /root/.kube
RUN chmod +x "/usr/bin/local/pod-mutator-webhook-deployment.sh"

ENTRYPOINT [ "/usr/bin/local/pod-mutator-webhook-deployment.sh" ]
