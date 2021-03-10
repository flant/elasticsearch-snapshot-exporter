package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"github.com/tidwall/gjson"
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

const namespace = "elasticsearch"

var (
	listenAddress = kingpin.Flag(
		"telemetry.addr",
		"host:port for exporter.",
	).Default(":9141").String()
	metricsPath = kingpin.Flag(
		"telemetry.path",
		"URL path for surfacing collected metrics.",
	).Default("/metrics").String()
	dataDir = kingpin.Flag(
		"data.dir",
		"Directory containing json files with snapshot status",
	).Default("/data").String()

	labels = []string{"repository", "state", "snapshot", "prefix"}

	snapshotSize = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "snapshot_stats", "size_in_bytes_total"),
		"Total size of files that are referenced by the snapshot",
		labels, nil,
	)
)

type Collector struct{}

func (e *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- snapshotSize
}

func (e *Collector) Collect(ch chan<- prometheus.Metric) {
	err := filepath.Walk(*dataDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				log.Fatalf("Error reading data.dir: %v", err)
			}
			if filepath.Ext(path) == ".json" {
				log.Debugf("Processing file %v. Size: %v", path, info.Size())
				f, err := ioutil.ReadFile(path)
				if err != nil {
					return err
				}
				for _, snapshot := range gjson.GetBytes(f, "snapshots").Array() {
					labelValues := getLabelValues(&snapshot)
					ch <- prometheus.MustNewConstMetric(
						snapshotSize, prometheus.GaugeValue, snapshot.Get("stats.total.size_in_bytes").Float(),
						labelValues...,
					)
				}
			}
			return nil
		})

	if err != nil {
		log.Error(err)
	}
}

func getLabelValues(snapshot *gjson.Result) []string {
	var values []string
	for _, label := range labels {
		if label == "prefix" {
			values = append(values, strings.Split(snapshot.Get("snapshot").String(), "-")[0])
			continue
		}
		values = append(values, snapshot.Get(label).String())
	}

	return values
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, err := fmt.Fprintln(w, `{"status":"ok"}`)
	if err != nil {
		log.Debugf("Failed to write to stream: %v", err)
	}
}

func main() {
	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("es-snapshot-exporter"))
	kingpin.HelpFlag.Short('h')

	kingpin.Parse()

	if _, err := os.Stat(*dataDir); err != nil {
		log.Fatalf("Error reading data.dir: %v", err)
	}

	prometheus.MustRegister(&Collector{})

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

	log.Infoln("Starting es-snapshot-exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	log.Infoln("Starting server on", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))

}
