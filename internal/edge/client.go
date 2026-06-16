package edge

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	baseURL             = "speech.platform.bing.com/consumer/speech/synthesize/readaloud"
	wssURL              = "wss://" + baseURL + "/edge/v1?TrustedClientToken=" + trustedClientToken
	voiceListURL        = "https://" + baseURL + "/voices/list?trustedclienttoken=" + trustedClientToken
	defaultVoice        = "en-US-EmmaMultilingualNeural"
	chromiumFullVersion = "143.0.3650.75"
	secMSGECVersion     = "1-" + chromiumFullVersion
	ticksPerSecond      = 10_000_000
	mp3BitrateBPS       = 48_000
	maxTextBytes        = 4096
)

var (
	ratePattern   = regexp.MustCompile(`^[+-]\d+%$`)
	volumePattern = regexp.MustCompile(`^[+-]\d+%$`)
	pitchPattern  = regexp.MustCompile(`^[+-]\d+Hz$`)
)

type Config struct {
	Timeout     time.Duration
	Proxy       string
	Concurrency int
	Interval    time.Duration
}

type RealClient struct {
	httpClient *http.Client
	dialer     *websocket.Dialer
	timeout    time.Duration
	limiter    *upstreamLimiter
}

func NewClient(cfg Config) (*RealClient, error) {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 120 * time.Second
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 10
	}
	if cfg.Interval < 0 {
		return nil, fmt.Errorf("upstream interval must be non-negative")
	}
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		Proxy:           http.ProxyFromEnvironment,
	}
	dialer := &websocket.Dialer{
		Proxy:             http.ProxyFromEnvironment,
		HandshakeTimeout:  45 * time.Second,
		TLSClientConfig:   &tls.Config{MinVersion: tls.VersionTLS12},
		EnableCompression: true,
	}
	if cfg.Proxy != "" {
		proxyURL, err := url.Parse(cfg.Proxy)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
		dialer.Proxy = http.ProxyURL(proxyURL)
	}
	return &RealClient{
		httpClient: &http.Client{Transport: transport, Timeout: cfg.Timeout},
		dialer:     dialer,
		timeout:    cfg.Timeout,
		limiter:    newUpstreamLimiter(cfg.Concurrency, cfg.Interval),
	}, nil
}

func (c *RealClient) ListVoices(ctx context.Context) ([]Voice, error) {
	release, err := c.limiter.acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	voices, err := c.listVoicesOnce(ctx)
	if httpErr := responseDateError(err); httpErr != nil {
		adjustClockSkew(httpErr.serverDate, time.Now())
		return c.listVoicesOnce(ctx)
	}
	return voices, err
}

func (c *RealClient) listVoicesOnce(ctx context.Context) ([]Voice, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, voiceListURL+"&Sec-MS-GEC="+secMSGEC(time.Now())+"&Sec-MS-GEC-Version="+secMSGECVersion, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range voiceHeaders() {
		req.Header.Set(k, v)
	}
	req.Header.Set("Cookie", "muid="+muid()+";")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden {
		return nil, newResponseDateError(resp)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("voice list upstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var voices []Voice
	if err := json.NewDecoder(resp.Body).Decode(&voices); err != nil {
		return nil, err
	}
	for i := range voices {
		if voices[i].VoiceTag.ContentCategories == nil {
			voices[i].VoiceTag.ContentCategories = []string{}
		}
		if voices[i].VoiceTag.VoicePersonalities == nil {
			voices[i].VoiceTag.VoicePersonalities = []string{}
		}
	}
	return voices, nil
}

func (c *RealClient) Synthesize(ctx context.Context, req SynthesizeRequest, handle func(Chunk) error) error {
	req = req.withDefaults()
	if err := req.validate(); err != nil {
		return err
	}
	texts := splitTextByByteLength(escapeInvalid(removeIncompatibleCharacters(req.Text)), maxTextBytes)
	if len(texts) == 0 {
		return errors.New("input text is empty")
	}
	state := synthState{}
	for _, text := range texts {
		state.partialText = text
		state.chunkAudioBytes = 0
		if err := c.streamOnceWithRetry(ctx, req, &state, handle); err != nil {
			return err
		}
	}
	return nil
}

func (c *RealClient) streamOnceWithRetry(ctx context.Context, req SynthesizeRequest, state *synthState, handle func(Chunk) error) error {
	err := c.streamOnce(ctx, req, state, handle)
	if httpErr := responseDateError(err); httpErr != nil {
		adjustClockSkew(httpErr.serverDate, time.Now())
		state.chunkAudioBytes = 0
		return c.streamOnce(ctx, req, state, handle)
	}
	return err
}

func (c *RealClient) streamOnce(ctx context.Context, req SynthesizeRequest, state *synthState, handle func(Chunk) error) error {
	release, err := c.limiter.acquire(ctx)
	if err != nil {
		return err
	}
	defer release()

	connectCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	u := wssURL + "&ConnectionId=" + connectID() + "&Sec-MS-GEC=" + secMSGEC(time.Now()) + "&Sec-MS-GEC-Version=" + secMSGECVersion
	conn, resp, err := c.dialer.DialContext(connectCtx, u, wssHeaders())
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusForbidden {
			return newResponseDateError(resp)
		}
		return err
	}
	defer conn.Close()

	if deadline, ok := connectCtx.Deadline(); ok {
		_ = conn.SetReadDeadline(deadline)
		_ = conn.SetWriteDeadline(deadline)
	}

	if err := conn.WriteMessage(websocket.TextMessage, []byte(speechConfig(req.Boundary))); err != nil {
		return err
	}
	ssml := makeSSML(req, state.partialText)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(ssmlHeadersPlusData(connectID(), dateString(), ssml))); err != nil {
		return err
	}

	audioReceived := false
	for {
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		switch messageType {
		case websocket.TextMessage:
			done, err := c.handleTextMessage(data, state, handle)
			if err != nil {
				return err
			}
			if done {
				if !audioReceived {
					return errors.New("no audio was received; verify voice and synthesis parameters")
				}
				state.compensateOffset()
				return nil
			}
		case websocket.BinaryMessage:
			audio, err := audioFromBinaryMessage(data)
			if err != nil {
				return err
			}
			if len(audio) == 0 {
				continue
			}
			audioReceived = true
			state.chunkAudioBytes += len(audio)
			if err := handle(Chunk{Type: "audio", Data: audio}); err != nil {
				return err
			}
		}
	}
}

