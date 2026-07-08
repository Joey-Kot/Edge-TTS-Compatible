package config

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseDefaults(t *testing.T) {
	clearConfigEnv(t)

	cfg, err := Parse(nil)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if cfg.Listen != ":8080" {
		t.Fatalf("Listen mismatch: got %q", cfg.Listen)
	}
	if cfg.DefaultVoice != "en-US-EmmaMultilingualNeural" {
		t.Fatalf("DefaultVoice mismatch: got %q", cfg.DefaultVoice)
	}
	if len(cfg.APITokens) != 0 {
		t.Fatalf("APITokens mismatch: got %v want empty", cfg.APITokens)
	}
	if cfg.ReadHeaderTimeout != 10*time.Second {
		t.Fatalf("ReadHeaderTimeout mismatch: got %s", cfg.ReadHeaderTimeout)
	}
	if cfg.IdleTimeout != 120*time.Second {
		t.Fatalf("IdleTimeout mismatch: got %s", cfg.IdleTimeout)
	}
	if cfg.UpstreamTimeout != 120*time.Second {
		t.Fatalf("UpstreamTimeout mismatch: got %s", cfg.UpstreamTimeout)
	}
	if cfg.UpstreamConcurrency != 10 {
		t.Fatalf("UpstreamConcurrency mismatch: got %d", cfg.UpstreamConcurrency)
	}
	if cfg.UpstreamInterval != 500*time.Millisecond {
		t.Fatalf("UpstreamInterval mismatch: got %s", cfg.UpstreamInterval)
	}
}

func TestParseEnvironmentOverridesDefaults(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("LISTEN", "127.0.0.1:9001")
	t.Setenv("API_TOKEN", "sk-env-one, sk-env-two ,,")
	t.Setenv("DEFAULT_VOICE", "zh-CN-XiaoxiaoNeural")
	t.Setenv("READ_HEADER_TIMEOUT", "3.5")
	t.Setenv("IDLE_TIMEOUT", "40")
	t.Setenv("UPSTREAM_TIMEOUT", "55")
	t.Setenv("UPSTREAM_CONCURRENCY", "4")
	t.Setenv("UPSTREAM_INTERVAL_MS", "150")
	t.Setenv("PROXY", "http://127.0.0.1:7891")

	cfg, err := Parse(nil)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if cfg.Listen != "127.0.0.1:9001" {
		t.Fatalf("Listen mismatch: got %q", cfg.Listen)
	}
	if !reflect.DeepEqual(cfg.APITokens, []string{"sk-env-one", "sk-env-two"}) {
		t.Fatalf("APITokens mismatch: got %v", cfg.APITokens)
	}
	if cfg.DefaultVoice != "zh-CN-XiaoxiaoNeural" {
		t.Fatalf("DefaultVoice mismatch: got %q", cfg.DefaultVoice)
	}
	if cfg.ReadHeaderTimeout != 3500*time.Millisecond {
		t.Fatalf("ReadHeaderTimeout mismatch: got %s", cfg.ReadHeaderTimeout)
	}
	if cfg.IdleTimeout != 40*time.Second {
		t.Fatalf("IdleTimeout mismatch: got %s", cfg.IdleTimeout)
	}
	if cfg.UpstreamTimeout != 55*time.Second {
		t.Fatalf("UpstreamTimeout mismatch: got %s", cfg.UpstreamTimeout)
	}
	if cfg.UpstreamConcurrency != 4 {
		t.Fatalf("UpstreamConcurrency mismatch: got %d", cfg.UpstreamConcurrency)
	}
	if cfg.UpstreamInterval != 150*time.Millisecond {
		t.Fatalf("UpstreamInterval mismatch: got %s", cfg.UpstreamInterval)
	}
	if cfg.ProxyURL != "http://127.0.0.1:7891" {
		t.Fatalf("ProxyURL mismatch: got %q", cfg.ProxyURL)
	}
}

