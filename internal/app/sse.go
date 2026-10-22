package app

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
)

const maxSSEEventSize = 2 << 20

type sseEvent struct {
	data string
	raw  []byte
}

func (e sseEvent) isDone() bool { return strings.TrimSpace(e.data) == "[DONE]" }
func (e sseEvent) hasData() bool { return e.data != "" }

func readSSEEvent(reader *bufio.Reader) (sseEvent, error) {
	var event sseEvent
	var data []string
	for {
		line, err := reader.ReadBytes('\n')
		if len(event.raw)+len(line) > maxSSEEventSize {
			return sseEvent{}, fmt.Errorf("upstream SSE event exceeds size limit")
		}
		event.raw = append(event.raw, line...)
		text := strings.TrimSuffix(string(line), "\n")
		text = strings.TrimSuffix(text, "\r")
		if text == "" {
			event.data = strings.Join(data, "\n")
			return event, nil
		}
		if strings.HasPrefix(text, "data:") {
			value := strings.TrimPrefix(text, "data:")
			if strings.HasPrefix(value, " ") {
				value = value[1:]
			}
			data = append(data, value)
		}
		if err != nil {
			if err == io.EOF && len(event.raw) > 0 {
				event.data = strings.Join(data, "\n")
				return event, nil
			}
			return sseEvent{}, err
		}
	}
}

func writeSSEEvent(w io.Writer, event sseEvent) error {
	if len(event.raw) > 0 {
		_, err := w.Write(event.raw)
		return err
	}
	_, err := bytes.NewReader(nil).WriteTo(w)
	return err
}
