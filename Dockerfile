FROM golang:1.19-buster as builder

WORKDIR /src

COPY . .

RUN go install -v github.com/prometheus/promu \
    && promu build -v --prefix build


FROM debian:buster-slim
LABEL maintainer="Vasily Maryutenkov <vasily.maryutenkov@flant.com>"

RUN DEBIAN_FRONTEND=noninteractive; apt-get update \
    && apt-get install -qy --no-install-recommends \
        ca-certificates \
        tzdata \
        curl

COPY --from=builder /src/build/es-snapshot-exporter /es-snapshot-exporter

EXPOSE 9141/tcp

CMD [ "/es-snapshot-exporter" ]