func TestParseFlagsOverrideEnvironment(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("LISTEN", "127.0.0.1:9001")
	t.Setenv("API_TOKEN", "sk-env")
	t.Setenv("DEFAULT_VOICE", "zh-CN-XiaoxiaoNeural")
	t.Setenv("READ_HEADER_TIMEOUT", "3.5")
	t.Setenv("IDLE_TIMEOUT", "40")
	t.Setenv("UPSTREAM_TIMEOUT", "55")
	t.Setenv("UPSTREAM_CONCURRENCY", "4")
	t.Setenv("UPSTREAM_INTERVAL_MS", "150")
	t.Setenv("PROXY", "http://127.0.0.1:7891")

	cfg, err := Parse([]string{
		"-listen", "127.0.0.1:9002",
		"-api-token", "sk-flag",
		"-default-voice", "en-US-EmmaMultilingualNeural",
		"-read-header-timeout", "4.5",
		"-idle-timeout", "50",
		"-upstream-timeout", "65",
		"-upstream-concurrency", "5",
		"-upstream-interval-ms", "175",
		"-proxy", "http://127.0.0.1:7892",
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if cfg.Listen != "127.0.0.1:9002" {
		t.Fatalf("Listen mismatch: got %q", cfg.Listen)
	}
	if !reflect.DeepEqual(cfg.APITokens, []string{"sk-flag"}) {
		t.Fatalf("APITokens mismatch: got %v", cfg.APITokens)
	}
	if cfg.DefaultVoice != "en-US-EmmaMultilingualNeural" {
		t.Fatalf("DefaultVoice mismatch: got %q", cfg.DefaultVoice)
	}
	if cfg.ReadHeaderTimeout != 4500*time.Millisecond {
		t.Fatalf("ReadHeaderTimeout mismatch: got %s", cfg.ReadHeaderTimeout)
	}
	if cfg.IdleTimeout != 50*time.Second {
		t.Fatalf("IdleTimeout mismatch: got %s", cfg.IdleTimeout)
	}
	if cfg.UpstreamTimeout != 65*time.Second {
		t.Fatalf("UpstreamTimeout mismatch: got %s", cfg.UpstreamTimeout)
	}
	if cfg.UpstreamConcurrency != 5 {
		t.Fatalf("UpstreamConcurrency mismatch: got %d", cfg.UpstreamConcurrency)
	}
	if cfg.UpstreamInterval != 175*time.Millisecond {
		t.Fatalf("UpstreamInterval mismatch: got %s", cfg.UpstreamInterval)
	}
	if cfg.ProxyURL != "http://127.0.0.1:7892" {
		t.Fatalf("ProxyURL mismatch: got %q", cfg.ProxyURL)
	}
}

func TestParseOverrides(t *testing.T) {
	clearConfigEnv(t)

	cfg, err := Parse([]string{
		"-listen", "127.0.0.1:9000",
		"-api-token", "sk-one, sk-two ,,",
		"-default-voice", "zh-CN-XiaoxiaoNeural",
		"-read-header-timeout", "2.5",
		"-idle-timeout", "30",
		"-upstream-timeout", "45",
		"-upstream-concurrency", "3",
		"-upstream-interval-ms", "125",
		"-proxy", "http://127.0.0.1:7890",
	})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if cfg.Listen != "127.0.0.1:9000" {
		t.Fatalf("Listen mismatch: got %q", cfg.Listen)
	}
	if !reflect.DeepEqual(cfg.APITokens, []string{"sk-one", "sk-two"}) {
		t.Fatalf("APITokens mismatch: got %v", cfg.APITokens)
	}
	if cfg.DefaultVoice != "zh-CN-XiaoxiaoNeural" {
		t.Fatalf("DefaultVoice mismatch: got %q", cfg.DefaultVoice)
	}
	if cfg.ReadHeaderTimeout != 2500*time.Millisecond {
		t.Fatalf("ReadHeaderTimeout mismatch: got %s", cfg.ReadHeaderTimeout)
	}
	if cfg.IdleTimeout != 30*time.Second {
		t.Fatalf("IdleTimeout mismatch: got %s", cfg.IdleTimeout)
	}
	if cfg.UpstreamTimeout != 45*time.Second {
		t.Fatalf("UpstreamTimeout mismatch: got %s", cfg.UpstreamTimeout)
	}
	if cfg.UpstreamConcurrency != 3 {
		t.Fatalf("UpstreamConcurrency mismatch: got %d", cfg.UpstreamConcurrency)
	}
	if cfg.UpstreamInterval != 125*time.Millisecond {
		t.Fatalf("UpstreamInterval mismatch: got %s", cfg.UpstreamInterval)
	}
	if cfg.ProxyURL != "http://127.0.0.1:7890" {
		t.Fatalf("ProxyURL mismatch: got %q", cfg.ProxyURL)
	}
}

func TestParseRejectsInvalidValues(t *testing.T) {
	clearConfigEnv(t)

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "empty voice", args: []string{"-default-voice", ""}, wantErr: "--default-voice must not be empty"},
		{name: "read timeout", args: []string{"-read-header-timeout", "0"}, wantErr: "--read-header-timeout must be positive"},
		{name: "idle timeout", args: []string{"-idle-timeout", "-1"}, wantErr: "--idle-timeout must be positive"},
		{name: "upstream timeout", args: []string{"-upstream-timeout", "0"}, wantErr: "--upstream-timeout must be positive"},
		{name: "concurrency", args: []string{"-upstream-concurrency", "0"}, wantErr: "--upstream-concurrency must be positive"},
		{name: "interval", args: []string{"-upstream-interval-ms", "-1"}, wantErr: "--upstream-interval-ms must be non-negative"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.args)
			if err == nil {
				t.Fatal("Parse returned nil error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error mismatch: got %q want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestParseRejectsInvalidEnvironmentValues(t *testing.T) {
	tests := []struct {
		name    string
		envName string
		envVal  string
		wantErr string
	}{
		{name: "read timeout", envName: "READ_HEADER_TIMEOUT", envVal: "nope", wantErr: "READ_HEADER_TIMEOUT must be a number"},
		{name: "idle timeout", envName: "IDLE_TIMEOUT", envVal: "nope", wantErr: "IDLE_TIMEOUT must be a number"},
		{name: "upstream timeout", envName: "UPSTREAM_TIMEOUT", envVal: "nope", wantErr: "UPSTREAM_TIMEOUT must be a number"},
		{name: "concurrency", envName: "UPSTREAM_CONCURRENCY", envVal: "nope", wantErr: "UPSTREAM_CONCURRENCY must be an integer"},
		{name: "interval", envName: "UPSTREAM_INTERVAL_MS", envVal: "nope", wantErr: "UPSTREAM_INTERVAL_MS must be a number"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearConfigEnv(t)
			t.Setenv(tt.envName, tt.envVal)

			_, err := Parse(nil)
			if err == nil {
				t.Fatal("Parse returned nil error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error mismatch: got %q want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestWriteUsage(t *testing.T) {
	var buf bytes.Buffer
	WriteUsage(&buf, "edge-tts-compatible")
	got := buf.String()

	for _, want := range []string{
		"Usage:\n  edge-tts-compatible [flags]",
		"Example:\n  edge-tts-compatible --listen :8080 --api-token sk-local --default-voice en-US-EmmaMultilingualNeural",
		"--api-token string",
		"--default-voice string",
		"--idle-timeout float",
		"--listen string",
		"--proxy string",
		"--read-header-timeout float",
		"--upstream-concurrency int",
		"--upstream-interval-ms float",
		"--upstream-timeout float",
		"Each flag can also be set with its equivalent environment variable. Command-line flags take priority. See docker.env.example.",
		"OpenAI Audio Speech: POST /v1/audio/speech, POST /audio/speech",
		"Models:              GET /v1/models",
		"Voices:              GET/POST /v1/voices, /voices, /v1/audio/voices, /audio/voices",
		"Common endpoints:    /health",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("usage missing %q:\n%s", want, got)
		}
	}
}

func clearConfigEnv(t *testing.T) {
	t.Helper()

	for _, name := range envVarNames {
		name := name
		value, ok := os.LookupEnv(name)
		if err := os.Unsetenv(name); err != nil {
			t.Fatalf("Unsetenv(%q) returned error: %v", name, err)
		}
		t.Cleanup(func() {
			var err error
			if ok {
				err = os.Setenv(name, value)
			} else {
				err = os.Unsetenv(name)
			}
			if err != nil {
				t.Fatalf("restore env %q returned error: %v", name, err)
			}
		})
	}
}
