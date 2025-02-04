package swarmgo

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/prathyushnallamothu/swarmgo/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockLLM is a mock implementation of the LLM interface
type MockLLM struct {
	mock.Mock
}

func (m *MockLLM) CreateChatCompletion(ctx context.Context, req llm.ChatCompletionRequest) (llm.ChatCompletionResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(llm.ChatCompletionResponse), args.Error(1)
}

func (m *MockLLM) CreateChatCompletionStream(ctx context.Context, req llm.ChatCompletionRequest) (llm.ChatCompletionStream, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(llm.ChatCompletionStream), args.Error(1)
}

// NewMockSwarm initializes a new Swarm instance with a mock LLM client
func NewMockSwarm(mockClient *MockLLM) *Swarm {
	return &Swarm{
		client: mockClient,
	}
}

// TestNewSwarm tests the NewSwarm function
func TestNewSwarm(t *testing.T) {
	apiKey := "test-api-key"
	sw := NewSwarm(apiKey, llm.OpenAI)
	assert.NotNil(t, sw)
	assert.NotNil(t, sw.client)
}

// TestNewSwarmWithHost tests the NewSwarmWithHost function
func TestNewSwarmWithHost(t *testing.T) {
	apiKey := "test-api-key"
	host := "https://api.xxxxx.com"
	sw := NewSwarmWithHost(apiKey, host, llm.OpenAI)
	assert.NotNil(t, sw)
	assert.NotNil(t, sw.client)
}

// TestFunctionToDefinition tests the FunctionToDefinition function
func TestFunctionToDefinition(t *testing.T) {
	af := AgentFunction{
		Name:        "testFunction",
		Description: "A test function",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"arg1": map[string]interface{}{
					"type":        "string",
					"description": "Argument 1",
				},
			},
			"required": []interface{}{"arg1"},
		},
	}

	def := FunctionToDefinition(af)

	assert.Equal(t, af.Name, def.Name)
	assert.Equal(t, af.Description, def.Description)
	assert.Equal(t, af.Parameters, def.Parameters)
}

// TestHandleToolCall tests the handleToolCall method
func TestHandleToolCall(t *testing.T) {
	sw := NewSwarm("test-api-key", llm.OpenAI)
	ctx := context.Background()

	toolCall := llm.ToolCall{
		ID:   "testFunction",
		Type: "function",
		Function: struct {
			Name      string "json:\"name\""
			Arguments string "json:\"arguments\""
		}{
			Name:      "testFunction",
			Arguments: `{"arg1": "value1"}`,
		},
	}

	agentFunction := AgentFunction{
		Name:        "testFunction",
		Description: "A test function",
		Function: func(args map[string]interface{}, contextVariables map[string]interface{}) Result {
			return Result{
				Success: true,
				Data:    "Function executed successfully",
			}
		},
	}

	agent := &Agent{
		Name:      "TestAgent",
		Functions: []AgentFunction{agentFunction},
	}

	contextVariables := map[string]interface{}{}

	response, err := sw.handleToolCall(ctx, &toolCall, agent, contextVariables, false)

	assert.NoError(t, err)
	assert.Len(t, response.Messages, 1)
	assert.Equal(t, llm.RoleAssistant, response.Messages[0].Role)
	assert.Equal(t, "Function executed successfully", response.Messages[0].Content)
}

// TestHandleToolCallFunctionNotFound tests handleToolCall when function is not found
func TestHandleToolCallFunctionNotFound(t *testing.T) {
	sw := NewSwarm("test-api-key", llm.OpenAI)
	ctx := context.Background()

	toolCall := llm.ToolCall{
		ID:   "nonExistentFunction",
		Type: "function",
		Function: struct {
			Name      string "json:\"name\""
			Arguments string "json:\"arguments\""
		}{
			Name:      "nonExistentFunction",
			Arguments: `{}`,
		},
	}

	agent := &Agent{
		Name:      "TestAgent",
		Functions: []AgentFunction{},
	}

	contextVariables := map[string]interface{}{}

	response, err := sw.handleToolCall(ctx, &toolCall, agent, contextVariables, false)

	assert.NoError(t, err)
	assert.Len(t, response.Messages, 1)
	assert.Equal(t, llm.RoleAssistant, response.Messages[0].Role)
	assert.Contains(t, response.Messages[0].Content, "Error: Tool nonExistentFunction not found.")
}

