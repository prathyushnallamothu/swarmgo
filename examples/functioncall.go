package main

import (
	"context"
	"fmt"
	"os"

	dotenv "github.com/joho/godotenv"
	swarmgo "github.com/prathyushnallamothu/swarmgo"
	openai "github.com/sashabaranov/go-openai"
)

func getWeather1(args map[string]interface{}, contextVariables map[string]interface{}) swarmgo.Result {
	location := args["location"].(string)
	return swarmgo.Result{
		Value: fmt.Sprintf("{'temp':67, 'unit':'F', 'location':'%s'}", location),
	}
}
func functionCallExample() {
	dotenv.Load()
	client := swarmgo.NewSwarm(os.Getenv("OPENAI_API_KEY"))

	agent := &swarmgo.Agent{
		Name:         "Agent",
		Instructions: "You are a helpful agent.",
		Functions: []swarmgo.AgentFunction{
			{
				Name:        "getWeather",
				Description: "Get the current weather in a given location.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "The city to get the weather for",
						},
					},
					"required": []string{"location"},
				},
				Function: getWeather1,
			},
		},
		Model: "gpt-4",
	}

	messages := []openai.ChatCompletionMessage{
		{Role: "user", Content: "What's the weather in NYC?"},
	}

	ctx := context.Background()
	response, err := client.Run(ctx, agent, messages, nil, "", false, false, 5, true)
	if err != nil {
		panic(err)
	}

	fmt.Println(response.Messages[len(response.Messages)-1].Content)

}
