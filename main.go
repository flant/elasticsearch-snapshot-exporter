package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
	log "github.com/sirupsen/logrus"

	"github.com/robfig/cron/v3"

	"gopkg.in/alecthomas/kingpin.v2"
)

// Same struct prometheus uses for their /version endpoint.
// Separate copy to avoid pulling all of prometheus as a dependency
type prometheusVersion struct {
	Version   string `json:"version"`
	Revision  string `json:"revision"`
	Branch    string `json:"branch"`
	BuildUser string `json:"buildUser"`
	BuildDate string `json:"buildDate"`
	GoVersion string `json:"goVersion"`
}

var (
	listenAddress = kingpin.Flag(
		"telemetry.addr",
		"host:port for exporter.",
	).Default(":9141").String()
	metricsPath = kingpin.Flag(
		"telemetry.path",
		"URL path for surfacing collected metrics.",
	).Default("/metrics").String()
	logLevel = kingpin.Flag("log.level",
		"Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, fatal]",
	).Default("info").Enum("debug", "info", "warn", "error", "fatal")
	logFormat = kingpin.Flag("log.format",
		"Set the log format. Valid formats: [json, text]",
	).Default("json").Enum("json", "text")

	schedule = kingpin.Flag("schedule",
		"Cron job schedule",
	).Default("0 14 * * *").String()

	endpoint = kingpin.Flag("endpoint",
		"Elasticsearch URL. ",
	).Default("http://localhost:9200").String()
	cacert = kingpin.Flag("cacert",
		"Elasticsearch CA certificate",
	).PlaceHolder("/etc/ssl/certs/elk-root-ca.pem").ExistingFile()
	insecure = kingpin.Flag("insecure",
		"Allow insecure server connections when using SSL",
	).Default("false").Bool()
	repository = kingpin.Flag("repository",
		"Elasticsearch backup repository name",
	).Default("s3-backup").String()
	threads = kingpin.Flag("threads",
		"Number of concurrent http requests to elasticsearch",
	).Default("1").Int()

	snapshotSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   "elasticsearch",
		Subsystem:   "snapshot_stats",
		Name:        "size_in_bytes_total",
		Help:        "Total size of files that are referenced by the snapshot",
		ConstLabels: nil,
	}, []string{"repository", "state", "snapshot", "prefix"})
)

func main() {
	kingpin.Version(version.Print("es-snapshot-exporter"))
	kingpin.HelpFlag.Short('h')

	kingpin.Parse()
	setLogLevel(*logLevel)
	setLogFormat(*logFormat)

	prometheus.MustRegister(snapshotSize)

	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/healthz", healthCheck)
	http.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		// we can't use "version" directly as it is a package, and not an object that
		// can be serialized.
		err := json.NewEncoder(w).Encode(prometheusVersion{
			Version:   version.Version,
			Revision:  version.Revision,
			Branch:    version.Branch,
			BuildUser: version.BuildUser,
			BuildDate: version.BuildDate,
			GoVersion: version.GoVersion,
		})
		if err != nil {
			http.Error(w, fmt.Sprintf("error encoding JSON: %s", err), http.StatusInternalServerError)
		}
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html>
<head><title>es-snapshot-exporter</title></head>
<body>
<h1>es-snapshot-exporter</h1>
<p><a href="` + *metricsPath + `">Metrics</a></p>
<p><i>` + version.Info() + `</i></p>
</body>
</html>`))
	})

	log.Info("Starting es-snapshot-exporter", version.Info())
	log.Info("Build context", version.BuildContext())

	go func() {
		log.Infof("Fetching data from: %s", *endpoint)
		if err := getMetrics(); err != nil {
			log.Fatal(err)
		}
	}()

	c := cron.New()
	if _, err := c.AddFunc(*schedule, func() { getMetrics() }); err != nil {
		log.Fatal(err)
	}
	c.Start()

	log.Info("Starting server on", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))

}

func getMetrics() error {
	client, err := NewClient([]string{*endpoint}, *cacert, *repository, *insecure)
	if err != nil {
		return fmt.Errorf("error creating the client: %s", err)
	}

	s, err := client.GetSnapshots()
	if err != nil {
		return fmt.Errorf("error fetching snapshots: %s", err)
	}

	var wg sync.WaitGroup
	ch := make(chan string)
	for i := 0; i < *threads; i++ {
		wg.Add(1)
		go func() {
			for s := range ch {
				snap, err := client.GetSnapshot(s)
				if err != nil {
					log.Error(err)
					continue
				}
				snapshotSize.WithLabelValues(
					snap.Repository,
					snap.State,
					snap.Snapshot,
					strings.Split(snap.Snapshot, "-")[0],
				).Set(float64(snap.Stats.Total.SizeInBytes))
			}
		}()
	}

	for _, s := range s {
		ch <- s
	}

	close(ch)
	wg.Wait()

	log.Info("Finished fetching snapshot info")

	return nil
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, err := fmt.Fprintln(w, `{"status":"ok"}`)
	if err != nil {
		log.Debugf("Failed to write to stream: %v", err)
	}
}

func setLogLevel(level string) error {
	lvl, err := log.ParseLevel(level)
	if err != nil {
		return err
	}
	log.SetLevel(lvl)

	return nil
}

func setLogFormat(format string) error {
	var formatter log.Formatter

	switch format {
	case "text":
		formatter = &log.TextFormatter{
			DisableColors: true,
			FullTimestamp: true,
		}
	case "json":
		formatter = &log.JSONFormatter{}
	default:
		return fmt.Errorf("invalid log format: %s", format)
	}

	log.SetFormatter(formatter)

	return nil
}
