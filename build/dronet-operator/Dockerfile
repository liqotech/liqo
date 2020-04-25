FROM golang:alpine as builder
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go
COPY . /go/src/github.com/netgroup-polito/dronev2
WORKDIR /go/src/github.com/netgroup-polito/dronev2
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/dronet-operator/
RUN cp dronet-operator /usr/bin/dronet-operator

FROM alpine
RUN apk update && apk add iptables && apk add bash
COPY --from=builder /usr/bin/dronet-operator /usr/bin/dronet-operator
ENTRYPOINT [ "/usr/bin/dronet-operator" ]