FROM golang:1.16 as builder
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go
WORKDIR /go/src/github.com/liqotech/liqo
COPY go.mod ./go.mod
COPY go.sum ./go.sum
RUN  go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) go build ./cmd/webhook-configuration/
RUN cp webhook-configuration /usr/bin/webhook-configuration

FROM alpine:3.13.2
COPY --from=builder /usr/bin/webhook-configuration /usr/bin/webhook-configuration
ENTRYPOINT [ "/usr/bin/webhook-configuration" ]
