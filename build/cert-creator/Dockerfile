FROM alpine:3.23.2

RUN apk update && \
    apk add --no-cache openssl && \
    rm -rf /var/cache/apk/*

ENTRYPOINT ["openssl"]