// TestMemoryStore tests the MemoryStore methods
func TestMemoryStore(t *testing.T) {
	ms := NewMemoryStore(10)
	ms.AddMemory(Memory{Content: "Hello, world!", Type: "conversation", Timestamp: time.Now(), Importance: 0.5})
	assert.Len(t, ms.shortTerm, 1)
	assert.Len(t, ms.longTerm["conversation"], 1)

	ms.RemoveMemory(ms.shortTerm[0].Content, ms.shortTerm[0].Type)
	assert.Len(t, ms.shortTerm, 0)
	assert.Len(t, ms.longTerm["conversation"], 0)

	ms.AddMemory(Memory{Content: "Hello, world!", Type: "conversation", Timestamp: time.Now(), Importance: 0.5})
	assert.Len(t, ms.shortTerm, 1)
	assert.Len(t, ms.longTerm["conversation"], 1)

	ms.RemoveMemory("Hello, world!", "conversation")
	assert.Len(t, ms.shortTerm, 0)
	assert.Len(t, ms.longTerm["conversation"], 0)
}

// TestRun tests the Run method
func TestRun(t *testing.T) {
	mockClient := new(MockLLM)
	sw := NewMockSwarm(mockClient)
	ctx := context.Background()

	agentFunction := AgentFunction{
		Name:        "testFunction",
		Description: "A test function",
		Function: func(args map[string]interface{}, contextVariables map[string]interface{}) Result {
			return Result{
				Success: true,
				Data:    "Function executed successfully",
			}
		},
	}

	agent := &Agent{
		Name:      "TestAgent",
		Functions: []AgentFunction{agentFunction},
		Model:     "test-model",
	}

	messages := []llm.Message{
		{Role: llm.RoleUser, Content: "Hello"},
	}

	// Mock the LLM API response
	mockResponse1 := llm.ChatCompletionResponse{
		Choices: []llm.Choice{
			{
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "",
					ToolCalls: []llm.ToolCall{
						{
							ID:   "testFunction",
							Type: "function",
							Function: struct {
								Name      string "json:\"name\""
								Arguments string "json:\"arguments\""
							}{
								Name:      "testFunction",
								Arguments: `{"arg1": "value1"}`,
							},
						},
					},
				},
			},
		},
	}

	mockResponse2 := llm.ChatCompletionResponse{
		Choices: []llm.Choice{
			{
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "Here is the result of the function.",
				},
			},
		},
	}

	mockClient.On("CreateChatCompletion", mock.Anything, mock.Anything).Return(mockResponse1, nil).Once()
	mockClient.On("CreateChatCompletion", mock.Anything, mock.Anything).Return(mockResponse2, nil).Once()

	response, err := sw.Run(ctx, agent, messages, nil, "", false, false, 5, true)

	assert.NoError(t, err)
	assert.Len(t, response.Messages, 3)
	assert.Equal(t, "TestAgent", response.Agent.Name)
	assert.Equal(t, "Here is the result of the function.", response.Messages[2].Content)
}

// TestRunFunctionCallError tests the Run method when function call returns an error
func TestRunFunctionCallError(t *testing.T) {
	mockClient := new(MockLLM)
	sw := NewMockSwarm(mockClient)
	ctx := context.Background()

	agentFunction := AgentFunction{
		Name:        "testFunction",
		Description: "A test function",
		Function: func(args map[string]interface{}, contextVariables map[string]interface{}) Result {
			return Result{
				Success: true,
				Data:    "Function executed successfully",
			}
		},
	}

	agent := &Agent{
		Name:      "TestAgent",
		Functions: []AgentFunction{agentFunction},
		Model:     "test-model",
	}

	messages := []llm.Message{
		{Role: llm.RoleUser, Content: "Hello"},
	}

	// Mock the LLM API to return an error
	mockClient.On("CreateChatCompletion", mock.Anything, mock.Anything).Return(llm.ChatCompletionResponse{}, errors.New("API error"))

	response, err := sw.Run(ctx, agent, messages, nil, "", false, false, 5, true)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API error")
	assert.Len(t, response.Messages, 0)
}

// TestProcessAndPrintResponse tests the ProcessAndPrintResponse function
func TestProcessAndPrintResponse(t *testing.T) {
	response := Response{
		Messages: []llm.Message{
			{
				Role:    llm.RoleAssistant,
				Name:    "TestAgent",
				Content: "Hello, how can I assist you?",
			},
			{
				Role:    llm.RoleAssistant,
				Name:    "testFunction",
				Content: "Function output",
			},
		},
	}

	// Capture the output
	var buf bytes.Buffer
	writer := io.MultiWriter(os.Stdout, &buf)
	log.SetOutput(writer)

	ProcessAndPrintResponse(response)

	output := buf.String()
	assert.NotNil(t, output)
}
