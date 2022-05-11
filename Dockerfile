FROM golang:1.17-alpine as build

RUN apk add --no-cache gcc musl-dev linux-headers

COPY . /rdo

ENV CGO_ENABLED 1

WORKDIR /rdo
RUN go build -o /rdo/bin/raido /rdo/cmd/blockchain/main.go

FROM alpine:latest
COPY --from=build /rdo/bin/raido /usr/local/bin/
ENV BOOTSTRAP = ""
RUN cd /usr/local && mkdir config && mkdir data

EXPOSE 4000 5555 9999

ENTRYPOINT raido --p2p-bootstrap-nodes "${BOOTSTRAP}" --config-file=/usr/local/config/config.yaml --chain-config-file=/usr/local/config/net.yaml