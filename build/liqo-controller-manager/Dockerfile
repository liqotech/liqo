FROM alpine:3.9.5
RUN apk add --update curl bash openssl
RUN <<EOR
  case "`uname -i`" in
  x86_64|amd64)
    curl -LO https://storage.googleapis.com/kubernetes-release/release/v1.20.0/bin/linux/amd64/kubectl && chmod +x ./kubectl && cp kubectl /usr/bin/kubectl
  arm*|aarch64)
    curl -LO https://storage.googleapis.com/kubernetes-release/release/v1.20.0/bin/linux/arm64/kubectl && chmod +x ./kubectl && cp kubectl /usr/bin/kubectl
  esac
EOR

COPY scripts/webhook-installer/webhook-installer.sh /usr/bin/local/webhook-installer.sh
RUN mkdir /root/.kube
ENTRYPOINT [ "/usr/bin/local/webhook-installer.sh" ]
