package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"

	elasticsearch "github.com/elastic/go-elasticsearch/v7"
)

type Client struct {
	es         *elasticsearch.Client
	repository string
}

func NewClient(addresses []string, repository string, insecure bool) (*Client, error) {
	cfg := elasticsearch.Config{
		Addresses: addresses,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: insecure,
			},
		},
	}

	es, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	return &Client{es, repository}, nil

}

func (c *Client) GetSnapshot(s []string) ([]map[string]interface{}, error) {
	log.Debug("Getting snapshots from repository: ", c.repository)
	resp, err := c.es.Snapshot.Get(c.repository, s)
	if err != nil {
		return nil, fmt.Errorf("error getting response: %s", err)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return nil, fmt.Errorf("request failed: %v", resp.String())
	}

	var r map[string][]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}

	m := make([]map[string]interface{}, 0, len(r))
	for _, v := range r["snapshots"] {
		m = append(m, v.(map[string]interface{}))
	}

	return m, nil
}

func (c *Client) GetSnapshotStatus(s []string) ([]map[string]interface{}, error) {
	log.Debug("Getting snapshot status for: ", s)
	resp, err := c.es.Snapshot.Status(
		c.es.Snapshot.Status.WithRepository(c.repository),
		c.es.Snapshot.Status.WithSnapshot(s...),
	)
	if err != nil {
		return nil, fmt.Errorf("error getting response: %s", err)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return nil, fmt.Errorf("request failed: %v", resp.String())
	}

	var r map[string][]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}

	m := make([]map[string]interface{}, 0, len(r))
	for _, v := range r["snapshots"] {
		m = append(m, v.(map[string]interface{}))
	}

	return m, nil
}

func (c *Client) GetInfo() (map[string]interface{}, error) {
	log.Debug("Getting cluster info")
	resp, err := c.es.Info()
	if err != nil {
		return nil, fmt.Errorf("error getting response: %s", err)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return nil, fmt.Errorf("request failed: %v", resp.String())
	}

	var r map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}

	return r, nil
}
