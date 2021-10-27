package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/bombsimon/logrusr/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
	"github.com/sirupsen/logrus"

	"github.com/robfig/cron/v3"

	"gopkg.in/alecthomas/kingpin.v2"
)

// Same struct prometheus uses for their /version address.
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
	log = logrus.New()

	listenAddress = kingpin.Flag("telemetry.addr",
		"Listen on host:port.",
	).Default(":9141").String()
	metricsPath = kingpin.Flag("telemetry.path",
		"URL path for surfacing collected metrics.",
	).Default("/metrics").String()
	logLevel = kingpin.Flag("log.level",
		"Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, fatal]",
	).Default("info").Enum("debug", "info", "warn", "error", "fatal")
	logFormat = kingpin.Flag("log.format",
		"Set the log format. Valid formats: [json, text]",
	).Default("json").Enum("json", "text")

	schedule = kingpin.Flag("schedule",
		"Cron job schedule for fetching snapshot data.",
	).Default("0 14 * * *").String()

	address = kingpin.Flag("address",
		"Elasticsearch node to use.",
	).Default("http://localhost:9200").String()
	repository = kingpin.Flag("repository",
		"Elasticsearch snapshot repository name.",
	).Default("s3-backup").String()
	insecure = kingpin.Flag("insecure",
		"Allow insecure server connections when using SSL.",
	).Default("false").Bool()
	threads = kingpin.Flag("threads",
		"Number of concurrent http requests to Elasticsearch.",
	).Default("2").Int()

	labels       = []string{"repository", "state", "snapshot", "prefix"}
	snapshotSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   "elasticsearch",
		Subsystem:   "snapshot_stats",
		Name:        "size_in_bytes_total",
		Help:        "Total size of files that are referenced by the snapshot",
		ConstLabels: nil,
	}, labels)

	currentSnapshots []map[string]interface{}
)

func main() {
	kingpin.Version(version.Print("es-snapshot-exporter"))
	kingpin.HelpFlag.Short('h')

	kingpin.Parse()

	if err := setLogLevel(*logLevel); err != nil {
		log.Fatal(err)
	}
	if err := setLogFormat(*logFormat); err != nil {
		log.Fatal(err)
	}

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

	if err := connectionCheck(); err != nil {
		log.Fatal(err)
	}

	log.Infof("Starting cron with schedule: \"%s\"", *schedule)
	c := cron.New(
		cron.WithChain(
			cron.SkipIfStillRunning(logrusr.New(log)),
			cron.Recover(logrusr.New(log)),
		),
	)
	e, err := c.AddFunc(*schedule, func() { getMetrics() })
	if err != nil {
		log.Fatal(err)
	}
	c.Start()

	go func() {
		c.Entry(e).WrappedJob.Run()
	}()

	log.Info("Starting server on ", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))

}

func getMetrics() error {
	log.Info("Fetching data from: ", *address)
	client, err := NewClient([]string{*address}, *repository, *insecure)
	if err != nil {
		return fmt.Errorf("error creating the client: %v", err)
	}

	previousSnapshots := currentSnapshots

	currentSnapshots, err = client.GetSnapshot([]string{"_all"})
	if err != nil {
		return fmt.Errorf("error fetching snapshot: %v", err)
	}
	log.Debugf("Got %d snapshots", len(currentSnapshots))

	// delete previous metrics to avoid exposing metrics for nonexistent snapshots
	m := make(map[string]struct{}, len(currentSnapshots))
	for _, cur := range currentSnapshots {
		m[cur["snapshot"].(string)] = struct{}{}
	}

	var todelete []map[string]interface{}
	for _, prev := range previousSnapshots {
		if _, found := m[prev["snapshot"].(string)]; !found {
			todelete = append(todelete, prev)
		}
	}

	for _, v := range todelete {
		log.Debug("Deleting previous snapshot metrics for: ", v["snapshot"].(string))
		snapshotSize.DeleteLabelValues(getLabelValues(v)...)
	}

	var wg sync.WaitGroup
	ch := make(chan string)
	for i := 0; i < *threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for s := range ch {
				stats, err := client.GetSnapshotStatus([]string{s})
				if err != nil {
					log.Errorf("error fetching snapshot status: %v", err)
					continue
				}
				for _, snap := range stats {
					stats := snap["stats"].(map[string]interface{})
					total := stats["total"].(map[string]interface{})
					size := total["size_in_bytes"].(float64)
					snapshotSize.WithLabelValues(getLabelValues(snap)...).Set(size)
				}
			}
		}()
	}

	for _, v := range currentSnapshots {
		ch <- v["snapshot"].(string)
	}

	close(ch)
	wg.Wait()

	log.Info("Finished fetching snapshot data")

	return nil
}

func getLabelValues(snapshot map[string]interface{}) (values []string) {
	for _, label := range labels {
		switch label {
		case "prefix":
			values = append(values, strings.Split(snapshot["snapshot"].(string), "-")[0])
		case "repository":
			values = append(values, *repository)
		default:
			values = append(values, snapshot[label].(string))
		}
	}

	return values
}

func connectionCheck() error {
	client, err := NewClient([]string{*address}, *repository, *insecure)
	if err != nil {
		return fmt.Errorf("error creating the client: %v", err)
	}

	v, err := client.GetInfo()
	if err != nil {
		return fmt.Errorf("error getting cluster info: %v", err)
	}

	log.Infof("Cluster info: %v", v)

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
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return err
	}
	log.SetLevel(lvl)

	return nil
}

func setLogFormat(format string) error {
	var formatter logrus.Formatter

	switch format {
	case "text":
		formatter = &logrus.TextFormatter{
			DisableColors: true,
			FullTimestamp: true,
		}
	case "json":
		formatter = &logrus.JSONFormatter{}
	default:
		return fmt.Errorf("invalid log format: %s", format)
	}

	log.SetFormatter(formatter)

	return nil
}
