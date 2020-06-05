FROM golang:1.13 as builder
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go
COPY . /go/src/github.com/liqoTech/liqo
WORKDIR /go/src/github.com/liqoTech/liqo
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/federation-request-operator/
RUN cp federation-request-operator /usr/bin/federation-request-operator

FROM scratch
COPY --from=builder /usr/bin/federation-request-operator /usr/bin/federation-request-operator
ENTRYPOINT [ "/usr/bin/federation-request-operator" ]
