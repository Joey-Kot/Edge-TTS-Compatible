package config

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

const DefaultModel = "edge"

var envVarNames = []string{
	"LISTEN",
	"API_TOKEN",
	"DEFAULT_VOICE",
	"PROXY",
	"UPSTREAM_TIMEOUT",
	"UPSTREAM_CONCURRENCY",
	"UPSTREAM_INTERVAL_MS",
	"READ_HEADER_TIMEOUT",
	"IDLE_TIMEOUT",
}

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
	fs.Usage = func() {
		WriteUsage(fs.Output(), fs.Name())
	}

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

	setFlags := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		setFlags[f.Name] = true
	})

	if !setFlags["listen"] {
		applyStringEnv("LISTEN", &cfg.Listen)
	}
	if !setFlags["api-token"] {
		applyStringEnv("API_TOKEN", &apiTokenCSV)
	}
	if !setFlags["default-voice"] {
		applyStringEnv("DEFAULT_VOICE", &cfg.DefaultVoice)
	}
	if !setFlags["proxy"] {
		applyStringEnv("PROXY", &cfg.ProxyURL)
	}
	if !setFlags["upstream-timeout"] {
		if err := applyFloatEnv("UPSTREAM_TIMEOUT", &upstreamTimeoutSeconds); err != nil {
			return Config{}, err
		}
	}
	if !setFlags["upstream-concurrency"] {
		if err := applyIntEnv("UPSTREAM_CONCURRENCY", &cfg.UpstreamConcurrency); err != nil {
			return Config{}, err
		}
	}
	if !setFlags["upstream-interval-ms"] {
		if err := applyFloatEnv("UPSTREAM_INTERVAL_MS", &upstreamIntervalMillis); err != nil {
			return Config{}, err
		}
	}
	if !setFlags["read-header-timeout"] {
		if err := applyFloatEnv("READ_HEADER_TIMEOUT", &readHeaderTimeoutSeconds); err != nil {
			return Config{}, err
		}
	}
	if !setFlags["idle-timeout"] {
		if err := applyFloatEnv("IDLE_TIMEOUT", &idleTimeoutSeconds); err != nil {
			return Config{}, err
		}
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

func WriteUsage(w io.Writer, program string) {
	fmt.Fprintf(w, `Usage:
  %[1]s [flags]

Example:
  %[1]s --listen :8080 --api-token sk-local --default-voice en-US-EmmaMultilingualNeural

Flags:
  --api-token string
      comma-separated local bearer token list
  --default-voice string
      default Edge TTS voice (default en-US-EmmaMultilingualNeural)
  --idle-timeout float
      local HTTP idle timeout in seconds (default 120)
  --listen string
      HTTP listen address (default :8080)
  --proxy string
      optional HTTP proxy URL for Edge TTS upstream requests
  --read-header-timeout float
      local HTTP read header timeout in seconds (default 10)
  --upstream-concurrency int
      maximum concurrent Edge TTS upstream requests (default 10)
  --upstream-interval-ms float
      minimum interval in milliseconds between any two Edge TTS upstream requests (default 500)
  --upstream-timeout float
      Edge TTS upstream timeout in seconds (default 120)

Environment variables:
  Each flag can also be set with its equivalent environment variable. Command-line flags take priority. See docker.env.example.

Compatible APIs:
  OpenAI Audio Speech: POST /v1/audio/speech, POST /audio/speech
  Models:              GET /v1/models
  Voices:              GET/POST /v1/voices, /voices, /v1/audio/voices, /audio/voices
  Common endpoints:    /health
`, program)
}

func applyStringEnv(name string, target *string) {
	if value, ok := os.LookupEnv(name); ok {
		*target = value
	}
}

func applyFloatEnv(name string, target *float64) error {
	value, ok := os.LookupEnv(name)
	if !ok {
		return nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("%s must be a number: %w", name, err)
	}
	*target = parsed
	return nil
}

func applyIntEnv(name string, target *int) error {
	value, ok := os.LookupEnv(name)
	if !ok {
		return nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("%s must be an integer: %w", name, err)
	}
	*target = parsed
	return nil
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
