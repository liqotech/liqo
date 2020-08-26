FROM golang:1.14 as builder
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go
COPY . /go/src/github.com/liqoTech/liqo
WORKDIR /go/src/github.com/liqoTech/liqo
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/pod-mutator/
RUN cp pod-mutator /usr/bin/pod-mutator

FROM scratch
COPY --from=builder /usr/bin/pod-mutator /usr/bin/pod-mutator
ENTRYPOINT [ "/usr/bin/pod-mutator" ]
