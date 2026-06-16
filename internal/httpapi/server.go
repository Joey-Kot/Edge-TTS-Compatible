package httpapi

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"edge-tts-compatible/internal/config"
	"edge-tts-compatible/internal/edge"
	"edge-tts-compatible/internal/sse"
)

type EdgeClient interface {
	Synthesize(ctx context.Context, req edge.SynthesizeRequest, handle func(edge.Chunk) error) error
	ListVoices(ctx context.Context) ([]edge.Voice, error)
}

type Server struct {
	cfg    config.Config
	client EdgeClient
}

const maxRequestBodyBytes = 1 << 20

func New(cfg config.Config, client EdgeClient) *Server {
	return &Server{cfg: cfg, client: client}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.setCommonHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.URL.Path == "/health" {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
		return
	}
	if !s.authorize(w, r) {
		return
	}

	path := strings.TrimRight(r.URL.Path, "/")
	if path == "" {
		path = "/"
	}
	switch {
	case path == "/v1/audio/speech" || path == "/audio/speech":
		s.handleSpeech(w, r)
	case r.Method == http.MethodGet && path == "/v1/models":
		s.handleModels(w, r)
	case path == "/v1/voices" || path == "/voices" || path == "/v1/audio/voices" || path == "/audio/voices":
		s.handleVoices(w, r)
	default:
		openAIError(w, http.StatusNotFound, "not found", "invalid_request_error")
	}
}

func (s *Server) handleSpeech(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	payload, ok := s.readJSON(w, r)
	if !ok {
		return
	}
	req, streamFormat, err := s.speechRequest(payload)
	if err != nil {
		openAIError(w, http.StatusBadRequest, err.Error(), "invalid_request_error")
		return
	}
	switch streamFormat {
	case "", "audio":
		s.speechAudio(w, r, req)
	case "sse":
		s.speechSSE(w, r, req)
	default:
		openAIError(w, http.StatusBadRequest, "stream_format must be audio or sse", "invalid_request_error")
	}
}

func (s *Server) speechRequest(payload map[string]any) (edge.SynthesizeRequest, string, error) {
	input, ok := payload["input"].(string)
	if !ok || strings.TrimSpace(input) == "" {
		return edge.SynthesizeRequest{}, "", errors.New("input is required")
	}
	if utf8.RuneCountInString(input) > 4096 {
		return edge.SynthesizeRequest{}, "", errors.New("input must be at most 4096 characters")
	}
	voice := s.cfg.DefaultVoice
	if rawVoice, ok := payload["voice"]; ok {
		value, err := voiceValue(rawVoice)
		if err != nil {
			return edge.SynthesizeRequest{}, "", err
		}
		if value != "" {
			voice = value
		}
	}
	if format := stringValue(payload["response_format"]); format != "" && format != "mp3" {
		return edge.SynthesizeRequest{}, "", errors.New("response_format must be mp3; Edge TTS upstream emits MP3 only")
	}
	rate, err := speedToRate(payload["speed"])
	if err != nil {
		return edge.SynthesizeRequest{}, "", err
	}
	volume := stringValue(payload["volume"])
	if volume == "" {
		volume = "+0%"
	}
	pitch := stringValue(payload["pitch"])
	if pitch == "" {
		pitch = "+0Hz"
	}
	boundary := stringValue(payload["boundary"])
	if boundary == "" {
		boundary = "SentenceBoundary"
	}
	return edge.SynthesizeRequest{
		Text:     input,
		Voice:    voice,
		Rate:     rate,
		Volume:   volume,
		Pitch:    pitch,
		Boundary: boundary,
	}, stringValue(payload["stream_format"]), nil
}

func (s *Server) speechAudio(w http.ResponseWriter, r *http.Request, req edge.SynthesizeRequest) {
	flusher, _ := w.(http.Flusher)
	wroteHeader := false
	err := s.client.Synthesize(r.Context(), req, func(chunk edge.Chunk) error {
		if chunk.Type == "audio" {
			if !wroteHeader {
				w.Header().Set("Content-Type", "audio/mpeg")
				w.WriteHeader(http.StatusOK)
				wroteHeader = true
			}
			_, err := w.Write(chunk.Data)
			if flusher != nil {
				flusher.Flush()
			}
			return err
		}
		return nil
	})
	if err != nil {
		if !wroteHeader {
			s.upstreamError(w, err)
			return
		}
		log.Printf("speech audio upstream/write error: %v", err)
		return
	}
}

