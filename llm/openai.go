package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/sashabaranov/go-openai"
)

// OpenAILLM implements the LLM interface for OpenAI
type OpenAILLM struct {
	client *openai.Client
}

// NewOpenAILLM creates a new OpenAI LLM client
func NewOpenAILLM(apiKey string) *OpenAILLM {
	client := openai.NewClient(apiKey)
	return &OpenAILLM{client: client}
}

func NewOpenAILLMWithHost(apiKey string, host string) *OpenAILLM {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = host
	openAIClient := openai.NewClientWithConfig(config)
	return &OpenAILLM{client: openAIClient}
}

// convertToOpenAIMessages converts our generic Message type to OpenAI's message type
func convertToOpenAIMessages(messages []Message) []openai.ChatCompletionMessage {
	openAIMessages := []openai.ChatCompletionMessage{}
	for _, msg := range messages {
		if msg.Content == "" {
			continue
		}
		openAIMessages = append(openAIMessages, openai.ChatCompletionMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
			Name:    msg.Name,
		})
	}
	return openAIMessages
}

// convertFromOpenAIMessage converts OpenAI's message type to our generic Message type
func convertFromOpenAIMessage(msg openai.ChatCompletionMessage) Message {
	return Message{
		Role:    Role(msg.Role),
		Content: msg.Content,
		Name:    msg.Name,
	}
}

// convertFromOpenAIDelta converts OpenAI's delta message type to our generic Message type
func convertFromOpenAIDelta(delta openai.ChatCompletionStreamChoiceDelta) Message {
	return Message{
		Role:    Role(delta.Role),
		Content: delta.Content,
	}
}

// convertToOpenAITools converts our generic Tool type to OpenAI's tool type
func convertToOpenAITools(tools []Tool) []openai.Tool {
	if len(tools) == 0 {
		return nil
	}

	openAITools := make([]openai.Tool, len(tools))
	for i, tool := range tools {
		def := openai.FunctionDefinition{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			Parameters:  tool.Function.Parameters,
		}
		openAITools[i] = openai.Tool{
			Type:     openai.ToolTypeFunction,
			Function: &def,
		}
	}
	return openAITools
}

// convertFromOpenAIToolCalls converts OpenAI's tool calls to our generic type
func convertFromOpenAIToolCalls(toolCalls []openai.ToolCall) []ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}

	calls := make([]ToolCall, len(toolCalls))
	for i, call := range toolCalls {
		calls[i] = ToolCall{
			ID:   call.ID,
			Type: string(call.Type),
		}
		calls[i].Function.Name = call.Function.Name
		calls[i].Function.Arguments = call.Function.Arguments
	}
	return calls
}

// CreateChatCompletion implements the LLM interface for OpenAI
func (o *OpenAILLM) CreateChatCompletion(ctx context.Context, req ChatCompletionRequest) (ChatCompletionResponse, error) {
	openAIReq := openai.ChatCompletionRequest{
		Model:           req.Model,
		Messages:        convertToOpenAIMessages(req.Messages),
		Temperature:     float32(req.Temperature),
		TopP:            float32(req.TopP),
		N:               req.N,
		Stop:            req.Stop,
		MaxTokens:       req.MaxTokens,
		PresencePenalty: req.PresencePenalty,
		Tools:           convertToOpenAITools(req.Tools),
	}

	resp, err := o.client.CreateChatCompletion(ctx, openAIReq)
	if err != nil {
		return ChatCompletionResponse{}, err
	}

	choices := make([]Choice, len(resp.Choices))
	for i, c := range resp.Choices {
		msg := convertFromOpenAIMessage(c.Message)
		msg.ToolCalls = convertFromOpenAIToolCalls(c.Message.ToolCalls)
		choices[i] = Choice{
			Index:        c.Index,
			Message:      msg,
			FinishReason: string(c.FinishReason),
		}
	}

	return ChatCompletionResponse{
		ID:      resp.ID,
		Choices: choices,
		Usage: Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}, nil
}

// openAIStreamWrapper wraps the OpenAI stream
type openAIStreamWrapper struct {
	stream          *openai.ChatCompletionStream
	currentToolCall *ToolCall
	toolCallBuffer  map[string]*ToolCall
}

