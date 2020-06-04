FROM golang:1.13 as builder
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go
COPY . /go/src/github.com/liqoTech/liqo
WORKDIR /go/src/github.com/liqoTech/liqo
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/liqonet/
RUN cp liqonet /usr/bin/liqonet

FROM alpine
RUN apk update && apk add iptables && apk add bash
COPY --from=builder /usr/bin/liqonet /usr/bin/liqonet
ENTRYPOINT [ "/usr/bin/liqonet" ]