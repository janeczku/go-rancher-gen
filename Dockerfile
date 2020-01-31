FROM alpine:edge

RUN apk add --no-cache go ca-certificates

WORKDIR /app

ADD go.mod .
ADD go.sum .
RUN go mod download

ADD cmd cmd/
RUN go build -o /usr/local/bin/rancher-conf ./cmd/rancher-conf

ENTRYPOINT [ "/usr/local/bin/rancher-conf" ]
