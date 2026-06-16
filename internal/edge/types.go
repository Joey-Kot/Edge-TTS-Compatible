package edge

import "context"

type Client interface {
	Synthesize(ctx context.Context, req SynthesizeRequest, handle func(Chunk) error) error
	ListVoices(ctx context.Context) ([]Voice, error)
}

type SynthesizeRequest struct {
	Text     string
	Voice    string
	Rate     string
	Volume   string
	Pitch    string
	Boundary string
}

type Chunk struct {
	Type     string
	Data     []byte
	Offset   int64
	Duration int64
	Text     string
}

type VoiceTag struct {
	ContentCategories  []string `json:"ContentCategories"`
	VoicePersonalities []string `json:"VoicePersonalities"`
}

type Voice struct {
	Name           string   `json:"Name"`
	ShortName      string   `json:"ShortName"`
	Gender         string   `json:"Gender"`
	Locale         string   `json:"Locale"`
	SuggestedCodec string   `json:"SuggestedCodec"`
	FriendlyName   string   `json:"FriendlyName"`
	Status         string   `json:"Status"`
	VoiceTag       VoiceTag `json:"VoiceTag"`
}
