FROM golang:1.14 as builder
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go
COPY . /go/src/github.com/liqotech/liqo
WORKDIR /go/src/github.com/liqotech/liqo
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/auth-service/
RUN cp auth-service /usr/bin/auth-service

FROM scratch
COPY --from=builder /usr/bin/auth-service /usr/bin/auth-service
ENTRYPOINT [ "/usr/bin/auth-service" ]
