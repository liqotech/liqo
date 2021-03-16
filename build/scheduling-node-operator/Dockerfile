FROM golang:1.14 as builder
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go
COPY . /go/src/github.com/liqotech/liqo
WORKDIR /go/src/github.com/liqotech/liqo
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) go build ./cmd/schedulingNodeOperator/
RUN cp schedulingNodeOperator /usr/bin/schedulingNodeOperator

FROM alpine:3.13.2
COPY --from=builder /usr/bin/schedulingNodeOperator /usr/bin/schedulingNodeOperator
ENTRYPOINT [ "/usr/bin/schedulingNodeOperator" ]
