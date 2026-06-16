package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"edge-tts-compatible/internal/config"
	"edge-tts-compatible/internal/edge"
)

type fakeEdgeClient struct {
	synthesizeErr error
	voicesErr     error
	chunks        []edge.Chunk
	voices        []edge.Voice
	lastReq       edge.SynthesizeRequest
}

func (f *fakeEdgeClient) Synthesize(_ context.Context, req edge.SynthesizeRequest, handle func(edge.Chunk) error) error {
	f.lastReq = req
	if f.synthesizeErr != nil {
		return f.synthesizeErr
	}
	for _, chunk := range f.chunks {
		if err := handle(chunk); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeEdgeClient) ListVoices(_ context.Context) ([]edge.Voice, error) {
	if f.voicesErr != nil {
		return nil, f.voicesErr
	}
	return f.voices, nil
}

func TestSpeechAudioMapsOpenAIRequestToEdgeRequest(t *testing.T) {
	client := &fakeEdgeClient{chunks: []edge.Chunk{{Type: "audio", Data: []byte("mp3")}}}
	server := New(config.Config{DefaultVoice: "default-voice"}, client)

	resp := performRequest(server, http.MethodPost, "/v1/audio/speech", `{
		"model":"ignored",
		"input":"hello",
		"voice":{"id":"en-US-EmmaMultilingualNeural"},
		"response_format":"mp3",
		"stream_format":"audio",
		"speed":1.5,
		"volume":"-20%",
		"pitch":"+10Hz",
		"boundary":"WordBoundary"
	}`, nil)

	if resp.Code != http.StatusOK {
		t.Fatalf("status mismatch: got %d body %s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get("Content-Type"); got != "audio/mpeg" {
		t.Fatalf("Content-Type mismatch: got %q", got)
	}
	if resp.Body.String() != "mp3" {
		t.Fatalf("body mismatch: got %q", resp.Body.String())
	}
	want := edge.SynthesizeRequest{
		Text:     "hello",
		Voice:    "en-US-EmmaMultilingualNeural",
		Rate:     "+50%",
		Volume:   "-20%",
		Pitch:    "+10Hz",
		Boundary: "WordBoundary",
	}
	if client.lastReq != want {
		t.Fatalf("SynthesizeRequest mismatch: got %+v want %+v", client.lastReq, want)
	}
}

func TestSpeechUsesDefaultsAndStringSpeed(t *testing.T) {
	client := &fakeEdgeClient{chunks: []edge.Chunk{{Type: "audio", Data: []byte("x")}}}
	server := New(config.Config{DefaultVoice: "default-voice"}, client)

	resp := performRequest(server, http.MethodPost, "/audio/speech", `{"input":"hello","speed":"+12%"}`, nil)

	if resp.Code != http.StatusOK {
		t.Fatalf("status mismatch: got %d body %s", resp.Code, resp.Body.String())
	}
	want := edge.SynthesizeRequest{
		Text:     "hello",
		Voice:    "default-voice",
		Rate:     "+12%",
		Volume:   "+0%",
		Pitch:    "+0Hz",
		Boundary: "SentenceBoundary",
	}
	if client.lastReq != want {
		t.Fatalf("SynthesizeRequest mismatch: got %+v want %+v", client.lastReq, want)
	}
}

func TestSpeechSSEWritesAudioDeltaAndDone(t *testing.T) {
	client := &fakeEdgeClient{chunks: []edge.Chunk{
		{Type: "audio", Data: []byte("ab")},
		{Type: "SentenceBoundary", Text: "ignored"},
		{Type: "audio", Data: []byte("cd")},
	}}
	server := New(config.Config{DefaultVoice: "default-voice"}, client)

	resp := performRequest(server, http.MethodPost, "/v1/audio/speech", `{"input":"hello world","stream_format":"sse"}`, nil)

	if resp.Code != http.StatusOK {
		t.Fatalf("status mismatch: got %d body %s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type mismatch: got %q", got)
	}
	body := resp.Body.String()
	if !strings.Contains(body, `"type":"speech.audio.delta"`) {
		t.Fatalf("SSE body missing delta event: %s", body)
	}
	if !strings.Contains(body, `"audio":"`+base64.StdEncoding.EncodeToString([]byte("ab"))+`"`) {
		t.Fatalf("SSE body missing first audio chunk: %s", body)
	}
	if !strings.Contains(body, `"type":"speech.audio.done"`) {
		t.Fatalf("SSE body missing done event: %s", body)
	}
	if !strings.Contains(body, `"total_tokens":2`) {
		t.Fatalf("SSE body missing usage: %s", body)
	}
}

func TestSpeechRejectsInvalidRequests(t *testing.T) {
	server := New(config.Config{DefaultVoice: "default-voice"}, &fakeEdgeClient{})
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "missing input", body: `{}`, want: "input is required"},
		{name: "invalid voice", body: `{"input":"x","voice":1}`, want: "voice must be a string or an object with id"},
		{name: "invalid response format", body: `{"input":"x","response_format":"wav"}`, want: "response_format must be mp3"},
		{name: "invalid stream format", body: `{"input":"x","stream_format":"json"}`, want: "stream_format must be audio or sse"},
		{name: "invalid speed", body: `{"input":"x","speed":0}`, want: "speed must be positive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := performRequest(server, http.MethodPost, "/v1/audio/speech", tt.body, nil)
			if resp.Code != http.StatusBadRequest {
				t.Fatalf("status mismatch: got %d body %s", resp.Code, resp.Body.String())
			}
			if !strings.Contains(resp.Body.String(), tt.want) {
				t.Fatalf("body mismatch: got %s want containing %q", resp.Body.String(), tt.want)
			}
		})
	}
}