func (c *RealClient) handleTextMessage(data []byte, state *synthState, handle func(Chunk) error) (bool, error) {
	headers, payload, err := parseHeadersAndData(data, bytes.Index(data, []byte("\r\n\r\n")))
	if err != nil {
		return false, err
	}
	switch headers["Path"] {
	case "audio.metadata":
		chunk, ok, err := parseMetadata(payload, state.offsetCompensation)
		if err != nil || !ok {
			return false, err
		}
		state.lastDurationOffset = chunk.Offset + chunk.Duration
		return false, handle(chunk)
	case "turn.end":
		return true, nil
	case "response", "turn.start":
		return false, nil
	default:
		return false, fmt.Errorf("unknown websocket text path %q", headers["Path"])
	}
}

type synthState struct {
	partialText        string
	offsetCompensation int64
	lastDurationOffset int64
	chunkAudioBytes    int
	cumulativeBytes    int
}

func (s *synthState) compensateOffset() {
	s.cumulativeBytes += s.chunkAudioBytes
	s.offsetCompensation = int64(s.cumulativeBytes) * 8 * ticksPerSecond / mp3BitrateBPS
	s.chunkAudioBytes = 0
}

func (r SynthesizeRequest) validate() error {
	if strings.TrimSpace(r.Text) == "" {
		return errors.New("input is required")
	}
	if r.Voice == "" {
		return errors.New("voice is required")
	}
	if !ratePattern.MatchString(r.Rate) {
		return fmt.Errorf("invalid rate %q; expected format like +0%% or -50%%", r.Rate)
	}
	if !volumePattern.MatchString(r.Volume) {
		return fmt.Errorf("invalid volume %q; expected format like +0%% or -50%%", r.Volume)
	}
	if !pitchPattern.MatchString(r.Pitch) {
		return fmt.Errorf("invalid pitch %q; expected format like +0Hz or -50Hz", r.Pitch)
	}
	if r.Boundary != "WordBoundary" && r.Boundary != "SentenceBoundary" {
		return fmt.Errorf("invalid boundary %q", r.Boundary)
	}
	return nil
}

func (r SynthesizeRequest) withDefaults() SynthesizeRequest {
	if r.Voice == "" {
		r.Voice = defaultVoice
	}
	if r.Rate == "" {
		r.Rate = "+0%"
	}
	if r.Volume == "" {
		r.Volume = "+0%"
	}
	if r.Pitch == "" {
		r.Pitch = "+0Hz"
	}
	if r.Boundary == "" {
		r.Boundary = "SentenceBoundary"
	}
	return r
}

