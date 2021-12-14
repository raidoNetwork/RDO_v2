package main

import (
	"net/http"
	_ "net/http/pprof"
	"os"
	"testing"
)

func TestMainApp(t *testing.T) {
	os.Args = []string{
		"raido",
		"-chain-config-file=/Users/Boris/Projects/raido-data/cfg/net.yaml",
		"-config-file=/Users/Boris/Projects/raido-data/cfg/config.yaml",
	}

	go func() {
		http.ListenAndServe("localhost:8080", nil)
	}()

	main()

	t.Log("All good")
}
