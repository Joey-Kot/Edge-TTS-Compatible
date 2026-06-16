package edge

import (
	"bytes"
	"context"
	"encoding/binary"
	"strings"
	"testing"
	"time"
)

func TestAudioFromBinaryMessageAllowsTrailingHeaderNewline(t *testing.T) {
	header := []byte("Path:audio\r\nContent-Type:audio/mpeg\r\n")
	payload := []byte{0x01, 0x02, 0x03}
	message := make([]byte, 2+len(header)+len(payload))
	binary.BigEndian.PutUint16(message[:2], uint16(len(header)))
	copy(message[2:], header)
	copy(message[2+len(header):], payload)

	audio, err := audioFromBinaryMessage(message)
	if err != nil {
		t.Fatalf("audioFromBinaryMessage returned error: %v", err)
	}
	if string(audio) != string(payload) {
		t.Fatalf("audio payload mismatch: got %v want %v", audio, payload)
	}
}

func TestAudioFromBinaryMessageAllowsTerminalEmptyAudioMessage(t *testing.T) {
	header := []byte("Path:audio\r\n")
	message := make([]byte, 2+len(header))
	binary.BigEndian.PutUint16(message[:2], uint16(len(header)))
	copy(message[2:], header)

	audio, err := audioFromBinaryMessage(message)
	if err != nil {
		t.Fatalf("audioFromBinaryMessage returned error: %v", err)
	}
	if len(audio) != 0 {
		t.Fatalf("audio payload mismatch: got %v want empty", audio)
	}
}

func TestParseHeadersAndDataSkipsEmptyLines(t *testing.T) {
	data := []byte("Path:audio.metadata\r\nX-RequestId:abc\r\n\r\npayload")
	headers, payload, err := parseHeadersAndData(data, bytes.Index(data, []byte("\r\n\r\n")))
	if err != nil {
		t.Fatalf("parseHeadersAndData returned error: %v", err)
	}
	if headers["Path"] != "audio.metadata" {
		t.Fatalf("Path mismatch: got %q", headers["Path"])
	}
	if headers["X-RequestId"] != "abc" {
		t.Fatalf("X-RequestId mismatch: got %q", headers["X-RequestId"])
	}
	if string(payload) != "payload" {
		t.Fatalf("payload mismatch: got %q", string(payload))
	}
}

func TestParseHeadersAndDataRejectsMalformedLine(t *testing.T) {
	data := []byte("Path:audio\r\nbroken\r\n\r\n")
	_, _, err := parseHeadersAndData(data, bytes.Index(data, []byte("\r\n\r\n")))
	if err == nil {
		t.Fatal("parseHeadersAndData returned nil error")
	}
	if !strings.Contains(err.Error(), "invalid header line") {
		t.Fatalf("error mismatch: got %v", err)
	}
}

func TestParseMetadata(t *testing.T) {
	data := []byte(`{"Metadata":[{"Type":"SentenceBoundary","Data":{"Offset":10,"Duration":20,"text":{"Text":"Tom &amp; Jerry"}}}]}`)

	chunk, ok, err := parseMetadata(data, 100)
	if err != nil {
		t.Fatalf("parseMetadata returned error: %v", err)
	}
	if !ok {
		t.Fatal("parseMetadata returned ok=false")
	}
	if chunk.Type != "SentenceBoundary" || chunk.Offset != 110 || chunk.Duration != 20 || chunk.Text != "Tom & Jerry" {
		t.Fatalf("chunk mismatch: %+v", chunk)
	}
}

func TestParseMetadataIgnoresSessionEnd(t *testing.T) {
	data := []byte(`{"Metadata":[{"Type":"SessionEnd","Data":{"Offset":0,"Duration":0,"text":{"Text":""}}}]}`)

	_, ok, err := parseMetadata(data, 0)
	if err != nil {
		t.Fatalf("parseMetadata returned error: %v", err)
	}
	if ok {
		t.Fatal("parseMetadata returned ok=true")
	}
}

func TestMakeSSMLEscapesAttributesAndText(t *testing.T) {
	req := SynthesizeRequest{
		Text:   "ignored",
		Voice:  `x"y`,
		Rate:   "+0%",
		Volume: "+0%",
		Pitch:  `+0"Hz`,
	}

	ssml := makeSSML(req, escapeInvalid(`<hello & goodbye>`))

	if !strings.Contains(ssml, "name='x&#34;y'") {
		t.Fatalf("SSML voice attribute was not escaped: %s", ssml)
	}
	if !strings.Contains(ssml, "pitch='+0&#34;Hz'") {
		t.Fatalf("SSML pitch attribute was not escaped: %s", ssml)
	}
	if !strings.Contains(ssml, "&lt;hello &amp; goodbye&gt;") {
		t.Fatalf("SSML text was not escaped: %s", ssml)
	}
}

