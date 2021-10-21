package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	elasticsearch "github.com/elastic/go-elasticsearch/v7"
)

type Client struct {
	es         *elasticsearch.Client
	repository string
}

func NewClient(endpoints []string, cacert, repository string, insecure bool) (*Client, error) {
	rootCAs, _ := x509.SystemCertPool()
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}
	if cacert != "" {
		cert, err := ioutil.ReadFile(cacert)
		if err != nil {
			return nil, fmt.Errorf("failed to append %q to RootCAs: %v", cacert, err)
		}
		if ok := rootCAs.AppendCertsFromPEM(cert); !ok {
			log.Error("No certs appended, using system certs only")
		}
	}

	cfg := elasticsearch.Config{
		Addresses: endpoints,
		Transport: &http.Transport{
			MaxIdleConnsPerHost:   10,
			ResponseHeaderTimeout: time.Second,
			DialContext:           (&net.Dialer{Timeout: time.Second}).DialContext,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: insecure,
				RootCAs:            rootCAs,
			},
		},
	}

	es, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	return &Client{es, repository}, nil

}

func (c *Client) GetSnapshots() ([]string, error) {
	resp, err := c.es.Cat.Snapshots()
	var r []CatSnapshot
	if err != nil {
		return nil, fmt.Errorf("error getting response: %s", err)
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}

	var s []string
	for _, v := range r {
		s = append(s, v.ID)
	}

	return s, nil
}

func (c *Client) GetSnapshot(s string) (*Snapshot, error) {
	resp, err := c.es.Snapshot.Get(c.repository, []string{s})
	if err != nil {
		return nil, fmt.Errorf("error getting response: %s", err)
	}
	defer resp.Body.Close()

	var r Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}

	return &r, nil
}

type CatSnapshot struct {
	ID               string    `json:"id"`
	Status           string    `json:"status"`
	StartEpoch       int       `json:"start_epoch"`
	StartTime        time.Time `json:"start_time"`
	EndEpoch         int       `json:"end_epoch"`
	EndTime          time.Time `json:"end_time"`
	Duration         string    `json:"duration"`
	Indices          int       `json:"indices"`
	SuccessfulShards int       `json:"successful_shards"`
	FailedShards     int       `json:"failed_shards"`
	TotalShards      int       `json:"total_shards"`
}

type ShardStats struct {
	Initializing int `json:"initializing"`
	Started      int `json:"started"`
	Finalizing   int `json:"finalizing"`
	Done         int `json:"done"`
	Failed       int `json:"failed"`
	Total        int `json:"total"`
}

type Stats struct {
	Incremental struct {
		FileCount   int `json:"file_count"`
		SizeInBytes int `json:"size_in_bytes"`
	} `json:"incremental"`
	Total struct {
		FileCount   int `json:"file_count"`
		SizeInBytes int `json:"size_in_bytes"`
	} `json:"total"`
	StartTimeInMillis int `json:"start_time_in_millis"` //time.Seconds
	TimeInMillis      int `json:"time_in_millis"`
}

type IndexStats struct {
	ShardsStats ShardStats `json:"shards_stats"`
	Stats       Stats      `json:"stats"`
	Shards      map[string]struct {
		Stage string `json:"stage"`
		Stats Stats  `json:"stats"`
	} `json:"shards"`
}

type Snapshot struct {
	Snapshot           string                `json:"snapshot"`
	Repository         string                `json:"repository"`
	UUID               string                `json:"uuid"`
	State              string                `json:"state"`
	IncludeGlobalState bool                  `json:"include_global_state"`
	ShardStats         ShardStats            `json:"shards_stats"`
	Stats              Stats                 `json:"stats"`
	Indices            map[string]IndexStats `json:"indices"`
}
