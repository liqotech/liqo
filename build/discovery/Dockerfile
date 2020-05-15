FROM golang:1.13 as builder
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go
COPY . /go/src/github.com/netgroup-polito/dronev2
WORKDIR /go/src/github.com/netgroup-polito/dronev2
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/discovery/
RUN cp discovery /usr/bin/discovery

FROM scratch
COPY --from=builder /usr/bin/discovery /usr/bin/discovery
ENTRYPOINT [ "/usr/bin/discovery" ]