func TestAuthAcceptsBearerAndXAPIKey(t *testing.T) {
	cfg := config.Config{DefaultVoice: "default-voice", APITokens: []string{"sk-a", "sk-b"}}
	server := New(cfg, &fakeEdgeClient{})

	missing := performRequest(server, http.MethodGet, "/v1/models", "", nil)
	if missing.Code != http.StatusUnauthorized {
		t.Fatalf("missing token status mismatch: got %d", missing.Code)
	}

	bearer := performRequest(server, http.MethodGet, "/v1/models", "", map[string]string{"Authorization": "Bearer sk-a"})
	if bearer.Code != http.StatusOK {
		t.Fatalf("bearer status mismatch: got %d body %s", bearer.Code, bearer.Body.String())
	}

	xAPIKey := performRequest(server, http.MethodGet, "/v1/models", "", map[string]string{"x-api-key": "sk-b"})
	if xAPIKey.Code != http.StatusOK {
		t.Fatalf("x-api-key status mismatch: got %d body %s", xAPIKey.Code, xAPIKey.Body.String())
	}
}

func TestModelsAndVoices(t *testing.T) {
	client := &fakeEdgeClient{voices: []edge.Voice{{ShortName: "en-US-EmmaMultilingualNeural"}}}
	server := New(config.Config{DefaultVoice: "default-voice"}, client)

	models := performRequest(server, http.MethodGet, "/v1/models", "", nil)
	if models.Code != http.StatusOK {
		t.Fatalf("models status mismatch: got %d body %s", models.Code, models.Body.String())
	}
	var modelPayload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(models.Body.Bytes(), &modelPayload); err != nil {
		t.Fatalf("decode models: %v", err)
	}
	if len(modelPayload.Data) != 1 || modelPayload.Data[0].ID != "edge" {
		t.Fatalf("models payload mismatch: %+v", modelPayload)
	}

	voices := performRequest(server, http.MethodPost, "/v1/audio/voices", "", nil)
	if voices.Code != http.StatusOK {
		t.Fatalf("voices status mismatch: got %d body %s", voices.Code, voices.Body.String())
	}
	if !strings.Contains(voices.Body.String(), "en-US-EmmaMultilingualNeural") {
		t.Fatalf("voices body mismatch: %s", voices.Body.String())
	}
}

func TestUpstreamErrorBeforeAudioReturnsBadGateway(t *testing.T) {
	server := New(config.Config{DefaultVoice: "default-voice"}, &fakeEdgeClient{synthesizeErr: errors.New("upstream failed")})

	resp := performRequest(server, http.MethodPost, "/v1/audio/speech", `{"input":"hello"}`, nil)

	if resp.Code != http.StatusBadGateway {
		t.Fatalf("status mismatch: got %d body %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "upstream failed") {
		t.Fatalf("body mismatch: %s", resp.Body.String())
	}
}

func TestHealthAndOptionsBypassAuth(t *testing.T) {
	server := New(config.Config{DefaultVoice: "default-voice", APITokens: []string{"sk"}}, &fakeEdgeClient{})

	health := performRequest(server, http.MethodGet, "/health", "", nil)
	if health.Code != http.StatusOK {
		t.Fatalf("health status mismatch: got %d body %s", health.Code, health.Body.String())
	}

	options := performRequest(server, http.MethodOptions, "/v1/audio/speech", "", nil)
	if options.Code != http.StatusNoContent {
		t.Fatalf("options status mismatch: got %d body %s", options.Code, options.Body.String())
	}
}

func performRequest(handler http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}
