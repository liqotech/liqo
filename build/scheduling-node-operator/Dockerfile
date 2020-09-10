FROM golang:1.14 as builder
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go
COPY . /go/src/github.com/liqotech/liqo
WORKDIR /go/src/github.com/liqotech/liqo
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/schedulingNodeOperator/
RUN cp schedulingNodeOperator /usr/bin/schedulingNodeOperator

FROM scratch
COPY --from=builder /usr/bin/schedulingNodeOperator /usr/bin/schedulingNodeOperator
ENTRYPOINT [ "/usr/bin/schedulingNodeOperator" ]