func TestSpeechConfigBoundaryFlags(t *testing.T) {
	word := speechConfig("WordBoundary")
	if !strings.Contains(word, `"sentenceBoundaryEnabled":"false"`) || !strings.Contains(word, `"wordBoundaryEnabled":"true"`) {
		t.Fatalf("WordBoundary config mismatch: %s", word)
	}

	sentence := speechConfig("SentenceBoundary")
	if !strings.Contains(sentence, `"sentenceBoundaryEnabled":"true"`) || !strings.Contains(sentence, `"wordBoundaryEnabled":"false"`) {
		t.Fatalf("SentenceBoundary config mismatch: %s", sentence)
	}
}

func TestSynthesizeRequestDefaultsAndValidation(t *testing.T) {
	req := SynthesizeRequest{Text: "hello"}.withDefaults()

	if req.Voice != defaultVoice || req.Rate != "+0%" || req.Volume != "+0%" || req.Pitch != "+0Hz" || req.Boundary != "SentenceBoundary" {
		t.Fatalf("defaults mismatch: %+v", req)
	}
	if err := req.validate(); err != nil {
		t.Fatalf("validate returned error: %v", err)
	}

	invalid := req
	invalid.Rate = "fast"
	if err := invalid.validate(); err == nil || !strings.Contains(err.Error(), "invalid rate") {
		t.Fatalf("invalid rate error mismatch: %v", err)
	}
}

func TestNormalizeVoice(t *testing.T) {
	got := normalizeVoice("zh-CN-liaoning-XiaobeiNeural")
	want := "Microsoft Server Speech Text to Speech Voice (zh-CN-liaoning, XiaobeiNeural)"
	if got != want {
		t.Fatalf("normalizeVoice mismatch: got %q want %q", got, want)
	}

	unchanged := normalizeVoice("Microsoft Server Speech Text to Speech Voice (en-US, EmmaNeural)")
	if unchanged != "Microsoft Server Speech Text to Speech Voice (en-US, EmmaNeural)" {
		t.Fatalf("normalizeVoice should leave full names unchanged: got %q", unchanged)
	}
}

func TestSplitTextByByteLengthPreservesUTF8AndEntities(t *testing.T) {
	text := "你好 &amp; world 你好"
	parts := splitTextByByteLength(text, 12)
	if len(parts) < 2 {
		t.Fatalf("expected split text, got %v", parts)
	}
	joined := strings.Join(parts, "")
	if strings.ReplaceAll(joined, " ", "") != strings.ReplaceAll(text, " ", "") {
		t.Fatalf("split text did not preserve content: parts=%v", parts)
	}
	for _, part := range parts {
		if strings.Contains(part, "&am") && !strings.Contains(part, "&amp;") {
			t.Fatalf("split broke XML entity: %v", parts)
		}
	}
}

func TestRemoveIncompatibleCharacters(t *testing.T) {
	got := removeIncompatibleCharacters("a\x00b\x0bc\x1fd")
	if got != "a b c d" {
		t.Fatalf("removeIncompatibleCharacters mismatch: got %q", got)
	}
}

func TestUpstreamLimiterQueuesAndRespectsInterval(t *testing.T) {
	limiter := newUpstreamLimiter(1, 25*time.Millisecond)
	ctx := context.Background()

	release, err := limiter.acquire(ctx)
	if err != nil {
		t.Fatalf("first acquire returned error: %v", err)
	}

	acquired := make(chan time.Time, 1)
	go func() {
		release2, err := limiter.acquire(ctx)
		if err != nil {
			t.Errorf("second acquire returned error: %v", err)
			return
		}
		defer release2()
		acquired <- time.Now()
	}()

	select {
	case <-acquired:
		t.Fatal("second acquire should wait while first slot is held")
	case <-time.After(10 * time.Millisecond):
	}

	start := time.Now()
	release()

	select {
	case acquiredAt := <-acquired:
		if acquiredAt.Sub(start) < 8*time.Millisecond {
			t.Fatalf("second acquire did not respect interval: waited %s", acquiredAt.Sub(start))
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("second acquire timed out")
	}
}

func TestUpstreamLimiterAcquireCanBeCanceledWhileQueued(t *testing.T) {
	limiter := newUpstreamLimiter(1, 0)
	release, err := limiter.acquire(context.Background())
	if err != nil {
		t.Fatalf("first acquire returned error: %v", err)
	}
	defer release()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := limiter.acquire(ctx); err == nil {
		t.Fatal("second acquire returned nil error")
	}
}