func parseHeadersAndData(data []byte, headerLength int) (map[string]string, []byte, error) {
	if headerLength < 0 {
		return nil, nil, errors.New("message is missing header separator")
	}
	headers := map[string]string{}
	for _, line := range bytes.Split(data[:headerLength], []byte("\r\n")) {
		parts := bytes.SplitN(line, []byte(":"), 2)
		if len(parts) != 2 {
			return nil, nil, fmt.Errorf("invalid header line %q", string(line))
		}
		headers[string(parts[0])] = string(parts[1])
	}
	start := headerLength + len("\r\n\r\n")
	if start > len(data) {
		return nil, nil, errors.New("header length exceeds message length")
	}
	return headers, data[start:], nil
}

func audioFromBinaryMessage(data []byte) ([]byte, error) {
	if len(data) < 2 {
		return nil, errors.New("binary message missing header length")
	}
	headerLength := int(binary.BigEndian.Uint16(data[:2]))
	if headerLength > len(data)-2 {
		return nil, errors.New("binary header length exceeds message length")
	}
	headerData := data[2 : 2+headerLength]
	payload := data[2+headerLength:]
	headers, _, err := parseHeadersAndData(append(headerData, []byte("\r\n\r\n")...), headerLength)
	if err != nil {
		return nil, err
	}
	if headers["Path"] != "audio" {
		return nil, errors.New("binary message path is not audio")
	}
	contentType, hasContentType := headers["Content-Type"]
	if !hasContentType {
		if len(payload) == 0 {
			return nil, nil
		}
		return nil, errors.New("binary message has payload without Content-Type")
	}
	if contentType != "audio/mpeg" {
		return nil, fmt.Errorf("unexpected audio Content-Type %q", contentType)
	}
	if len(payload) == 0 {
		return nil, errors.New("audio message has empty payload")
	}
	return payload, nil
}

type metadataEnvelope struct {
	Metadata []struct {
		Type string `json:"Type"`
		Data struct {
			Offset   int64 `json:"Offset"`
			Duration int64 `json:"Duration"`
			Text     struct {
				Text string `json:"Text"`
			} `json:"text"`
		} `json:"Data"`
	} `json:"Metadata"`
}

func parseMetadata(data []byte, offsetCompensation int64) (Chunk, bool, error) {
	var envelope metadataEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return Chunk{}, false, err
	}
	for _, item := range envelope.Metadata {
		switch item.Type {
		case "WordBoundary", "SentenceBoundary":
			return Chunk{
				Type:     item.Type,
				Offset:   item.Data.Offset + offsetCompensation,
				Duration: item.Data.Duration,
				Text:     html.UnescapeString(item.Data.Text.Text),
			}, true, nil
		case "SessionEnd":
			continue
		default:
			return Chunk{}, false, fmt.Errorf("unknown metadata type %q", item.Type)
		}
	}
	return Chunk{}, false, nil
}

func makeSSML(req SynthesizeRequest, escapedText string) string {
	return "<speak version='1.0' xmlns='http://www.w3.org/2001/10/synthesis' xml:lang='en-US'>" +
		"<voice name='" + xmlEscapeAttr(normalizeVoice(req.Voice)) + "'>" +
		"<prosody pitch='" + xmlEscapeAttr(req.Pitch) + "' rate='" + xmlEscapeAttr(req.Rate) + "' volume='" + xmlEscapeAttr(req.Volume) + "'>" +
		escapedText +
		"</prosody></voice></speak>"
}

func speechConfig(boundary string) string {
	wordBoundary := boundary == "WordBoundary"
	wd := "false"
	sq := "true"
	if wordBoundary {
		wd = "true"
		sq = "false"
	}
	return "X-Timestamp:" + dateString() + "\r\n" +
		"Content-Type:application/json; charset=utf-8\r\n" +
		"Path:speech.config\r\n\r\n" +
		`{"context":{"synthesis":{"audio":{"metadataoptions":{"sentenceBoundaryEnabled":"` + sq + `","wordBoundaryEnabled":"` + wd + `"},"outputFormat":"audio-24khz-48kbitrate-mono-mp3"}}}}` +
		"\r\n"
}

func ssmlHeadersPlusData(requestID, timestamp, ssml string) string {
	return "X-RequestId:" + requestID + "\r\n" +
		"Content-Type:application/ssml+xml\r\n" +
		"X-Timestamp:" + timestamp + "Z\r\n" +
		"Path:ssml\r\n\r\n" +
		ssml
}

func dateString() string {
	return time.Now().UTC().Format("Mon Jan 02 2006 15:04:05 GMT+0000 (Coordinated Universal Time)")
}

func normalizeVoice(voice string) string {
	re := regexp.MustCompile(`^([a-z]{2,})-([A-Z]{2,})-(.+Neural)$`)
	match := re.FindStringSubmatch(voice)
	if match == nil {
		return voice
	}
	lang, region, name := match[1], match[2], match[3]
	if idx := strings.Index(name, "-"); idx >= 0 {
		region += "-" + name[:idx]
		name = name[idx+1:]
	}
	return "Microsoft Server Speech Text to Speech Voice (" + lang + "-" + region + ", " + name + ")"
}

