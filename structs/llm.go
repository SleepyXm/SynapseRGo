package structs

type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIRequest struct {
	Model     string       `json:"model"`
	Messages  []LLMMessage `json:"messages"`
	Stream    bool         `json:"stream"`
	MaxTokens int          `json:"max_tokens,omitempty"`
}

type StreamDelta struct {
	Content string `json:"content"`
}

type StreamChoice struct {
	Delta StreamDelta `json:"delta"`
}

type StreamChunk struct {
	Choices []StreamChoice `json:"choices"`
}
