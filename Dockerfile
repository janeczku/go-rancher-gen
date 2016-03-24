FROM alpine:edge
MAINTAINER Jan Broer <jan@festplatte.eu.org>

RUN apk add --no-cache ca-certificates

ENV RANCHER_TEMPLATE_RELEASE v0.1.0

ADD https://github.com/janeczku/rancher-template/releases/download/${RANCHER_TEMPLATE_RELEASE}/rancher-template-linux-amd64.tar.gz /tmp/rancher-template.tar.gz
RUN tar -zxvf /tmp/rancher-template.tar.gz -C /usr/local/bin \
	&& chmod +x /usr/local/bin/rancher-template

ENTRYPOINT ["/usr/local/bin/rancher-template"]