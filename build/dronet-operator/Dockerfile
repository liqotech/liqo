FROM golang:1.13 as builder
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go
COPY . /go/src/github.com/netgroup-polito/dronev2
WORKDIR /go/src/github.com/netgroup-polito/dronev2
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/dronet-operator/
RUN cp dronet-operator /usr/bin/dronet-operator

FROM scratch
COPY --from=builder /usr/bin/dronet-operator /usr/bin/dronet-operator
ENTRYPOINT [ "/usr/bin/dronet-operator" ]

# Build
#RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o manager main.go
