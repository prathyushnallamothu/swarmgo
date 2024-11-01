package swarmgo

import (
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

// FunctionToDefinition converts an AgentFunction to an openai.FunctionDefinition.
// It directly assigns the Name, Description, and Parameters from the AgentFunction.
func FunctionToDefinition(af AgentFunction) *openai.FunctionDefinition {
	return &openai.FunctionDefinition{
		Name:        af.Name,
		Description: af.Description,
		Parameters:  af.Parameters, // Assign directly without marshaling
	}
}

// ProcessAndPrintResponse processes a Response object and prints messages based on their role.
// It uses different colors for different roles: blue for "assistant" and magenta for "function" or "tool".
func ProcessAndPrintResponse(response Response) {
	for _, message := range response.Messages {
		if message.Role == "assistant" {
			// Print assistant messages in blue
			fmt.Printf("\033[94m%s\033[0m: %s\n", message.Name, message.Content)
		} else if message.Role == "function" || message.Role == "tool" {
			// Print function or tool messages in magenta
			fmt.Printf("\033[95mFunction %s\033[0m Output: %s\n", message.Name, message.Content)
		}
	}
}
