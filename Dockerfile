FROM golang:latest as build


WORKDIR /go/src/github.com/orktes/homeautomation-metrics
ADD . .
RUN set -x && \
    cd cmd/homeautomation-prometheus && \
    CGO_ENABLED=0 go build \
        -o /build/homeautomation-prometheus
RUN set -x && \
    cd cmd/homeautomation-influxdb && \
    CGO_ENABLED=0 go build \
        -o /build/homeautomation-influxdb

FROM alpine:3.10  
RUN apk add --update --no-cache ca-certificates
WORKDIR /root/
COPY --from=build /build/homeautomation-prometheus .
COPY --from=build /build/homeautomation-influxdb .
CMD ./homeautomation-prometheus --mqtt-topic "haaga/status/#"