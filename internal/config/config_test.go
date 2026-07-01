package config

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseDefaults(t *testing.T) {
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

func TestParseOverrides(t *testing.T) {
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
		"docker-entrypoint.sh maps environment variables to the same flags. See docker.env.example.",
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
