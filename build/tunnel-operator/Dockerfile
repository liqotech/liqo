FROM golang:1.13 as builder
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go
COPY . /go/src/github.com/netgroup-polito/dronev2
WORKDIR /go/src/github.com/netgroup-polito/dronev2
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/tunnel-operator/
RUN cp tunnel-operator /usr/bin/tunnel-operator

FROM scratch
COPY --from=builder /usr/bin/tunnel-operator /usr/bin/tunnel-operator
ENTRYPOINT [ "/usr/bin/tunnel-operator" ]

# Build
#RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o manager main.go
