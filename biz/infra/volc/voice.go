package volc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type VoiceInfo struct {
	VoiceType    string `json:"voiceType"`
	Name         string `json:"name"`
	Language     string `json:"language"`
	Gender       string `json:"gender"`
	Category     string `json:"category"`
	DemoAudioURL string `json:"demoAudioUrl"`
}

type ListVoicesResp struct {
	Voices   []*VoiceInfo `json:"voices"`
	RawDebug string       `json:"rawDebug,omitempty"`
}

func ListVoices(baseURL, accessKey string) (*ListVoicesResp, ListRawResponse, error) {
	if !strings.HasPrefix(baseURL, "http") {
		baseURL = "https://" + baseURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	endpoints := []string{
		baseURL + "/api/v1/bigmodel/list_timbres",
		baseURL + "/api/v3/tts/list_speakers",
		baseURL + "/api/v1/tts/list_speakers",
	}

	var lastErr error
	var lastRaw ListRawResponse

	for _, ep := range endpoints {
		req, err := http.NewRequest("GET", ep, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer;%s", accessKey))

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("call %s: %w", ep, err)
			continue
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode == 404 {
			lastErr = fmt.Errorf("endpoint not found: %s (status %d)", ep, resp.StatusCode)
			continue
		}

		lastRaw = ListRawResponse{
			Endpoint:   ep,
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}

		if resp.StatusCode != 200 {
			lastErr = fmt.Errorf("API error %d from %s: %s", resp.StatusCode, ep, string(body))
			continue
		}

		voices, err := parseSpeakerResponse(body)
		if err != nil {
			lastErr = fmt.Errorf("parse response from %s: %w (body: %s)", ep, err, string(body))
			continue
		}

		return &ListVoicesResp{
			Voices:   voices,
			RawDebug: string(body),
		}, lastRaw, nil
	}

	return nil, lastRaw, lastErr
}

type ListRawResponse struct {
	Endpoint   string `json:"endpoint"`
	StatusCode int    `json:"statusCode"`
	Body       string `json:"body"`
}

func parseSpeakerResponse(body []byte) ([]*VoiceInfo, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	data, ok := raw["data"]
	if !ok {
		data = raw
	}

	var speakers []any
	switch d := data.(type) {
	case map[string]any:
		for _, key := range []string{"speakers", "timbres", "speaker_list", "list"} {
			if v, ok := d[key]; ok {
				if arr, ok := v.([]any); ok {
					speakers = arr
					break
				}
			}
		}
		if speakers == nil {
			return nil, fmt.Errorf("cannot find speaker list in response, keys: %v", keysOf(d))
		}
	case []any:
		speakers = d
	default:
		return nil, fmt.Errorf("unexpected data type: %T", data)
	}

	var voices []*VoiceInfo
	for _, s := range speakers {
		sm, ok := s.(map[string]any)
		if !ok {
			continue
		}
		v := &VoiceInfo{
			VoiceType:    stringVal(sm, "speaker", "voice_type", "voiceType", "voice", "Speaker"),
			Name:         stringVal(sm, "speaker_name", "name", "speakerName", "display_name", "displayName"),
			Language:     stringVal(sm, "language", "lang"),
			Gender:       stringVal(sm, "gender"),
			DemoAudioURL: stringVal(sm, "audio_url", "demo_url", "demoUrl", "demo_audio_url", "demoAudioUrl", "audio", "audioUrl"),
		}
		if v.VoiceType == "" {
			continue
		}
		voices = append(voices, v)
	}

	if len(voices) == 0 {
		return nil, fmt.Errorf("no speakers found in response")
	}

	return voices, nil
}

func stringVal(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
