FROM golang:1.14 as builder
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go
COPY . /go/src/github.com/liqotech/liqo
WORKDIR /go/src/github.com/liqotech/liqo
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/schedulingNode-operator/
RUN cp schedulingNode-operator /usr/bin/schedulingNode-operator

FROM scratch
COPY --from=builder /usr/bin/schedulingNode-operator /usr/bin/schedulingNode-operator
ENTRYPOINT [ "/usr/bin/schedulingNode-operator" ]
