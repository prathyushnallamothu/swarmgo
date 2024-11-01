package main

import (
	"context"
	"fmt"
	"os"

	dotenv "github.com/joho/godotenv"
	swarmgo "github.com/prathyushnallamothu/swarmgo"
	openai "github.com/sashabaranov/go-openai"
)

func basicExample() {
	dotenv.Load()

	client := swarmgo.NewSwarm(os.Getenv("OPENAI_API_KEY"))

	agent := &swarmgo.Agent{
		Name:         "Agent",
		Instructions: "You are a helpful agent.",
		Model:        "gpt-3.5-turbo",
	}

	messages := []openai.ChatCompletionMessage{
		{Role: "user", Content: "Hi!"},
	}

	ctx := context.Background()
	response, err := client.Run(ctx, agent, messages, nil, "", false, false, 5, true)
	if err != nil {
		panic(err)
	}

	fmt.Println(response.Messages[len(response.Messages)-1].Content)
}
