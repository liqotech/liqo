FROM alpine:3.23.4

RUN apk update && \
    apk add --no-cache openssl && \
    rm -rf /var/cache/apk/*

ENTRYPOINT ["openssl"]
