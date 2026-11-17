package main

import (
	"crypto/tls"
	"net/http"
	"reflect"
	"testing"
)

func TestNewElasticsearchConfig(t *testing.T) {
	addresses := []string{"https://opensearch.example:9200"}
	tlsConfig := &tls.Config{InsecureSkipVerify: true}

	cfg := newElasticsearchConfig(addresses, tlsConfig, "exporter", "secret")

	if !reflect.DeepEqual(cfg.Addresses, addresses) {
		t.Fatalf("addresses = %v, want %v", cfg.Addresses, addresses)
	}
	if cfg.Username != "exporter" {
		t.Errorf("username = %q, want %q", cfg.Username, "exporter")
	}
	if cfg.Password != "secret" {
		t.Errorf("password = %q, want %q", cfg.Password, "secret")
	}
	if cfg.Transport.(*http.Transport).TLSClientConfig != tlsConfig {
		t.Error("TLS configuration was not passed to the transport")
	}
}
