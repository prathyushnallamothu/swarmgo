![Swarmgo Logo](assets/logo.png)
# SwarmGo (agents-sdk-go)

SwarmGo is a Go package that allows you to create AI agents capable of interacting, coordinating, and executing tasks. Inspired by OpenAI's Swarm framework, SwarmGo focuses on making agent coordination and execution lightweight, highly controllable, and easily testable.

It achieves this through two primitive abstractions: Agents and handoffs. An Agent encompasses instructions and tools (functions it can execute), and can at any point choose to hand off a conversation to another Agent.

These primitives are powerful enough to express rich dynamics between tools and networks of agents, allowing you to build scalable, real-world solutions while avoiding a steep learning curve.

## Table of Contents

- [Why SwarmGo](#why-swarmgo)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Usage](#usage)
  - [Creating an Agent](#creating-an-agent)
  - [Running the Agent](#running-the-agent)
  - [Adding Functions (Tools)](#adding-functions-tools)
  - [Using Context Variables](#using-context-variables)
  - [Memory Management](#memory-management)
- [Agent Handoff](#agent-handoff)
- [Streaming Support](#streaming-support)
- [Concurrent Agent Execution](#concurrent-agent-execution)
- [LLM Interface](#llm-interface)
- [Workflows](#workflows)
  - [1. Supervisor Workflow](#1-supervisor-workflow)
  - [2. Hierarchical Workflow](#2-hierarchical-workflow)
  - [3. Collaborative Workflow](#3-collaborative-workflow)
- [Examples](#examples)
- [Contributing](#contributing)
- [License](#license)
- [Acknowledgments](#acknowledgments)

## Why SwarmGo

SwarmGo explores patterns that are lightweight, scalable, and highly customizable by design. It's best suited for situations dealing with a large number of independent capabilities and instructions that are difficult to encode into a single prompt.

SwarmGo runs (almost) entirely on the client and, much like the Chat Completions API, does not store state between calls.

## Installation

```bash
go get github.com/prathyushnallamothu/swarmgo
```

## Quick Start
Here's a simple example to get you started:

```go
package main

import (
	"context"
	"fmt"
	"log"

	swarmgo "github.com/prathyushnallamothu/swarmgo"
	openai "github.com/sashabaranov/go-openai"
	llm "github.com/prathyushnallamothu/swarmgo/llm"
)

func main() {
	client := swarmgo.NewSwarm("YOUR_OPENAI_API_KEY", llm.OpenAI)

	agent := &swarmgo.Agent{
		Name:         "Agent",
		Instructions: "You are a helpful assistant.",
		Model:        "gpt-3.5-turbo",
	}

	messages := []openai.ChatCompletionMessage{
		{Role: "user", Content: "Hello!"},
	}

	ctx := context.Background()
	response, err := client.Run(ctx, agent, messages, nil, "", false, false, 5, true)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Println(response.Messages[len(response.Messages)-1].Content)
}
```

## Usage
Creating an Agent
An Agent represents an AI assistant with specific instructions and functions (tools) it can use.

```go
agent := &swarmgo.Agent{
	Name:         "Agent",
	Instructions: "You are a helpful assistant.",
	Model:        "gpt-4o",
}
```

- Name: Identifier for the agent (no spaces or special characters).
- Instructions: The system prompt or instructions for the agent.
- Model: The OpenAI model to use (e.g., "gpt-4o").

### Running the Agent

To interact with the agent, use the Run method:

```go
messages := []openai.ChatCompletionMessage{
	{Role: "user", Content: "Hello!"},
}

ctx := context.Background()
response, err := client.Run(ctx, agent, messages, nil, "", false, false, 5, true)
if err != nil {
	log.Fatalf("Error: %v", err)
}

fmt.Println(response.Messages[len(response.Messages)-1].Content)
```

### Adding Functions (Tools)

Agents can use functions to perform specific tasks. Functions are defined and then added to an agent.

### Defining a Function

```go
func getWeather(args map[string]interface{}, contextVariables map[string]interface{}) swarmgo.Result {
	location := args["location"].(string)
	// Simulate fetching weather data
	return swarmgo.Result{
		Value: fmt.Sprintf(`{"temp": 67, "unit": "F", "location": "%s"}`, location),
	}
}
```

### Adding the Function to an Agent

```go
agent.Functions = []swarmgo.AgentFunction{
	{
		Name:        "getWeather",
		Description: "Get the current weather in a given location.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location": map[string]interface{}{
					"type":        "string",
					"description": "The city to get the weather for.",
				},
			},
			"required": []interface{}{"location"},
		},
		Function: getWeather,
	},
}
```

### Using Context Variables

Context variables allow you to pass information between function calls and agents.

### Using Context Variables in Instructions

```go
func instructions(contextVariables map[string]interface{}) string {
	name, ok := contextVariables["name"].(string)
	if !ok {
		name = "User"
	}
	return fmt.Sprintf("You are a helpful assistant. Greet the user by name (%s).", name)
}

agent.InstructionsFunc = instructions
```

### Agent Handoff

Agents can hand off conversations to other agents. This is useful for delegating tasks or escalating when an agent is unable to handle a request.

```go
func transferToAnotherAgent(args map[string]interface{}, contextVariables map[string]interface{}) swarmgo.Result {
	anotherAgent := &swarmgo.Agent{
		Name:         "AnotherAgent",
		Instructions: "You are another agent.",
		Model:        "gpt-3.5-turbo",
	}
	return swarmgo.Result{
		Agent: anotherAgent,
		Value: "Transferring to AnotherAgent.",
	}
}
```

### Adding Handoff Functions to Agents

```go
agent.Functions = append(agent.Functions, swarmgo.AgentFunction{
	Name:        "transferToAnotherAgent",
	Description: "Transfer the conversation to AnotherAgent.",
	Parameters: map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	},
	Function: transferToAnotherAgent,
})
```


## Streaming Support

SwarmGo now includes built-in support for streaming responses, allowing real-time processing of AI responses and tool calls. This is particularly useful for long-running operations or when you want to provide immediate feedback to users.

### Using Streaming

To use streaming, implement the `StreamHandler` interface:

```go
type StreamHandler interface {
    OnStart()
    OnToken(token string)
    OnToolCall(toolCall openai.ToolCall)
    OnComplete(message openai.ChatCompletionMessage)
    OnError(err error)
}
```

A default implementation (`DefaultStreamHandler`) is provided, but you can create your own handler for custom behavior:

```go
type CustomStreamHandler struct {
    totalTokens int
}

func (h *CustomStreamHandler) OnStart() {
    fmt.Println("Starting stream...")
}

func (h *CustomStreamHandler) OnToken(token string) {
    h.totalTokens++
    fmt.Print(token)
}

func (h *CustomStreamHandler) OnComplete(msg openai.ChatCompletionMessage) {
    fmt.Printf("\nComplete! Total tokens: %d\n", h.totalTokens)
}

func (h *CustomStreamHandler) OnError(err error) {
    fmt.Printf("Error: %v\n", err)
}

func (h *CustomStreamHandler) OnToolCall(tool openai.ToolCall) {
    fmt.Printf("\nUsing tool: %s\n", tool.Function.Name)
}
```

### Streaming Example

Here's an example of using streaming with a file analyzer:

```go
client := swarmgo.NewSwarm("YOUR_OPENAI_API_KEY", llm.OpenAI)

agent := &swarmgo.Agent{
    Name:         "FileAnalyzer",
    Instructions: "You are an assistant that analyzes files.",
    Model:        "gpt-4",
}

handler := &CustomStreamHandler{}
err := client.StreamingResponse(
    context.Background(),
    agent,
    messages,
    nil,
    "",
    handler,
    true,
)
```
For a complete example of file analysis with streaming, see [examples/file_analyzer_stream/main.go](examples/file_analyzer_stream/main.go).


### Concurrent Agent Execution

SwarmGo supports running multiple agents concurrently using the `ConcurrentSwarm` type. This is particularly useful when you need to parallelize agent tasks or run multiple analyses simultaneously.

```go
// Create a concurrent swarm
cs := swarmgo.NewConcurrentSwarm(apiKey)

// Configure multiple agents
configs := map[string]swarmgo.AgentConfig{
    "agent1": {
        Agent: agent1,
        Messages: []openai.ChatCompletionMessage{
            {Role: openai.ChatMessageRoleUser, Content: "Task 1"},
        },
        MaxTurns: 1,
        ExecuteTools: true,
    },
    "agent2": {
        Agent: agent2,
        Messages: []openai.ChatCompletionMessage{
            {Role: openai.ChatMessageRoleUser, Content: "Task 2"},
        },
        MaxTurns: 1,
        ExecuteTools: true,
    },
}

// Run agents concurrently with a timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

results := cs.RunConcurrent(ctx, configs)

// Process results
for _, result := range results {
    if result.Error != nil {
        log.Printf("Error in %s: %v\n", result.AgentName, result.Error)
        continue
    }
    // Handle successful response
    fmt.Printf("Result from %s: %s\n", result.AgentName, result.Response)
}
```

Key features of concurrent execution:
- Run multiple agents in parallel with independent configurations
- Context-based timeout and cancellation support
- Thread-safe result collection
- Support for both ordered and unordered execution
- Error handling for individual agent failures

See the `examples/concurrent_analyzer/main.go` for a complete example of concurrent code analysis using multiple specialized agents.


## Memory Management

SwarmGo includes a built-in memory management system that allows agents to store and recall information across conversations. The memory system supports both short-term and long-term memory, with features for organizing and retrieving memories based on type and context.

```go
// Create a new agent with memory capabilities
agent := swarmgo.NewAgent("MyAgent", "gpt-4")

// Memory is automatically managed for conversations and tool calls
// You can also explicitly store memories:
memory := swarmgo.Memory{
    Content:    "User prefers dark mode",
    Type:       "preference",
    Context:    map[string]interface{}{"setting": "ui"},
    Timestamp:  time.Now(),
    Importance: 0.8,
}
agent.Memory.AddMemory(memory)

// Retrieve recent memories
recentMemories := agent.Memory.GetRecentMemories(5)

// Search specific types of memories
preferences := agent.Memory.SearchMemories("preference", nil)
```

Key features of the memory system:
- **Automatic Memory Management**: Conversations and tool interactions are automatically stored
- **Memory Types**: Organize memories by type (conversation, fact, tool_result, etc.)
- **Context Association**: Link memories with relevant context
- **Importance Scoring**: Assign priority levels to memories (0-1)
- **Memory Search**: Search by type and context
- **Persistence**: Save and load memories to/from disk
- **Thread Safety**: Concurrent memory access is protected
- **Short-term Buffer**: Recent memories are kept in a FIFO buffer
- **Long-term Storage**: Organized storage by memory type

See the [memory_demo](examples/memory_demo/main.go) example for a complete demonstration of memory capabilities.

## LLM Interface

SwarmGo provides a flexible LLM (Language Learning Model) interface that supports multiple providers:
currently OpenAI and Gemini.

To initialize a new Swarm with a specific provider:

```go
// Initialize with OpenAI
client := swarmgo.NewSwarm("YOUR_API_KEY", llm.OpenAI)

// Initialize with Gemini
client := swarmgo.NewSwarm("YOUR_API_KEY", llm.Gemini)
```

## Workflows

Workflows in SwarmGo provide structured patterns for organizing and coordinating multiple agents. They help manage complex interactions between agents, define communication paths, and establish clear hierarchies or collaboration patterns. Think of workflows as the orchestration layer that determines how your agents work together to accomplish tasks.

Each workflow type serves a different organizational need:

### 1. Supervisor Workflow
A hierarchical pattern where a supervisor agent oversees and coordinates tasks among worker agents. This is ideal for:
- Task delegation and monitoring
- Quality control and oversight
- Centralized decision making
- Resource allocation across workers

```go
workflow := swarmgo.NewWorkflow(apiKey, llm.OpenAI, swarmgo.SupervisorWorkflow)

// Add agents to teams
workflow.AddAgentToTeam(supervisorAgent, swarmgo.SupervisorTeam)
workflow.AddAgentToTeam(workerAgent1, swarmgo.WorkerTeam)
workflow.AddAgentToTeam(workerAgent2, swarmgo.WorkerTeam)

// Set supervisor as team leader
workflow.SetTeamLeader(supervisorAgent.Name, swarmgo.SupervisorTeam)

// Connect agents
workflow.ConnectAgents(supervisorAgent.Name, workerAgent1.Name)
workflow.ConnectAgents(supervisorAgent.Name, workerAgent2.Name)
```

### 2. Hierarchical Workflow
A tree-like structure where tasks flow from top to bottom through multiple levels. This pattern is best for:
- Complex task decomposition
- Specialized agent roles at each level
- Clear reporting structures
- Sequential processing pipelines

```go
workflow := swarmgo.NewWorkflow(apiKey, llm.OpenAI, swarmgo.HierarchicalWorkflow)

// Add agents to teams
workflow.AddAgentToTeam(managerAgent, swarmgo.SupervisorTeam)
workflow.AddAgentToTeam(researchAgent, swarmgo.ResearchTeam)
workflow.AddAgentToTeam(analysisAgent, swarmgo.AnalysisTeam)

// Connect agents in hierarchy
workflow.ConnectAgents(managerAgent.Name, researchAgent.Name)
workflow.ConnectAgents(researchAgent.Name, analysisAgent.Name)
```

### 3. Collaborative Workflow
A peer-based pattern where agents work together as equals, passing tasks between them as needed. This approach excels at:
- Team-based problem solving
- Parallel processing
- Iterative refinement
- Dynamic task sharing

```go
workflow := swarmgo.NewWorkflow(apiKey, llm.OpenAI, swarmgo.CollaborativeWorkflow)

// Add agents to document team
workflow.AddAgentToTeam(editor, swarmgo.DocumentTeam)
workflow.AddAgentToTeam(reviewer, swarmgo.DocumentTeam)
workflow.AddAgentToTeam(writer, swarmgo.DocumentTeam)

// Connect agents in collaborative pattern
workflow.ConnectAgents(editor.Name, reviewer.Name)
workflow.ConnectAgents(reviewer.Name, writer.Name)
workflow.ConnectAgents(writer.Name, editor.Name)
```

Key workflow features:
- **Team Management**: Organize agents into functional teams
- **Leadership Roles**: Designate team leaders for coordination
- **Flexible Routing**: Dynamic task routing between agents
- **Cycle Detection**: Built-in cycle detection and handling
- **State Management**: Share state between agents in a workflow
- **Error Handling**: Robust error handling and recovery

## Examples

For more examples, see the [examples](examples) directory.

## Contributing
Contributions are welcome! Please follow these steps:

1. Fork the repository.
2. Create a new branch (git checkout -b feature/YourFeature).
3. Commit your changes (git commit -am 'Add a new feature').
4. Push to the branch (git push origin feature/YourFeature).
5. Open a Pull Request.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Thanks to OpenAI for the inspiration and the [Swarm framework](https://github.com/openai/swarm).
- Thanks to [Sashabaranov](https://github.com/sashabaranov) for the [go-openai](https://github.com/sashabaranov/go-openai) package.
