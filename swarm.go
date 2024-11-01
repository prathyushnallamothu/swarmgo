package swarmgo

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	openai "github.com/sashabaranov/go-openai"
)

// OpenAIClient defines the methods used from the OpenAI client
type OpenAIClient interface {
	CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
}

// Swarm represents the main structure
type Swarm struct {
	client OpenAIClient
}

// NewSwarm initializes a new Swarm instance with an OpenAI client
func NewSwarm(apiKey string) *Swarm {
	client := openai.NewClient(apiKey)
	return &Swarm{
		client: client,
	}
}

// NewSwarmWithProxy initializes a new Swarm instance with the given API key and proxy URL.
func NewSwarmWithProxy(apiKey, proxyURL string) *Swarm {
	config := openai.DefaultConfig(apiKey)
	if proxyURL != "" {
		config.BaseURL = proxyURL
	}
	client := openai.NewClientWithConfig(config)
	return &Swarm{
		client: client,
	}
}

// getChatCompletion requests a chat completion from the OpenAI API
func (s *Swarm) getChatCompletion(
	ctx context.Context,
	agent *Agent,
	history []openai.ChatCompletionMessage,
	contextVariables map[string]interface{},
	modelOverride string,
	stream bool,
	debug bool,
) (openai.ChatCompletionResponse, error) {

	// Prepare the initial system message with agent instructions
	instructions := agent.Instructions
	if agent.InstructionsFunc != nil {
		instructions += agent.InstructionsFunc(contextVariables)
	}
	messages := append([]openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: instructions,
		},
	}, history...)

	// Build function tools definitions from agent's functions
	var tools []openai.Tool
	for _, af := range agent.Functions {
		def := FunctionToDefinition(af)
		tools = append(tools, openai.Tool{
			Type:     openai.ToolTypeFunction,
			Function: def,
		})
	}

	// Prepare the chat completion request
	model := agent.Model
	if modelOverride != "" {
		model = modelOverride
	}
	req := openai.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Tools:    tools,
	}

	if debug {
		log.Printf("Getting chat completion for: %+v\n", messages)
	}

	// Call the OpenAI API to get a chat completion
	resp, err := s.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return openai.ChatCompletionResponse{}, err
	}

	return resp, nil
}

// handleFunctionCall processes a function call from the chat completion
func (s *Swarm) handleFunctionCall(
	ctx context.Context,
	ToolCalls []openai.ToolCall,
	agent *Agent,
	contextVariables map[string]interface{},
	debug bool,
) (Response, error) {

	var functionResultMessage []openai.ChatCompletionMessage
	var wg sync.WaitGroup

	for _, call := range ToolCalls {
		functionCall := call.Function

		functionName := functionCall.Name
		argsJSON := functionCall.Arguments

		// Parse the function call arguments
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return Response{}, err
		}

		if debug {
			log.Printf("Processing function call: %s with arguments %v\n", functionName, args)
		}

		// Find the corresponding function in the agent's functions
		var functionFound *AgentFunction
		for _, af := range agent.Functions {
			if af.Name == functionName {
				functionFound = &af
				break
			}
		}

		// Handle case where function is not found
		if functionFound == nil {
			errorMessage := fmt.Sprintf("Error: Tool %s not found.", functionName)
			if debug {
				log.Println(errorMessage)
			}
			return Response{
				Messages: []openai.ChatCompletionMessage{
					{
						Role:    "tool",
						Name:    functionName,
						Content: errorMessage,
					},
				},
			}, nil
		}

		// Execute the function and update context variables using a goroutine for asynchronous execution
		wg.Add(1)
		go func(args map[string]interface{}, contextVariables map[string]interface{}, call openai.ToolCall) {
			defer wg.Done()
			result := functionFound.Function(args, contextVariables)
			for k, v := range result.ContextVariables {
				contextVariables[k] = v
			}
			// Create a message with the function result
			functionResultMessage = append(functionResultMessage, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Name:       functionName,
				Content:    result.Value,
				ToolCallID: call.ID,
			})
		}(args, contextVariables, call)
	}
	wg.Wait()
	// Return the partial response with the function result
	partialResponse := Response{
		Messages:         functionResultMessage,
		Agent:            agent,
		ContextVariables: contextVariables,
	}

	return partialResponse, nil
}

// Run executes the chat interaction loop with the agent
func (s *Swarm) Run(
	ctx context.Context,
	agent *Agent,
	messages []openai.ChatCompletionMessage,
	contextVariables map[string]interface{},
	modelOverride string,
	stream bool,
	debug bool,
	maxTurns int,
	executeTools bool,
) (Response, error) {
	activeAgent := agent
	history := make([]openai.ChatCompletionMessage, len(messages))
	copy(history, messages)
	if contextVariables == nil {
		contextVariables = make(map[string]interface{})
	}

	initLen := len(messages)
	turns := 0

	// Main loop for chat interaction
	for turns < maxTurns && activeAgent != nil {
		turns++

		// Get a chat completion from the API
		resp, err := s.getChatCompletion(
			ctx,
			activeAgent,
			history,
			contextVariables,
			modelOverride,
			stream,
			debug,
		)
		if err != nil {
			return Response{}, err
		}

		if len(resp.Choices) == 0 {
			return Response{}, fmt.Errorf("no choices in response")
		}

		choice := resp.Choices[0]
		message := choice.Message

		if debug {
			log.Printf("Received completion: %+v\n", message)
		}

		// Update message role and name
		message.Role = openai.ChatMessageRoleAssistant
		message.Name = activeAgent.Name

		history = append(history, message)

		// Handle function calls if any
		if message.ToolCalls != nil && executeTools {
			// Process the function call
			partialResponse, err := s.handleFunctionCall(
				ctx,
				message.ToolCalls,
				activeAgent,
				contextVariables,
				debug,
			)
			if err != nil {
				return Response{}, err
			}

			history = append(history, partialResponse.Messages...)
			for k, v := range partialResponse.ContextVariables {
				contextVariables[k] = v
			}
			if partialResponse.Agent != nil {
				activeAgent = partialResponse.Agent
			}

			// Get the assistant's response after function result
			resp, err := s.getChatCompletion(
				ctx,
				activeAgent,
				history,
				contextVariables,
				modelOverride,
				stream,
				debug,
			)
			if err != nil {
				return Response{}, err
			}

			if len(resp.Choices) == 0 {
				return Response{}, fmt.Errorf("no choices in response")
			}

			choice = resp.Choices[0]
			message = choice.Message

			if debug {
				log.Printf("Received completion: %+v\n", message)
			}

			message.Role = openai.ChatMessageRoleAssistant
			message.Name = activeAgent.Name

			history = append(history, message)

			// Break the outer loop if the assistant didn't make a function call
			if message.FunctionCall == nil || !executeTools {
				if debug {
					log.Println("Ending turn.")
				}
				break
			}
		}
	}

	// Return the final response
	return Response{
		Messages:         history[initLen:],
		Agent:            activeAgent,
		ContextVariables: contextVariables,
	}, nil
}
