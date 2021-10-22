# elasticsearch-snapshot-exporter

[![Go](https://github.com/flant/elasticsearch-snapshot-exporter/actions/workflows/go.yml/badge.svg)](https://github.com/flant/elasticsearch-snapshot-exporter/actions/workflows/go.yml)

```
$ es-snapshot-exporter -h
usage: es-snapshot-exporter [<flags>]

Flags:
  -h, --help                    Show context-sensitive help (also try --help-long and --help-man).
      --telemetry.addr=":9141"  Listen on host:port.
      --telemetry.path="/metrics"
                                URL path for surfacing collected metrics.
      --log.level=info          Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, fatal]
      --log.format=json         Set the log format. Valid formats: [json, text]
      --schedule="0 14 * * *"   Cron job schedule for fetching snapshot data.
      --address="http://localhost:9200"
                                Elasticsearch node to use.
      --root.ca=/etc/ssl/certs/elk-root-ca.pem
                                PEM-encoded certificate authorities
      --repository="s3-backup"  Elasticsearch snapshot repository name.
      --insecure                Allow insecure server connections when using SSL.
      --threads=2               Number of concurrent http requests to Elasticsearch.
      --version                 Show application version.
```

Expose snapshot size metric

```
# HELP elasticsearch_snapshot_stats_size_in_bytes_total Total size of files that are referenced by the snapshot
# TYPE elasticsearch_snapshot_stats_size_in_bytes_total gauge
elasticsearch_snapshot_stats_size_in_bytes_total{prefix="",repository="",snapshot="",state=""} 0
```
