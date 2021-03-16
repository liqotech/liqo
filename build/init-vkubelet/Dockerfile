FROM alpine:3.13.2
RUN apk add --update --no-cache curl bash && \
   apk add --update --no-cache -X http://dl-cdn.alpinelinux.org/alpine/edge/testing cfssl && \
  rm -rf /var/cache/apk/
COPY scripts/virtual-kubelet/dep.sh /usr/bin/local/dep.sh
RUN /usr/bin/local/dep.sh
COPY scripts/virtual-kubelet/script.sh /usr/bin/local/kubelet-setup.sh
RUN mkdir /root/.kube
ENTRYPOINT [ "/usr/bin/local/kubelet-setup.sh" ]
