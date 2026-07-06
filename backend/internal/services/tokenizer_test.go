package services

import "testing"

func TestCountTokensEmpty(t *testing.T) {
	tok := NewTokenizer()
	if got := tok.CountTokens(""); got != 0 {
		t.Errorf("CountTokens(\"\") = %d, want 0", got)
	}
}

func TestCountTokensNonEmpty(t *testing.T) {
	tok := NewTokenizer()
	short := tok.CountTokens("hello world")
	if short <= 0 {
		t.Errorf("CountTokens(short) = %d, want > 0", short)
	}

	long := tok.CountTokens(`
		In the ancient manuscript, the ink had faded across centuries,
		yet the words still carried the weight of forgotten kingdoms
		and the sorrow of characters long since written out of the story.
		Every paragraph held a secret, every chapter a contradiction
		waiting to be found by careful readers of the ledger.
	`)
	if long <= short {
		t.Errorf("CountTokens(long) = %d, want > CountTokens(short) = %d", long, short)
	}
}

func TestCountTokensForMessagesSumsRoles(t *testing.T) {
	tok := NewTokenizer()

	single := []QwenMessage{
		{Role: "user", Content: "What happened to Bob in chapter 3?"},
	}
	singleTotal := tok.CountTokensForMessages(single)
	if singleTotal <= 0 {
		t.Fatalf("CountTokensForMessages(single) = %d, want > 0", singleTotal)
	}

	multi := []QwenMessage{
		{Role: "system", Content: "You are a contradiction detector."},
		{Role: "user", Content: "What happened to Bob in chapter 3?"},
		{
			Role: "assistant",
			ToolCalls: []QwenToolCall{
				{
					ID:   "call_1",
					Type: "function",
					Function: QwenToolCallFunction{
						Name:      "search_vector_memory",
						Arguments: `{"query":"Bob chapter 3"}`,
					},
				},
			},
		},
		{Role: "tool", Content: "Bob died in chapter 2.", ToolCallID: "call_1"},
	}
	multiTotal := tok.CountTokensForMessages(multi)

	contentOnly := 0
	for _, m := range multi {
		contentOnly += tok.CountTokens(m.Content)
	}

	if multiTotal <= contentOnly {
		t.Errorf("CountTokensForMessages(multi) = %d, want > content-only sum %d (overhead not counted)", multiTotal, contentOnly)
	}
	if multiTotal <= singleTotal {
		t.Errorf("CountTokensForMessages(multi) = %d, want > CountTokensForMessages(single) = %d", multiTotal, singleTotal)
	}
}

func TestCountTokensForMessagesEmpty(t *testing.T) {
	tok := NewTokenizer()
	if got := tok.CountTokensForMessages(nil); got != 0 {
		t.Errorf("CountTokensForMessages(nil) = %d, want 0", got)
	}
}
