FROM golang:1.13 as builder
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go
COPY . /go/src/github.com/liqoTech/liqo
WORKDIR /go/src/github.com/liqoTech/liqo
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/dispatcher/
RUN cp dispatcher /usr/bin/dispatcher

FROM scratch
COPY --from=builder /usr/bin/dispatcher /usr/bin/dispatcher
ENTRYPOINT [ "/usr/bin/dispatcher" ]