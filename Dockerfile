FROM alpine:edge

RUN apk add --no-cache ca-certificates

ADD build/rancher-conf-linux-amd64 /usr/local/bin/rancher-conf

ENTRYPOINT [ "/usr/local/bin/rancher-conf" ]
