FROM alpine:3.9.5
RUN apk add --update curl bash
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/v1.17.0/bin/linux/amd64/kubectl && chmod +x ./kubectl && cp kubectl /usr/bin/kubectl
RUN curl -LO https://github.com/cloudflare/cfssl/releases/download/v1.4.1/cfssl_1.4.1_linux_amd64 && chmod +x ./cfssl_1.4.1_linux_amd64 && cp cfssl_1.4.1_linux_amd64 /usr/bin/cfssl
RUN curl -LO https://github.com/cloudflare/cfssl/releases/download/v1.4.1/cfssljson_1.4.1_linux_amd64 && chmod +x ./cfssljson_1.4.1_linux_amd64 && cp cfssljson_1.4.1_linux_amd64 /usr/bin/cfssljson
COPY config/virtual-kubelet/script.sh /usr/bin/local/kubelet-setup.sh
RUN mkdir /root/.kube
ENTRYPOINT [ "/usr/bin/local/kubelet-setup.sh" ]
