FROM liqo/runner as builder
#install required dependencies
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get -y install gcc tar wget libgtk-3-dev libappindicator3-dev \
libwebkit2gtk-4.0-dev

FROM builder as gobuild
RUN wget https://dl.google.com/go/go1.14.3.linux-amd64.tar.gz && tar -xzf go1.14.3.linux-amd64.tar.gz
ENV PATH /go/bin:$PATH
ENV GOROOT /go
ENV GOPATH /go
COPY . /go/src/github.com/liqoTech/liqo
WORKDIR /go/src/github.com/liqoTech/liqo
RUN GO111MODULE=on CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build ./cmd/tray-agent/
RUN cp tray-agent /usr/bin/tray-agent

FROM builder
LABEL net.liqo.agent.maintainer="camix1984@gmail.com"
LABEL net.liqo.agent.version="0.1"
LABEL net.liqo.agent.description="first demo version of the Liqo tray Agent"
ENV HOME /root
COPY --from=gobuild /usr/bin/tray-agent /usr/bin/tray-agent
COPY ./internal/tray-agent/icon/icons $HOME/liqo/icons
ENTRYPOINT [ "/usr/bin/tray-agent" ]
