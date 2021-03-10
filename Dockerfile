FROM debian:stable
LABEL maintainer="Vasily Maryutenkov <vasily.maryutenkov@flant.com>"

RUN apt-get update \
        && apt-get dist-upgrade -y \
        && apt-get install -y --no-install-recommends \
            ca-certificates \
            curl

ENV VERSION 0.1.0
ENV DOWNLOAD_URL https://github.com/flant/elasticsearch-snapshot/exporter/es-snapshot-exporter/releases/download/${VERSION}/es-snapshot-exporter-linux-amd64

RUN curl -fsSL "$DOWNLOAD_URL" -o /es-snapshot-exporter

EXPOSE 9141/tcp

ENTRYPOINT [ "/es-snapshot-exporter" ]
CMD [ "--base.dir", "/data" ]
