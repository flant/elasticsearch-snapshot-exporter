FROM golang:1.19-buster@sha256:46bab7f043402231a9ed12ef2a4b0a9d090e3abc005297de920160a76bc71da3 as builder

WORKDIR /src

COPY . .

RUN go install -v github.com/prometheus/promu \
    && promu build -v --prefix build


FROM debian:buster-slim@sha256:bb3dc79fddbca7e8903248ab916bb775c96ec61014b3d02b4f06043b604726dc
LABEL maintainer="Vasily Maryutenkov <vasily.maryutenkov@flant.com>"

RUN DEBIAN_FRONTEND=noninteractive; apt-get update \
    && apt-get install -qy --no-install-recommends \
        ca-certificates \
        tzdata \
        curl

COPY --from=builder /src/build/es-snapshot-exporter /es-snapshot-exporter

EXPOSE 9141/tcp

CMD [ "/es-snapshot-exporter" ]
