package services

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newSSEServerFunc is like newSSEServer but computes the response lines per
// request, so a test can simulate the multi-round-trip shape of an agent
// loop (tool call round, then final-answer round) against a single server.
func newSSEServerFunc(t *testing.T, lines func() []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("ResponseWriter does not support flushing")
		}
		for _, line := range lines() {
			fmt.Fprintf(w, "data: %s\n\n", line)
			flusher.Flush()
		}
	}))
}

// recordingExecutor implements ToolExecutor and records every call it
// receives, returning a canned result.
type recordingExecutor struct {
	calls []string
}

func (r *recordingExecutor) ExecuteTool(name string, argsJSON string) (string, error) {
	r.calls = append(r.calls, name+":"+argsJSON)
	return "tool result for " + name, nil
}

// TestRunAgentLoopStream_NoToolCalls covers the plain-streamed-answer path:
// the model responds with text only, no tool calls, and onProgress never
// fires (no tool call ever completes).
func TestRunAgentLoopStream_NoToolCalls(t *testing.T) {
	server := newSSEServer(t, []string{
		`{"choices":[{"delta":{"content":"The answer "},"finish_reason":null}]}`,
		`{"choices":[{"delta":{"content":"is 42."},"finish_reason":null}]}`,
		`{"choices":[{"delta":{},"finish_reason":"stop"}]}`,
		`[DONE]`,
	})
	defer server.Close()

	svc := newStreamTestService(server.URL)
	exec := &recordingExecutor{}
	tools := []QwenTool{{Type: "function", Function: QwenToolFunction{Name: "search_vector_memory"}}}

	var progressCalls int
	answer, err := svc.RunAgentLoopStream(context.Background(), []QwenMessage{{Role: "user", Content: "what is the answer?"}}, tools, exec, 5, func(stage string, tc *QwenToolCall) {
		progressCalls++
	})
	if err != nil {
		t.Fatalf("RunAgentLoopStream: %v", err)
	}
	if answer != "The answer is 42." {
		t.Errorf("answer = %q, want %q", answer, "The answer is 42.")
	}
	if len(exec.calls) != 0 {
		t.Errorf("expected no tool calls executed, got %v", exec.calls)
	}
	if progressCalls != 0 {
		t.Errorf("expected onProgress to never fire when no tool calls occur, got %d calls", progressCalls)
	}
}

// TestRunAgentLoopStream_SingleToolCallRoundTrip covers a single tool-call
// round-trip: first stream response emits a tool call, the executor runs,
// the result is fed back as a role:"tool" message, and a second stream
// response (server round 2) yields the final answer.
func TestRunAgentLoopStream_SingleToolCallRoundTrip(t *testing.T) {
	round := 0
	server := newSSEServerFunc(t, func() []string {
		round++
		if round == 1 {
			return []string{
				`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"search_vector_memory","arguments":"{\"query\":\"dragon\"}"}}]},"finish_reason":null}]}`,
				`{"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
				`[DONE]`,
			}
		}
		return []string{
			`{"choices":[{"delta":{"content":"Final answer using tool result."},"finish_reason":null}]}`,
			`{"choices":[{"delta":{},"finish_reason":"stop"}]}`,
			`[DONE]`,
		}
	})
	defer server.Close()

	svc := newStreamTestService(server.URL)
	exec := &recordingExecutor{}
	tools := []QwenTool{{Type: "function", Function: QwenToolFunction{Name: "search_vector_memory"}}}

	var progressed []QwenToolCall
	answer, err := svc.RunAgentLoopStream(context.Background(), []QwenMessage{{Role: "user", Content: "search for dragons"}}, tools, exec, 5, func(stage string, tc *QwenToolCall) {
		if tc != nil {
			progressed = append(progressed, *tc)
		}
	})
	if err != nil {
		t.Fatalf("RunAgentLoopStream: %v", err)
	}
	if answer != "Final answer using tool result." {
		t.Errorf("answer = %q, want %q", answer, "Final answer using tool result.")
	}
	if len(exec.calls) != 1 || exec.calls[0] != `search_vector_memory:{"query":"dragon"}` {
		t.Errorf("unexpected executor calls: %v", exec.calls)
	}

	// onProgress must fire exactly once, with the completed tool call.
	if len(progressed) != 1 {
		t.Fatalf("expected onProgress to fire once, got %d calls: %+v", len(progressed), progressed)
	}
	if progressed[0].Function.Name != "search_vector_memory" || progressed[0].ID != "call_1" {
		t.Errorf("onProgress tool call = %+v, want name=search_vector_memory id=call_1", progressed[0])
	}
}
