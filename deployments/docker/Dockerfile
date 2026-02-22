FROM alpine:3.21

RUN apk add --no-cache \
    mysql-client \
    mariadb-client

ARG TARGETARCH

COPY bin/optidump-linux-${TARGETARCH} /usr/local/bin/optidump

VOLUME /etc/optidump
VOLUME /opt/backup

ENTRYPOINT ["optidump"]
CMD ["--config", "/etc/optidump/config.yml"]
