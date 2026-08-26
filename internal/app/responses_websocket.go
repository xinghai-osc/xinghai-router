package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const (
	responsesWSFirstFrameTimeout = 30 * time.Second
)

func isWebSocketUpgrade(r *http.Request) bool {
	return r != nil && r.Method == http.MethodGet && strings.EqualFold(r.Header.Get("Upgrade"), "websocket") && strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

func (s *Service) responsesWebSocket(w http.ResponseWriter, r *http.Request) {
	if !isWebSocketUpgrade(r) {
		writeError(w, http.StatusUpgradeRequired, "invalid_request_error", "WebSocket upgrade required (Upgrade: websocket)")
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionContextTakeover})
	if err != nil {
		return
	}
	defer conn.CloseNow()
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	first := true
	model := ""
	for {
		readCtx := ctx
		var stop context.CancelFunc
		if first {
			readCtx, stop = context.WithTimeout(ctx, responsesWSFirstFrameTimeout)
		}
		typ, payload, readErr := conn.Read(readCtx)
		if stop != nil {
			stop()
		}
		if readErr != nil {
			return
		}
		if typ != websocket.MessageText && typ != websocket.MessageBinary {
			_ = conn.Close(websocket.StatusPolicyViolation, "only text or binary JSON messages are supported")
			return
		}
		normalized, requestModel, err := normalizeResponsesWSRequest(payload, model, first)
		if err != nil {
			_ = conn.Close(websocket.StatusPolicyViolation, err.Error())
			return
		}
		model = requestModel
		first = false
		converted, _, err := responsesRequestToChatCompletions(normalized)
		if err != nil {
			_ = conn.Close(websocket.StatusPolicyViolation, err.Error())
			return
		}
		// The WS bridge always consumes an upstream stream and emits Responses
		// event JSON frames, matching Sub2Api's HTTP-bridge behavior.
		adapter := &wsResponsesAdapter{conn: conn, ctx: ctx, model: model}
		request := r.Clone(ctx)
		request.Body = io.NopCloser(bytes.NewReader(converted))
		s.proxyChatCompletions(adapter, request, converted, model, true, 0, nil, adapter.stream, nil, nil, nil, nil)
	}
}

func normalizeResponsesWSRequest(body []byte, sessionModel string, first bool) ([]byte, string, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil || payload == nil {
		return nil, "", fmt.Errorf("invalid websocket request payload")
	}
	typeValue, _ := payload["type"].(string)
	if strings.TrimSpace(typeValue) == "" {
		payload["type"] = "response.create"
		typeValue = "response.create"
	}
	if typeValue != "response.create" {
		if typeValue == "response.append" {
			return nil, "", fmt.Errorf("response.append is not supported; use response.create with previous_response_id")
		}
		return nil, "", fmt.Errorf("unsupported websocket request type: %s", typeValue)
	}
	requestedModel, _ := payload["model"].(string)
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" {
		requestedModel = sessionModel
	}
	if first && !validModelName(requestedModel) {
		return nil, "", fmt.Errorf("model is required in first response.create payload")
	}
	if !validModelName(requestedModel) {
		return nil, "", fmt.Errorf("model is required in response.create payload")
	}
	payload["model"] = requestedModel
	if previous, _ := payload["previous_response_id"].(string); strings.TrimSpace(previous) != "" && !strings.HasPrefix(strings.TrimSpace(previous), "resp_") {
		return nil, "", fmt.Errorf("previous_response_id must be a response.id (resp_*), not a message id")
	}
	if isResponsesLiteBody(body) {
		normalized, _, err := normalizeResponsesLite(body)
		if err != nil {
			return nil, "", err
		}
		body = normalized
		if json.Unmarshal(body, &payload) != nil {
			return nil, "", fmt.Errorf("invalid normalized websocket payload")
		}
		payload["model"] = requestedModel
	}
	delete(payload, "type")
	delete(payload, "generate")
	delete(payload, "previous_response_id")
	payload["stream"] = true
	out, err := json.Marshal(payload)
	return out, requestedModel, err
}

type wsResponsesAdapter struct {
	conn  *websocket.Conn
	ctx   context.Context
	mu    sync.Mutex
	buf   bytes.Buffer
	model string
}

func (a *wsResponsesAdapter) stream(w http.ResponseWriter, resp *http.Response) (streamStats, error) {
	return streamChatCompletionsToResponses(w, resp, "resp_"+randomIDString(), responsesEcho{model: a.model})
}

func (a *wsResponsesAdapter) Header() http.Header { return http.Header{} }
func (a *wsResponsesAdapter) WriteHeader(int)     {}
func (a *wsResponsesAdapter) Flush()              {}
func (a *wsResponsesAdapter) Write(p []byte) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, _ = a.buf.Write(p)
	for {
		raw := a.buf.Bytes()
		i := bytes.Index(raw, []byte("\n\n"))
		if i < 0 {
			break
		}
		event := append([]byte(nil), raw[:i]...)
		a.buf.Next(i + 2)
		var data []string
		for scanner := bufio.NewScanner(bytes.NewReader(event)); scanner.Scan(); {
			line := strings.TrimSuffix(scanner.Text(), "\r")
			if strings.HasPrefix(line, "data:") {
				data = append(data, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			}
		}
		if len(data) == 0 {
			continue
		}
		joined := strings.Join(data, "\n")
		if joined == "[DONE]" {
			continue
		}
		if !json.Valid([]byte(joined)) {
			continue
		}
		if err := a.conn.Write(a.ctx, websocket.MessageText, []byte(joined)); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}
