package services

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// StreamChunk is one incremental event emitted while consuming a Qwen
// streaming chat completion. Type is one of "text", "tool_call", "done", or
// "error"; only the field(s) matching Type are populated.
type StreamChunk struct {
	Type     string
	Text     string
	ToolCall *QwenToolCall
	Finish   string
	Err      error
}

// accToolCall buffers a tool call's fragmented deltas (name arrives once,
// arguments arrive as concatenated string fragments) until the stream signals
// completion, keyed by the delta's index.
type accToolCall struct {
	ID   string
	Name string
	Args strings.Builder
}

// streamResponseFrame mirrors one SSE "data:" frame of a Qwen streaming chat
// completion response.
type streamResponseFrame struct {
	Choices []struct {
		Delta struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// ChatCompletionStream sends payload with stream:true and returns a channel
// of StreamChunk values read from the SSE response body. The channel is
// closed when the stream ends (successfully or on error).
//
// Verified against a live DashScope streaming + tool-call response (PR3
// spike): the final chunk before [DONE] carries finish_reason:"tool_calls",
// tool-call deltas are keyed by index, function.name arrives once, and
// function.arguments arrives as concatenated string fragments.
func (s *QwenService) ChatCompletionStream(ctx context.Context, payload QwenRequest) (<-chan StreamChunk, error) {
	payload = s.normalizeRequestMessages(payload)
	payload.Stream = true
	resp, release, err := s.sendQwenRequest(ctx, s.tierForModel(payload.Model), payload.Model, http.MethodPost, "/chat/completions", payload, s.estimateChatTokens(payload), true)
	if err != nil {
		return nil, err
	}

	ch := make(chan StreamChunk)
	go func() { release(readStream(resp.Body, ch)) }()
	return ch, nil
}

// readStream parses SSE frames off body, accumulates tool-call deltas by
// index, and emits StreamChunk values on ch. It closes ch and body when done.
func readStream(body io.ReadCloser, ch chan<- StreamChunk) (success bool) {
	defer close(ch)
	defer body.Close()

	acc := map[int]*accToolCall{}
	finished := false

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var frame streamResponseFrame
		if err := json.Unmarshal([]byte(data), &frame); err != nil {
			ch <- StreamChunk{Type: "error", Err: fmt.Errorf("unmarshal stream chunk: %w", err)}
			return false
		}
		if len(frame.Choices) == 0 {
			continue
		}
		choice := frame.Choices[0]

		if choice.Delta.Content != "" {
			ch <- StreamChunk{Type: "text", Text: choice.Delta.Content}
		}

		for _, tc := range choice.Delta.ToolCalls {
			entry, ok := acc[tc.Index]
			if !ok {
				entry = &accToolCall{}
				acc[tc.Index] = entry
			}
			if tc.ID != "" {
				entry.ID = tc.ID
			}
			if tc.Function.Name != "" {
				entry.Name = tc.Function.Name
			}
			entry.Args.WriteString(tc.Function.Arguments)
		}

		switch choice.FinishReason {
		case "tool_calls":
			finished = true
			flushToolCalls(acc, ch)
		case "stop":
			finished = true
			ch <- StreamChunk{Type: "done", Finish: "stop"}
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- StreamChunk{Type: "error", Err: fmt.Errorf("read stream: %w", err)}
		return false
	}

	if !finished {
		ch <- StreamChunk{Type: "error", Err: fmt.Errorf("stream ended without a finish_reason completion signal")}
		return false
	}
	return true
}

// flushToolCalls emits one tool_call StreamChunk per accumulated entry, in
// index order, so multi-tool-call responses dispatch deterministically.
func flushToolCalls(acc map[int]*accToolCall, ch chan<- StreamChunk) {
	indices := make([]int, 0, len(acc))
	for idx := range acc {
		indices = append(indices, idx)
	}
	sort.Ints(indices)

	for _, idx := range indices {
		entry := acc[idx]
		ch <- StreamChunk{
			Type: "tool_call",
			ToolCall: &QwenToolCall{
				ID:   entry.ID,
				Type: "function",
				Function: QwenToolCallFunction{
					Name:      entry.Name,
					Arguments: entry.Args.String(),
				},
			},
		}
	}
}