func newOpenAIStreamWrapper(stream *openai.ChatCompletionStream) *openAIStreamWrapper {
	return &openAIStreamWrapper{
		stream:         stream,
		toolCallBuffer: make(map[string]*ToolCall),
	}
}

func (w *openAIStreamWrapper) Recv() (ChatCompletionResponse, error) {
	resp, err := w.stream.Recv()
	if err != nil {
		if err == io.EOF {
			return ChatCompletionResponse{}, err
		}
		var openAIErr *openai.APIError
		if errors.As(err, &openAIErr) {
			return ChatCompletionResponse{}, fmt.Errorf("OpenAI API error: %s - %s", openAIErr.Code, openAIErr.Message)
		}
		return ChatCompletionResponse{}, fmt.Errorf("stream receive failed: %w", err)
	}

	choices := make([]Choice, len(resp.Choices))
	for i, c := range resp.Choices {
		message := Message{
			Role:    Role(c.Delta.Role),
			Content: c.Delta.Content,
		}

		// Handle tool calls in delta
		if len(c.Delta.ToolCalls) > 0 {
			message.ToolCalls = make([]ToolCall, 0)
			for _, tc := range c.Delta.ToolCalls {
				// Get or create tool call buffer
				toolCall, exists := w.toolCallBuffer[tc.ID]
				if !exists {
					if tc.ID == "" {
						// Skip empty IDs but accumulate arguments if present
						if tc.Function.Arguments != "" && w.currentToolCall != nil {
							w.currentToolCall.Function.Arguments += tc.Function.Arguments

							// Try to parse the arguments to verify if it's complete JSON
							var args map[string]interface{}
							if err := json.Unmarshal([]byte(w.currentToolCall.Function.Arguments), &args); err == nil {
								// Add to message's tool calls and remove from buffer
								message.ToolCalls = append(message.ToolCalls, *w.currentToolCall)
								delete(w.toolCallBuffer, w.currentToolCall.ID)
								w.currentToolCall = nil
							}
						}
						continue
					}
					toolCall = &ToolCall{
						ID:   tc.ID,
						Type: string(tc.Type),
						Function: ToolCallFunction{
							Name:      tc.Function.Name,
							Arguments: "",
						},
					}
					w.toolCallBuffer[tc.ID] = toolCall
					w.currentToolCall = toolCall
				}

				// Update function name if provided
				if tc.Function.Name != "" {
					toolCall.Function.Name = tc.Function.Name
				}

				// Update arguments if provided
				if tc.Function.Arguments != "" {
					// Always append new arguments
					toolCall.Function.Arguments += tc.Function.Arguments

					// Try to parse the arguments to verify if it's complete JSON
					var args map[string]interface{}
					if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err == nil {
						// Add to message's tool calls and remove from buffer
						message.ToolCalls = append(message.ToolCalls, *toolCall)
						delete(w.toolCallBuffer, tc.ID)
						w.currentToolCall = nil
					}
				}
			}
		}

		choices[i] = Choice{
			Index:        c.Index,
			Message:      message,
			FinishReason: string(c.FinishReason),
		}
	}

	return ChatCompletionResponse{
		ID:      resp.ID,
		Choices: choices,
	}, nil
}

func (w *openAIStreamWrapper) Close() error {
	return w.stream.Close()
}

// CreateChatCompletionStream implements the LLM interface for OpenAI streaming
func (o *OpenAILLM) CreateChatCompletionStream(ctx context.Context, req ChatCompletionRequest) (ChatCompletionStream, error) {
	openAIReq := openai.ChatCompletionRequest{
		Model:           req.Model,
		Messages:        convertToOpenAIMessages(req.Messages),
		Temperature:     float32(req.Temperature),
		TopP:            float32(req.TopP),
		N:               req.N,
		Stop:            req.Stop,
		MaxTokens:       req.MaxTokens,
		PresencePenalty: float32(req.PresencePenalty),
		Tools:           convertToOpenAITools(req.Tools),
		Stream:          true,
	}

	stream, err := o.client.CreateChatCompletionStream(ctx, openAIReq)
	if err != nil {
		var openAIErr *openai.APIError
		if errors.As(err, &openAIErr) {
			return nil, fmt.Errorf("OpenAI API error: %s - %s", openAIErr.Code, openAIErr.Message)
		}
		return nil, fmt.Errorf("stream creation failed: %w", err)
	}

	return newOpenAIStreamWrapper(stream), nil
}
