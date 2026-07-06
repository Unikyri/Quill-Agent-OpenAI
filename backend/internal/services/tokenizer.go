package services

import (
	"github.com/pkoukk/tiktoken-go"
	tiktoken_loader "github.com/pkoukk/tiktoken-go-loader"
)

// init registers an offline BPE loader so tiktoken.GetEncoding doesn't need
// network access to fetch cl100k_base.tiktoken (the Docker demo container
// isn't guaranteed egress).
func init() {
	tiktoken.SetBpeLoader(tiktoken_loader.NewOfflineLoader())
}

// Tokenizer counts tokens using the cl100k_base BPE encoding, with a
// len(text)/4 fallback if the encoding fails to load.
type Tokenizer struct {
	enc *tiktoken.Tiktoken
}

// NewTokenizer loads the cl100k_base encoding. If loading fails, enc stays
// nil and CountTokens falls back to a length-based approximation.
func NewTokenizer() *Tokenizer {
	enc, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return &Tokenizer{enc: nil}
	}
	return &Tokenizer{enc: enc}
}

// CountTokens returns the token count for text, using the BPE encoding when
// available or a len(text)/4 + 1 fallback otherwise.
func (t *Tokenizer) CountTokens(text string) int {
	if text == "" {
		return 0
	}
	if t.enc == nil {
		return len(text)/4 + 1
	}
	return len(t.enc.Encode(text, nil, nil))
}

// CountTokensForMessages sums token counts across a chat message list,
// including a small per-message overhead and tool-call name/arguments
// tokens, matching the rough shape of OpenAI-style chat token accounting.
func (t *Tokenizer) CountTokensForMessages(messages []QwenMessage) int {
	if len(messages) == 0 {
		return 0
	}

	const perMessageOverhead = 4
	const priming = 2

	total := priming
	for _, msg := range messages {
		total += perMessageOverhead
		total += t.CountTokens(msg.Role)
		total += t.CountTokens(msg.Content)
		for _, tc := range msg.ToolCalls {
			total += t.CountTokens(tc.Function.Name)
			total += t.CountTokens(tc.Function.Arguments)
		}
	}
	return total
}
