package swarmgo

import (
	openai "github.com/sashabaranov/go-openai"
)

// Response represents a response from an agent, including messages and context
type Response struct {
	Messages         []openai.ChatCompletionMessage // List of chat messages in the response
	History          []openai.ChatCompletionMessage // List of chat messages in the response
	Agent            *Agent                         // Reference to the agent that generated the response
	ContextVariables map[string]interface{}         // Additional context variables associated with the response
}

// Result represents the outcome of an operation, including its value and context
type Result struct {
	Value            string                 // The resulting value as a string
	Agent            *Agent                 // Reference to the agent that produced the result
	ContextVariables map[string]interface{} // Additional context variables associated with the result
}