func removeIncompatibleCharacters(text string) string {
	var b strings.Builder
	for _, r := range text {
		if (r >= 0 && r <= 8) || (r >= 11 && r <= 12) || (r >= 14 && r <= 31) {
			b.WriteRune(' ')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func escapeInvalid(text string) string {
	return html.EscapeString(text)
}

func xmlEscapeAttr(text string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(text))
	return b.String()
}

func splitTextByByteLength(text string, limit int) []string {
	raw := []byte(text)
	out := []string{}
	for len(raw) > limit {
		splitAt := findLastNewlineOrSpace(raw, limit)
		if splitAt < 0 {
			splitAt = safeUTF8SplitPoint(raw[:limit])
		}
		splitAt = adjustSplitPointForXMLEntity(raw, splitAt)
		if splitAt <= 0 {
			splitAt = safeUTF8SplitPoint(raw[:limit])
		}
		if splitAt <= 0 {
			splitAt = limit
		}
		chunk := bytes.TrimSpace(raw[:splitAt])
		if len(chunk) > 0 {
			out = append(out, string(chunk))
		}
		raw = raw[splitAt:]
	}
	chunk := bytes.TrimSpace(raw)
	if len(chunk) > 0 {
		out = append(out, string(chunk))
	}
	return out
}

func findLastNewlineOrSpace(text []byte, limit int) int {
	if idx := bytes.LastIndexByte(text[:limit], '\n'); idx >= 0 {
		return idx
	}
	return bytes.LastIndexByte(text[:limit], ' ')
}

func safeUTF8SplitPoint(segment []byte) int {
	for i := len(segment); i > 0; i-- {
		if string(segment[:i]) != "" && bytes.Equal([]byte(string(segment[:i])), segment[:i]) {
			return i
		}
	}
	return 0
}

func adjustSplitPointForXMLEntity(text []byte, splitAt int) int {
	for splitAt > 0 && bytes.Contains(text[:splitAt], []byte("&")) {
		amp := bytes.LastIndexByte(text[:splitAt], '&')
		if bytes.IndexByte(text[amp:splitAt], ';') != -1 {
			break
		}
		splitAt = amp
	}
	return splitAt
}

func connectID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strings.ReplaceAll(fmt.Sprintf("%032x", time.Now().UnixNano()), "-", "")
	}
	return hex.EncodeToString(b[:])
}

func baseHeaders() map[string]string {
	major := strings.SplitN(chromiumFullVersion, ".", 2)[0]
	return map[string]string{
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" + major + ".0.0.0 Safari/537.36 Edg/" + major + ".0.0.0",
		"Accept-Encoding": "gzip, deflate, br, zstd",
		"Accept-Language": "en-US,en;q=0.9",
	}
}

func voiceHeaders() map[string]string {
	major := strings.SplitN(chromiumFullVersion, ".", 2)[0]
	headers := baseHeaders()
	headers["Authority"] = "speech.platform.bing.com"
	headers["Sec-CH-UA"] = `" Not;A Brand";v="99", "Microsoft Edge";v="` + major + `", "Chromium";v="` + major + `"`
	headers["Sec-CH-UA-Mobile"] = "?0"
	headers["Accept"] = "*/*"
	headers["Sec-Fetch-Site"] = "none"
	headers["Sec-Fetch-Mode"] = "cors"
	headers["Sec-Fetch-Dest"] = "empty"
	return headers
}

func wssHeaders() http.Header {
	headers := http.Header{}
	for k, v := range baseHeaders() {
		headers.Set(k, v)
	}
	headers.Set("Pragma", "no-cache")
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Origin", "chrome-extension://jdiccldimpdaibmpdkjnbmckianbfold")
	headers.Set("Cookie", "muid="+muid()+";")
	return headers
}

type upstreamDateError struct {
	statusCode int
	serverDate time.Time
}

func (e upstreamDateError) Error() string {
	return fmt.Sprintf("upstream status %d", e.statusCode)
}

func newResponseDateError(resp *http.Response) error {
	dateHeader := resp.Header.Get("Date")
	if dateHeader == "" {
		return fmt.Errorf("upstream status %d without Date header", resp.StatusCode)
	}
	serverDate, err := http.ParseTime(dateHeader)
	if err != nil {
		return fmt.Errorf("parse upstream Date header: %w", err)
	}
	return upstreamDateError{statusCode: resp.StatusCode, serverDate: serverDate}
}

func responseDateError(err error) *upstreamDateError {
	var target upstreamDateError
	if errors.As(err, &target) {
		return &target
	}
	return nil
}
