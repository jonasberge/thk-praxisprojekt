FROM golang:1.16-alpine

RUN set -ex; \
    apk update; \
    apk add --no-cache \
        git

WORKDIR /mod
COPY go.mod go.sum ./
RUN go mod download

WORKDIR /
RUN wget https://raw.githubusercontent.com/eficode/wait-for/23cd9a012e9c097fd150567818ffda51185640b2/wait-for

WORKDIR /test
ENV CGO_ENABLED=0
CMD tail -f /dev/null
