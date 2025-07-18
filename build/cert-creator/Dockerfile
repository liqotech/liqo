FROM alpine:3.22.1

RUN apk update && \
  apk add --no-cache openssl && \
  rm -rf /var/cache/apk/*

ENTRYPOINT ["openssl"]
