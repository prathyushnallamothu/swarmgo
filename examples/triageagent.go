package main

import (
	"fmt"
	"os"

	dotenv "github.com/joho/godotenv"
	swarmgo "github.com/prathyushnallamothu/swarmgo"
)

func processRefund(args map[string]interface{}, contextVariables map[string]interface{}) swarmgo.Result {
	itemID := args["item_id"].(string)
	reason := "NOT SPECIFIED"
	if val, ok := args["reason"].(string); ok {
		reason = val
	}
	fmt.Printf("[mock] Refunding item %s because %s...\n", itemID, reason)
	return swarmgo.Result{
		Value: fmt.Sprintf("Refunded item %s because %s.", itemID, reason),
	}
}

func applyDiscount(args map[string]interface{}, contextVariables map[string]interface{}) swarmgo.Result {
	fmt.Println("[mock] Applying discount...")
	return swarmgo.Result{
		Value: "Applied discount of 11%",
	}
}

func transferBackToTriage(args map[string]interface{}, contextVariables map[string]interface{}) swarmgo.Result {
	return swarmgo.Result{
		Agent: triageAgent,
		Value: "Transferring back to TriageAgent.",
	}
}

func transferToSales(args map[string]interface{}, contextVariables map[string]interface{}) swarmgo.Result {
	return swarmgo.Result{
		Agent: salesAgent,
		Value: "Transferring to SalesAgent.",
	}
}

func transferToRefunds(args map[string]interface{}, contextVariables map[string]interface{}) swarmgo.Result {
	return swarmgo.Result{
		Agent: refundsAgent,
		Value: "Transferring to RefundsAgent.",
	}
}

var triageAgent *swarmgo.Agent
var salesAgent *swarmgo.Agent
var refundsAgent *swarmgo.Agent

func initAgents() {
	triageAgent = &swarmgo.Agent{
		Name:         "TriageAgent",
		Instructions: "Determine which agent is best suited to handle the user's request, and transfer the conversation to that agent.",
		Model:        "gpt-4",
	}

	salesAgent = &swarmgo.Agent{
		Name:         "SalesAgent",
		Instructions: "Be super enthusiastic about selling bees. If the user's request is unrelated to sales and related to discount or refund, call the 'transferBackToTriage' function to transfer the conversation back to the triage agent.",
		Model:        "gpt-4",
	}

	refundsAgent = &swarmgo.Agent{
		Name:         "RefundsAgent",
		Instructions: "Assist the user with refund inquiries. If the reason is that it was too expensive, offer the user a discount code. If they insist, acknowledge their request and inform them that the refund process will be initiated through the appropriate channels.",
		Functions: []swarmgo.AgentFunction{
			{
				Name:        "processRefund",
				Description: "Process a refund request. Confirm with the user that they wish to proceed with the refund without asking for personal details.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"item_id": map[string]interface{}{
							"type":        "string",
							"description": "The ID of the item to refund.",
						},
						"reason": map[string]interface{}{
							"type":        "string",
							"description": "The reason for the refund.",
						},
					},
					"required": []interface{}{"item_id"},
				},
				Function: processRefund,
			},
			{
				Name:        "applyDiscount",
				Description: "Apply a discount to the user's cart.",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
				Function: applyDiscount,
			},
		},
		Model: "gpt-4",
	}

	// Assign functions to agents
	triageAgent.Functions = []swarmgo.AgentFunction{
		{
			Name:        "transferToSales",
			Description: "Transfer the conversation to the SalesAgent.",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
			Function: transferToSales,
		},
		{
			Name:        "transferToRefunds",
			Description: "Transfer the conversation to the RefundsAgent.",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
			Function: transferToRefunds,
		},
	}

	salesAgent.Functions = []swarmgo.AgentFunction{
		{
			Name:        "transferBackToTriage",
			Description: "If you are unable to assist the user or if the user's request is outside your expertise, call this function to transfer the conversation back to the triage agent.",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
			Function: transferBackToTriage,
		},
	}

	refundsAgent.Functions = append(refundsAgent.Functions, swarmgo.AgentFunction{
		Name:        "transferBackToTriage",
		Description: "If you are unable to assist the user or if the user's request is outside your expertise, call this function to transfer the conversation back to the triage agent.",
		Parameters: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Function: transferBackToTriage,
	})
}

func triageAgentExample() {
	dotenv.Load()

	client := swarmgo.NewSwarm(os.Getenv("OPENAI_API_KEY"))

	initAgents() // Initialize agents and their functions

	swarmgo.RunDemoLoop(client, triageAgent)
}
