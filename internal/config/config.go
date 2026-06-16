package config

import (
	"flag"
	"fmt"
	"strings"
	"time"
)

const DefaultModel = "edge"

type Config struct {
	Listen              string
	APITokens           []string
	DefaultVoice        string
	ReadHeaderTimeout   time.Duration
	IdleTimeout         time.Duration
	UpstreamTimeout     time.Duration
	ProxyURL            string
	UpstreamConcurrency int
	UpstreamInterval    time.Duration
}

func Parse(args []string) (Config, error) {
	fs := flag.NewFlagSet("edge-tts-compatible", flag.ContinueOnError)

	var apiTokenCSV string
	var readHeaderTimeoutSeconds float64
	var idleTimeoutSeconds float64
	var upstreamTimeoutSeconds float64
	var upstreamIntervalMillis float64

	cfg := Config{}
	fs.StringVar(&cfg.Listen, "listen", ":8080", "HTTP listen address")
	fs.StringVar(&apiTokenCSV, "api-token", "", "comma-separated local bearer token list")
	fs.StringVar(&cfg.DefaultVoice, "default-voice", "en-US-EmmaMultilingualNeural", "default Edge TTS voice")
	fs.Float64Var(&readHeaderTimeoutSeconds, "read-header-timeout", 10, "local HTTP read header timeout in seconds")
	fs.Float64Var(&idleTimeoutSeconds, "idle-timeout", 120, "local HTTP idle timeout in seconds")
	fs.Float64Var(&upstreamTimeoutSeconds, "upstream-timeout", 120, "Edge TTS upstream timeout in seconds")
	fs.IntVar(&cfg.UpstreamConcurrency, "upstream-concurrency", 10, "maximum concurrent Edge TTS upstream requests")
	fs.Float64Var(&upstreamIntervalMillis, "upstream-interval-ms", 500, "minimum interval in milliseconds between any two Edge TTS upstream requests")
	fs.StringVar(&cfg.ProxyURL, "proxy", "", "optional HTTP proxy URL for Edge TTS upstream requests")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	cfg.APITokens = splitCSV(apiTokenCSV)
	if cfg.DefaultVoice == "" {
		return Config{}, fmt.Errorf("--default-voice must not be empty")
	}
	if readHeaderTimeoutSeconds <= 0 {
		return Config{}, fmt.Errorf("--read-header-timeout must be positive")
	}
	if idleTimeoutSeconds <= 0 {
		return Config{}, fmt.Errorf("--idle-timeout must be positive")
	}
	if upstreamTimeoutSeconds <= 0 {
		return Config{}, fmt.Errorf("--upstream-timeout must be positive")
	}
	if cfg.UpstreamConcurrency <= 0 {
		return Config{}, fmt.Errorf("--upstream-concurrency must be positive")
	}
	if upstreamIntervalMillis < 0 {
		return Config{}, fmt.Errorf("--upstream-interval-ms must be non-negative")
	}
	cfg.ReadHeaderTimeout = time.Duration(readHeaderTimeoutSeconds * float64(time.Second))
	cfg.IdleTimeout = time.Duration(idleTimeoutSeconds * float64(time.Second))
	cfg.UpstreamTimeout = time.Duration(upstreamTimeoutSeconds * float64(time.Second))
	cfg.UpstreamInterval = time.Duration(upstreamIntervalMillis * float64(time.Millisecond))
	return cfg, nil
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
