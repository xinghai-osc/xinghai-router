package app

import (
	"bytes"
	"encoding/json"
	"mime"
	"net/http"
	"strings"
	"unicode/utf8"
)

func (o mediaGatewayOptions) transcriptContentType() string {
	switch o.responseFormat {
	case "text":
		return "text/plain"
	case "srt":
		return "application/x-subrip"
	case "vtt":
		return "text/vtt"
	default:
		return "application/json"
	}
}

func (o mediaGatewayOptions) validTextTranscription(body []byte, contentType string) bool {
	if !utf8.Valid(body) || bytes.IndexByte(body, 0) >= 0 {
		return false
	}
	if contentType != "" {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil {
			return false
		}
		switch mediaType {
		case "text/plain", "application/octet-stream":
		case "application/x-subrip", "application/srt", "application/x-srt", "text/srt", "text/x-srt":
			if o.responseFormat != "srt" {
				return false
			}
		case "text/vtt":
			if o.responseFormat != "vtt" {
				return false
			}
		default:
			return false
		}
	}
	content := bytes.TrimSpace(bytes.TrimPrefix(bytes.TrimSpace(body), []byte("\ufeff")))
	if len(content) == 0 {
		return o.responseFormat == "text" || o.responseFormat == "srt"
	}
	if strings.HasPrefix(http.DetectContentType(content), "text/html") {
		return false
	}
	if content[0] == '{' {
		var obj struct {
			Error  json.RawMessage `json:"error"`
			Detail json.RawMessage `json:"detail"`
		}
		if json.Unmarshal(content, &obj) == nil && (len(obj.Error) > 0 || len(obj.Detail) > 0) {
			return false
		}
	}
	return true
}
