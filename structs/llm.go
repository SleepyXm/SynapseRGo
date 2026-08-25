package structs

type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIRequest struct {
	Model            string       `json:"model"`
	Messages         []LLMMessage `json:"messages"`
	Stream           bool         `json:"stream"`
	MaxTokens        *int         `json:"max_tokens,omitempty"`
	Temperature      *float64     `json:"temperature,omitempty"`
	TopP             *float64     `json:"top_p,omitempty"`
	PresencePenalty  *float64     `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64     `json:"frequency_penalty,omitempty"`
}
