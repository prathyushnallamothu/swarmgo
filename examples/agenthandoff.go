package main

import (
	"context"
	"fmt"
	"log"
	"os"

	dotenv "github.com/joho/godotenv"
	swarmgo "github.com/prathyushnallamothu/swarmgo"
	openai "github.com/sashabaranov/go-openai"
)

func transferToSpanishAgent(args map[string]interface{}, contextVariables map[string]interface{}) swarmgo.Result {
	spanishAgent := &swarmgo.Agent{
		Name:         "SpanishAgent",
		Instructions: "You only speak Spanish.",
		Model:        "gpt-4",
	}
	return swarmgo.Result{
		Agent: spanishAgent,
		Value: "Transferring to Spanish Agent.",
	}
}
func agentHandoffExample() {
	dotenv.Load()

	client := swarmgo.NewSwarm(os.Getenv("OPENAI_API_KEY"))
	fmt.Println("Agent Handoff Example", os.Getenv("OPENAI_API_KEY"), os.Getenv("OPENAI_PROXY"))

	englishAgent := &swarmgo.Agent{
		Name:         "EnglishAgent",
		Instructions: "You only speak English.",
		Functions: []swarmgo.AgentFunction{
			{
				Name:        "transferToSpanishAgent",
				Description: "Transfer Spanish-speaking users immediately.",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
				Function: transferToSpanishAgent,
			},
		},
		Model: "gpt-4",
	}

	messages := []openai.ChatCompletionMessage{
		{Role: "user", Content: "Hola. ¿Cómo estás?"},
	}

	ctx := context.Background()
	response, err := client.Run(ctx, englishAgent, messages, nil, "", false, false, 5, true)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Printf("%s: %s\n", response.Agent.Name, response.Messages[len(response.Messages)-1].Content)
}
