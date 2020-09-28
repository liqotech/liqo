FROM golang:1.14 as builder

ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go
COPY . /go/src/github.com/liqotech/liqo
WORKDIR /go/src/github.com/liqotech/liqo

ARG BUILD_TAGS=""
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/virtual-kubelet/
RUN cp virtual-kubelet /usr/bin/virtual-kubelet
RUN cp go.mod /usr/local/

FROM busybox
COPY --from=builder /usr/bin/virtual-kubelet /usr/bin/virtual-kubelet
COPY --from=builder /usr/local/go.mod /usr/local/go.mod
COPY --from=builder /etc/ssl/certs/ /etc/ssl/certs
ENTRYPOINT [ "/usr/bin/virtual-kubelet" ]
CMD [ "--help" ]
