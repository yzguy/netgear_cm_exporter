FROM golang:1.20.1 AS builder
WORKDIR /usr/src/app
COPY . .
RUN go build -v .
ENTRYPOINT ["/usr/src/app/netgear_cm_exporter"]
CMD ["-config.file=/usr/src/app/netgear_cm_exporter.yml"]

FROM alpine:3.17.3
COPY --from=builder /usr/src/app/netgear_cm_exporter /usr/local/bin
COPY --from=builder /usr/src/app/netgear_cm_exporter.yml /etc/netgear_cm_exporter

ENTRYPOINT ["/usr/local/bin/netgear_cm_exporter"]
CMD ["-config.file=/etc/netgear_cm_exporter/netgear_cm_exporter.yml"]
