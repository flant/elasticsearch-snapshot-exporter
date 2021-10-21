FROM debian:stable
LABEL maintainer="Vasily Maryutenkov <vasily.maryutenkov@flant.com>"

ENV VERSION 0.2.0
ENV DOWNLOAD_URL https://github.com/flant/elasticsearch-snapshot-exporter/releases/download/${VERSION}/es-snapshot-exporter-linux-amd64

RUN     DEBIAN_FRONTEND=noninteractive; apt-get update \
        && apt-get install -qy --no-install-recommends \
            ca-certificates \
            curl \
        && curl -fsSL "$DOWNLOAD_URL" -o /es-snapshot-exporter \
        && chmod 755 /es-snapshot-exporter

EXPOSE 9141/tcp

ENTRYPOINT [ "/es-snapshot-exporter" ]
