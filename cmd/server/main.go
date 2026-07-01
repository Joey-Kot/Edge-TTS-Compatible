package main

import (
	"errors"
	"flag"
	"log"
	"net/http"
	"os"

	"edge-tts-compatible/internal/config"
	"edge-tts-compatible/internal/edge"
	"edge-tts-compatible/internal/httpapi"
)

func main() {
	cfg, err := config.Parse(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		log.Fatal(err)
	}
	client, err := edge.NewClient(edge.Config{
		Timeout:     cfg.UpstreamTimeout,
		Proxy:       cfg.ProxyURL,
		Concurrency: cfg.UpstreamConcurrency,
		Interval:    cfg.UpstreamInterval,
	})
	if err != nil {
		log.Fatal(err)
	}
	handler := httpapi.New(cfg, client)
	server := newHTTPServer(cfg, handler)
	log.Printf("listening on %s", cfg.Listen)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func newHTTPServer(cfg config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.Listen,
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
}
