#!/bin/sh
case "$(uname -m)" in
  x86_64|amd64)
    export ARCH=amd64
    ;;
  arm*|aarch64)
    export ARCH=arm64
    ;;
esac

VERSION=v1.20.4

curl -LO https://storage.googleapis.com/kubernetes-release/release/${VERSION}/bin/linux/${ARCH}/kubectl && chmod +x ./kubectl && cp kubectl /usr/bin/kubectl