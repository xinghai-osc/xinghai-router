package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func (s *Service) writeMiniMaxVideoTask(w http.ResponseWriter, r *http.Request, resp *http.Response, id, upstreamID, model string, content bool) (int, error) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxVideoTaskBody+1))
	if err != nil || len(body) > maxVideoTaskBody {
		return 0, fmt.Errorf("could not read MiniMax task")
	}
	if r.Method == http.MethodDelete {
		var result struct {
			TaskID string `json:"task_id"`
			Action string `json:"action"`
			Status string `json:"status"`
		}
		if json.Unmarshal(body, &result) != nil || result.TaskID != upstreamID || result.Action != result.Status || result.Action != "cancelled" && result.Action != "deleted" {
			return 0, fmt.Errorf("invalid MiniMax cancellation response")
		}
		result.TaskID = id
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, resp.StatusCode, result)
		return resp.StatusCode, nil
	}
	converted, downloadURL, err := miniMaxVideoStatus(body, upstreamID)
	if err != nil {
		return 0, err
	}
	if content {
		if variant := r.URL.Query().Get("variant"); variant != "" && variant != "video" {
			writeError(w, 400, "invalid_request", "MiniMax supports only the video content variant")
			return 400, nil
		}
		var task struct {
			Task struct {
				Status string `json:"status"`
			} `json:"task"`
		}
		_ = json.Unmarshal(body, &task)
		if task.Task.Status != "succeeded" || downloadURL == "" {
			writeError(w, 409, "video_not_ready", "video is not ready for download")
			return 409, nil
		}
		return s.writeMiniMaxVideoContent(w, r, downloadURL)
	}
	if strings.HasPrefix(r.URL.Path, "/v2/") {
		var envelope map[string]json.RawMessage
		_ = json.Unmarshal(body, &envelope)
		var task map[string]json.RawMessage
		_ = json.Unmarshal(envelope["task"], &task)
		task["id"], _ = json.Marshal(id)
		task["model"], _ = json.Marshal(model)
		envelope["task"], _ = json.Marshal(task)
		body, err = json.Marshal(envelope)
	} else {
		body, err = videoTaskResponse(converted, id)
		body = rewriteJSONModel(body, model)
	}
	if err != nil {
		return 0, err
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(resp.StatusCode)
	_, err = w.Write(body)
	return resp.StatusCode, err
}

func (s *Service) writeMiniMaxVideoContent(w http.ResponseWriter, r *http.Request, downloadURL string) (int, error) {
	if validOutboundURL(downloadURL) != nil {
		return 0, fmt.Errorf("invalid MiniMax video download URL")
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return 0, fmt.Errorf("invalid video download request")
	}
	for _, name := range []string{"Range", "If-Range"} {
		if value := r.Header.Get(name); value != "" {
			req.Header.Set(name, value)
		}
	}
	client := clientWithTimeout(s.streamClient, 5*time.Minute)
	client.Jar = nil
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("video download failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
			w.Header().Set("Content-Range", resp.Header.Get("Content-Range"))
			writeError(w, 416, "invalid_range", "video range is not satisfiable")
			return 416, nil
		}
		return 0, fmt.Errorf("video download returned an unexpected status")
	}
	for _, name := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "Content-Disposition", "ETag", "Last-Modified"} {
		if value := resp.Header.Get(name); value != "" {
			w.Header().Set(name, value)
		}
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	return resp.StatusCode, err
}
