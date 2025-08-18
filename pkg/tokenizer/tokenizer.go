package tokenizer

import (
	"fmt"
	"strings"

	"github.com/pkoukk/tiktoken-go"

	"github.com/username/hexarag/internal/domain/entities"
)

// Tokenizer provides token counting functionality
type Tokenizer struct {
	encoding     *tiktoken.Tiktoken
	encodingName string
}

// NewTokenizer creates a new tokenizer for the given model
func NewTokenizer(model string) (*Tokenizer, error) {
	var encodingName string

	// Map model names to appropriate encodings
	switch {
	case strings.Contains(model, "gpt-4"):
		encodingName = "cl100k_base"
	case strings.Contains(model, "gpt-3.5"):
		encodingName = "cl100k_base"
	case strings.Contains(model, "gpt-3"):
		encodingName = "p50k_base"
	case strings.Contains(model, "claude"):
		encodingName = "cl100k_base" // Use GPT-4 encoding as approximation
	default:
		// For local models, use cl100k_base as a reasonable default
		encodingName = "cl100k_base"
	}

	encoding, err := tiktoken.GetEncoding(encodingName)
	if err != nil {
		return nil, fmt.Errorf("failed to get encoding %s: %w", encodingName, err)
	}

	return &Tokenizer{
		encoding:     encoding,
		encodingName: encodingName,
	}, nil
}

// CountTokens counts tokens in a text string
func (t *Tokenizer) CountTokens(text string) int {
	if text == "" {
		return 0
	}

	tokens := t.encoding.Encode(text, nil, nil)
	return len(tokens)
}

// CountMessageTokens counts tokens in a message, including role and formatting overhead
func (t *Tokenizer) CountMessageTokens(message *entities.Message) int {
	if message == nil {
		return 0
	}

	// Base tokens for the message content
	contentTokens := t.CountTokens(message.Content)

	// Add overhead for role and formatting
	// OpenAI format adds approximately 3-4 tokens per message for formatting
	roleTokens := t.CountTokens(string(message.Role))
	formatOverhead := 4 // Approximate overhead for JSON formatting

	total := contentTokens + roleTokens + formatOverhead

	// Add tokens for tool calls if present
	for _, toolCall := range message.ToolCalls {
		total += t.CountToolCallTokens(&toolCall)
	}

	return total
}

// CountToolCallTokens counts tokens used by a tool call
func (t *Tokenizer) CountToolCallTokens(toolCall *entities.ToolCall) int {
	if toolCall == nil {
		return 0
	}

	total := 0

	// Tool name
	total += t.CountTokens(toolCall.Name)

	// Tool arguments (JSON encoded)
	if argsJSON, err := toolCall.ArgumentsJSON(); err == nil {
		total += t.CountTokens(argsJSON)
	}

	// Tool result if available
	if toolCall.Result != nil {
		if resultJSON, err := toolCall.ResultJSON(); err == nil {
			total += t.CountTokens(resultJSON)
		}
	}

	// Overhead for tool call formatting
	total += 10 // Approximate overhead

	return total
}

// CountConversationTokens counts total tokens in a conversation
func (t *Tokenizer) CountConversationTokens(messages []*entities.Message, systemPrompt string) int {
	total := 0

	// System prompt tokens
	if systemPrompt != "" {
		total += t.CountTokens(systemPrompt)
		total += 4 // formatting overhead
	}

	// Message tokens
	for _, message := range messages {
		total += t.CountMessageTokens(message)
	}

	// Add a small buffer for conversation-level formatting
	total += 2

	return total
}

// EstimateResponseTokens estimates how many tokens a response might use
// This is useful for checking if we're approaching token limits
func (t *Tokenizer) EstimateResponseTokens(maxTokens int, usedTokens int) int {
	remaining := maxTokens - usedTokens
	if remaining <= 0 {
		return 0
	}

	// Reserve some tokens for response formatting overhead
	overhead := 10
	if remaining > overhead {
		return remaining - overhead
	}

	return 1 // Always allow at least 1 token for response
}

// TruncateToTokenLimit truncates text to fit within a token limit
func (t *Tokenizer) TruncateToTokenLimit(text string, maxTokens int) string {
	if maxTokens <= 0 {
		return ""
	}

	tokens := t.encoding.Encode(text, nil, nil)
	if len(tokens) <= maxTokens {
		return text
	}

	// Truncate tokens and decode back to text
	truncatedTokens := tokens[:maxTokens]
	truncatedText := t.encoding.Decode(truncatedTokens)

	return truncatedText
}

// SplitTextByTokens splits text into chunks that fit within token limits
func (t *Tokenizer) SplitTextByTokens(text string, maxTokensPerChunk int) []string {
	if maxTokensPerChunk <= 0 {
		return []string{}
	}

	tokens := t.encoding.Encode(text, nil, nil)
	if len(tokens) <= maxTokensPerChunk {
		return []string{text}
	}

	var chunks []string
	for i := 0; i < len(tokens); i += maxTokensPerChunk {
		end := i + maxTokensPerChunk
		if end > len(tokens) {
			end = len(tokens)
		}

		chunkTokens := tokens[i:end]
		chunkText := t.encoding.Decode(chunkTokens)
		chunks = append(chunks, chunkText)
	}

	return chunks
}

// GetTokenDetails returns detailed information about tokenization
func (t *Tokenizer) GetTokenDetails(text string) map[string]interface{} {
	tokens := t.encoding.Encode(text, nil, nil)

	return map[string]interface{}{
		"text":            text,
		"token_count":     len(tokens),
		"character_count": len(text),
		"tokens_per_char": float64(len(tokens)) / float64(len(text)),
		"encoding":        t.encodingName,
	}
}
