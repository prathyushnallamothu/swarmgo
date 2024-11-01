package main

import (
	"fmt"
	"os"

	dotenv "github.com/joho/godotenv"
	swarmgo "github.com/prathyushnallamothu/swarmgo"
)

func getWeather(args map[string]interface{}, contextVariables map[string]interface{}) swarmgo.Result {
	location := args["location"].(string)
	time := "now"
	if t, ok := args["time"].(string); ok {
		time = t
	}
	return swarmgo.Result{
		Value: fmt.Sprintf(`{"location": "%s", "temperature": "65", "time": "%s"}`, location, time),
	}
}

func sendEmail(args map[string]interface{}, contextVariables map[string]interface{}) swarmgo.Result {
	recipient := args["recipient"].(string)
	subject := args["subject"].(string)
	body := args["body"].(string)
	fmt.Println("Sending email...")
	fmt.Printf("To: %s\nSubject: %s\nBody: %s\n", recipient, subject, body)
	return swarmgo.Result{
		Value: "Sent!",
	}
}

func weatherAgentExample() {
	dotenv.Load()

	client := swarmgo.NewSwarm(os.Getenv("OPENAI_API_KEY"))

	weatherAgent := &swarmgo.Agent{
		Name:         "WeatherAgent",
		Instructions: "You are a helpful agent.",
		Functions: []swarmgo.AgentFunction{
			{
				Name:        "getWeather",
				Description: "Get the current weather in a given location. Location MUST be a city.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "The city to get the weather for",
						},
						"time": map[string]interface{}{
							"type":        "string",
							"description": "The time to get the weather for",
						},
					},
					"required": []interface{}{"location"},
				},
				Function: getWeather,
			},
			{
				Name:        "sendEmail",
				Description: "Send an email to a recipient.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"recipient": map[string]interface{}{
							"type":        "string",
							"description": "The recipient of the email",
						},
						"subject": map[string]interface{}{
							"type":        "string",
							"description": "The subject of the email",
						},
						"body": map[string]interface{}{
							"type":        "string",
							"description": "The body of the email",
						},
					},
					"required": []interface{}{"recipient", "subject", "body"},
				},
				Function: sendEmail,
			},
		},
		Model: "gpt-4",
	}
	swarmgo.RunDemoLoop(client, weatherAgent)
}
