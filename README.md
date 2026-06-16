# Edge-TTS-Compatible

OpenAI-compatible speech API backed directly by Microsoft Edge's TTS endpoint.

This service implements the reverse-engineered Edge TTS HTTP/WebSocket protocol from `rany2/edge-tts` in Go. It exposes a small OpenAI-style API:

- `POST /v1/audio/speech`
- `POST /audio/speech`
- `GET /v1/models`
- `GET|POST /v1/voices`

The only exposed model is `edge`. Incoming `model` values are ignored.

## Request

```json
{
  "model": "anything",
  "input": "Hello world.",
  "voice": "en-US-EmmaMultilingualNeural",
  "speed": 1.0,
  "volume": "+0%",
  "pitch": "+0Hz",
  "stream_format": "audio"
}
```

Fields:

- `input`: required text.
- `voice`: Edge voice short name, or object `{ "id": "en-US-EmmaMultilingualNeural" }`.
- `speed`: OpenAI-style multiplier. `1.0` becomes Edge `rate="+0%"`, `1.5` becomes `+50%`, `0.5` becomes `-50%`.
- `volume`: Edge prosody volume, for example `+0%`, `-50%`.
- `pitch`: Edge prosody pitch, for example `+0Hz`, `-50Hz`.
- `stream_format`: `audio` for raw MP3 bytes, `sse` for base64 audio delta events.
- `response_format`: only `mp3` is accepted. The real Edge endpoint emits MP3.

## Upstream throttling

Requests from clients are not rejected when the Edge upstream is busy. The server queues them internally before opening the real Edge TTS HTTP/WebSocket request.

Defaults:

- Maximum concurrent Edge upstream requests: `10`
- Minimum global interval between any two Edge upstream requests: `500ms`

## Run

```bash
go run ./cmd/server -listen :8080 -api-token sk-local
```

```bash
curl http://localhost:8080/v1/audio/speech \
  -H 'Authorization: Bearer sk-local' \
  -H 'Content-Type: application/json' \
  -d '{"model":"edge","input":"Hello world.","voice":"en-US-EmmaMultilingualNeural"}' \
  --output speech.mp3
```

SSE:

```bash
curl http://localhost:8080/v1/audio/speech \
  -H 'Authorization: Bearer sk-local' \
  -H 'Content-Type: application/json' \
  -d '{"model":"edge","input":"Hello world.","voice":"en-US-EmmaMultilingualNeural","stream_format":"sse"}'
```

## Flags

```text
-listen string
-api-token string
-default-voice string
-proxy string
-upstream-timeout float
-upstream-concurrency int
-upstream-interval-ms float
-read-header-timeout float
-idle-timeout float
```

If `-api-token` is empty, local authentication is disabled.