func (s *Server) speechSSE(w http.ResponseWriter, r *http.Request, req edge.SynthesizeRequest) {
	setSSEHeaders(w)
	flusher, _ := w.(http.Flusher)
	inputWords := len(strings.Fields(req.Text))
	err := s.client.Synthesize(r.Context(), req, func(chunk edge.Chunk) error {
		if chunk.Type != "audio" {
			return nil
		}
		if err := sse.Data(w, map[string]any{
			"type":  "speech.audio.delta",
			"audio": base64.StdEncoding.EncodeToString(chunk.Data),
		}); err != nil {
			return err
		}
		if flusher != nil {
			flusher.Flush()
		}
		return nil
	})
	if err != nil {
		_ = sse.Data(w, errorPayload(err.Error(), "server_error"))
		if flusher != nil {
			flusher.Flush()
		}
		return
	}
	_ = sse.Data(w, map[string]any{
		"type": "speech.audio.done",
		"usage": map[string]any{
			"input_tokens":  inputWords,
			"output_tokens": 0,
			"total_tokens":  inputWords,
		},
	})
	if flusher != nil {
		flusher.Flush()
	}
}

func (s *Server) handleModels(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"object": "list",
		"data": []any{
			map[string]any{"id": config.DefaultModel, "object": "model", "owned_by": "edge-tts"},
		},
	})
}

func (s *Server) handleVoices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	voices, err := s.client.ListVoices(r.Context())
	if err != nil {
		s.upstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": voices})
}

func (s *Server) readJSON(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBodyBytes+1))
	if err != nil {
		openAIError(w, http.StatusBadRequest, "Request body could not be read", "invalid_request_error")
		return nil, false
	}
	if len(body) > maxRequestBodyBytes {
		openAIError(w, http.StatusRequestEntityTooLarge, "Request body is too large", "invalid_request_error")
		return nil, false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var payload any
	if err := decoder.Decode(&payload); err != nil {
		openAIError(w, http.StatusBadRequest, "Request body must be JSON", "invalid_request_error")
		return nil, false
	}
	result, ok := payload.(map[string]any)
	if !ok {
		openAIError(w, http.StatusBadRequest, "request body must be a JSON object", "invalid_request_error")
		return nil, false
	}
	return result, true
}

func (s *Server) authorize(w http.ResponseWriter, r *http.Request) bool {
	if len(s.cfg.APITokens) == 0 {
		return true
	}
	if token := r.Header.Get("x-api-key"); token != "" && s.tokenMatches(token) {
		return true
	}
	auth := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		w.Header().Set("WWW-Authenticate", "Bearer")
		openAIError(w, http.StatusUnauthorized, "Missing API key or Authorization Bearer token", "authentication_error")
		return false
	}
	token := strings.TrimPrefix(auth, prefix)
	if s.tokenMatches(token) {
		return true
	}
	w.Header().Set("WWW-Authenticate", "Bearer")
	openAIError(w, http.StatusUnauthorized, "Invalid authentication token", "authentication_error")
	return false
}

func (s *Server) tokenMatches(token string) bool {
	for _, expected := range s.cfg.APITokens {
		if subtle.ConstantTimeCompare([]byte(token), []byte(expected)) == 1 {
			return true
		}
	}
	return false
}

func (s *Server) setCommonHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "*")
	w.Header().Set("Access-Control-Allow-Headers", "*")
}

func (s *Server) upstreamError(w http.ResponseWriter, err error) {
	openAIError(w, http.StatusBadGateway, err.Error(), "server_error")
}

func voiceValue(raw any) (string, error) {
	switch v := raw.(type) {
	case string:
		return v, nil
	case map[string]any:
		id, _ := v["id"].(string)
		if id == "" {
			return "", errors.New("voice.id is required")
		}
		return id, nil
	default:
		return "", errors.New("voice must be a string or an object with id")
	}
}

func speedToRate(raw any) (string, error) {
	if raw == nil {
		return "+0%", nil
	}
	if text, ok := raw.(string); ok {
		if strings.HasPrefix(text, "+") || strings.HasPrefix(text, "-") {
			return text, nil
		}
		value, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return "", errors.New("speed must be a number or Edge rate string")
		}
		return speedNumberToRate(value)
	}
	switch v := raw.(type) {
	case json.Number:
		value, err := v.Float64()
		if err != nil {
			return "", errors.New("speed must be a number")
		}
		return speedNumberToRate(value)
	case float64:
		return speedNumberToRate(v)
	case int:
		return speedNumberToRate(float64(v))
	default:
		return "", errors.New("speed must be a number")
	}
}

func speedNumberToRate(speed float64) (string, error) {
	if speed <= 0 {
		return "", errors.New("speed must be positive")
	}
	change := math.Round((speed - 1) * 100)
	return fmt.Sprintf("%+d%%", int(change)), nil
}

func stringValue(raw any) string {
	switch v := raw.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	default:
		return ""
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("write json response: %v", err)
	}
}

func openAIError(w http.ResponseWriter, status int, message, typ string) {
	writeJSON(w, status, errorPayload(message, typ))
}

func errorPayload(message, typ string) map[string]any {
	return map[string]any{"error": map[string]any{"message": message, "type": typ, "param": nil, "code": nil}}
}

func methodNotAllowed(w http.ResponseWriter) {
	openAIError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error")
}

func setSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
}
