package swarmgo

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/prathyushnallamothu/swarmgo/llm"
)

// WorkflowType defines the type of agent interaction pattern
type WorkflowType int

const (
	CollaborativeWorkflow WorkflowType = iota
	SupervisorWorkflow
	HierarchicalWorkflow
)

// TeamType represents a type of agent team
type TeamType string

const (
	ResearchTeam   TeamType = "research"
	DocumentTeam   TeamType = "document"
	SupervisorTeam TeamType = "supervisor"
	AnalysisTeam   TeamType = "analysis"
	DeveloperTeam  TeamType = "developer"
)

// CycleHandling represents how to handle detected cycles
type CycleHandling int

const (
	StopOnCycle CycleHandling = iota
	ContinueOnCycle
)

// Workflow represents a collection of agents and their connections.
type Workflow struct {
	swarm         *Swarm
	agents        map[string]*Agent
	connections   map[string][]string
	workflowType  WorkflowType
	sharedState   map[string]interface{}
	agentStates   map[string]map[string]interface{}
	teams         map[TeamType][]*Agent // Agents grouped by team
	teamLeaders   map[TeamType]string   // Team leader for each team
	currentAgent  string                // Track current active agent
	routingLog    []string              // Log of agent transitions
	cycleHandling CycleHandling
	cycleCallback func(from, to string) (bool, error) // Callback for cycle detection
	stepResults   []StepResult                        // Track results of each step
	currentStep   int                                 // Current step number
	visualHook    VisualizationHook                   // Add visualization hook
}

// VisualizationHook defines the interface for workflow visualization
type VisualizationHook interface {
	OnWorkflowStart(workflow *Workflow)
	OnAgentStart(agentName string, step int)
	OnAgentComplete(agentName string, step int, duration time.Duration)
	OnMessageSent(fromAgent, toAgent, content string)
	OnCycleDetected(fromAgent, toAgent string, count int)
	OnWorkflowEnd(workflow *Workflow)
}

// NewWorkflow initializes a new Workflow instance.
func NewWorkflow(apikey string, provider llm.LLMProvider, workflowType WorkflowType) *Workflow {
	swarm := NewSwarm(apikey, provider)
	return NewWorkflowWithSwarm(swarm)
}

func NewWorkflowWithSwarm(swarm *Swarm) *Workflow {
	return &Workflow{
		swarm:         swarm,
		agents:        make(map[string]*Agent),
		connections:   make(map[string][]string),
		workflowType:  CollaborativeWorkflow,
		sharedState:   make(map[string]interface{}),
		agentStates:   make(map[string]map[string]interface{}),
		teams:         make(map[TeamType][]*Agent),
		teamLeaders:   make(map[TeamType]string),
		routingLog:    make([]string, 0),
		cycleHandling: StopOnCycle,
	}
}

// SetCycleCallback sets a callback function to be called when a cycle is detected
func (wf *Workflow) SetCycleCallback(callback func(from, to string) (bool, error)) {
	wf.cycleCallback = callback
}

// SetCycleHandling sets how cycles should be handled
func (wf *Workflow) SetCycleHandling(handling CycleHandling) {
	wf.cycleHandling = handling
}

// SetVisualizationHook sets the visualization hook for the workflow
func (wf *Workflow) SetVisualizationHook(hook VisualizationHook) {
	wf.visualHook = hook
}

// logTransition logs agent transitions for debugging
func (wf *Workflow) logTransition(from, to string, reason string) {
	log := fmt.Sprintf("Transition: %s -> %s (%s)", from, to, reason)
	wf.routingLog = append(wf.routingLog, log)
	fmt.Printf("\033[93m%s\033[0m\n", log)
}

// GetCurrentAgent returns the currently active agent
func (wf *Workflow) GetCurrentAgent() string {
	return wf.currentAgent
}

// GetRoutingLog returns the routing history
func (wf *Workflow) GetRoutingLog() []string {
	return wf.routingLog
}

// GetAgents returns all agents in the workflow
func (wf *Workflow) GetAgents() map[string]*Agent {
	return wf.agents
}

// GetConnections returns all connections in the workflow
func (wf *Workflow) GetConnections() map[string][]string {
	return wf.connections
}

// GetTeams returns all teams in the workflow
func (wf *Workflow) GetTeams() map[TeamType][]*Agent {
	return wf.teams
}

// GetTeamLeaders returns all team leaders in the workflow
func (wf *Workflow) GetTeamLeaders() map[TeamType]string {
	return wf.teamLeaders
}

// AddAgent adds an agent to the workflow.
func (wf *Workflow) AddAgent(agent *Agent) {
	wf.agents[agent.Name] = agent
	wf.agentStates[agent.Name] = make(map[string]interface{})
}

// AddAgentToTeam adds an agent to a specific team
func (wf *Workflow) AddAgentToTeam(agent *Agent, team TeamType) {
	wf.agents[agent.Name] = agent
	wf.agentStates[agent.Name] = make(map[string]interface{})
	wf.teams[team] = append(wf.teams[team], agent)
}

// SetTeamLeader designates an agent as the leader of a team
func (wf *Workflow) SetTeamLeader(agentName string, team TeamType) error {
	if _, exists := wf.agents[agentName]; !exists {
		return errors.New("agent does not exist")
	}
	wf.teamLeaders[team] = agentName
	return nil
}

// Execute runs the workflow and returns detailed results including step outcomes
func (wf *Workflow) Execute(startAgent string, userRequest string) (*WorkflowResult, error) {
	result := &WorkflowResult{
		Steps:     make([]StepResult, 0),
		StartTime: time.Now(),
	}

	if _, exists := wf.agents[startAgent]; !exists {
		return result, errors.New("startAgent does not exist")
	}

	// Notify visualization of workflow start
	if wf.visualHook != nil {
		wf.visualHook.OnWorkflowStart(wf)
	}

	messageHistory := []llm.Message{{Role: llm.RoleUser, Content: userRequest}}
	visited := make(map[string]bool)
	cycleCount := make(map[string]int)
	wf.currentAgent = startAgent
	wf.currentStep = 0
	wf.logTransition("start", startAgent, "workflow initialization")

	for {
		// Start new step
		stepResult := StepResult{
			AgentName:  wf.currentAgent,
			Input:      messageHistory,
			StartTime:  time.Now(),
			StepNumber: wf.currentStep + 1,
		}

		// Notify visualization of agent start
		if wf.visualHook != nil {
			wf.visualHook.OnAgentStart(wf.currentAgent, stepResult.StepNumber)
		}

		// Execute current agent
		fmt.Printf("\033[96mExecuting agent: %s (Step %d)\033[0m\n", wf.currentAgent, stepResult.StepNumber)
		response, err := wf.executeAgent(wf.currentAgent, messageHistory)
		stepResult.EndTime = time.Now()

		// Notify visualization of agent completion
		if wf.visualHook != nil {
			wf.visualHook.OnAgentComplete(wf.currentAgent, stepResult.StepNumber, stepResult.EndTime.Sub(stepResult.StartTime))
		}

		if err != nil {
			stepResult.Error = err
			result.Steps = append(result.Steps, stepResult)
			result.Error = err
			result.EndTime = time.Now()
			return result, err
		}

		stepResult.Output = response
		messageHistory = append(messageHistory, response...)

		// Notify visualization of message sent
		if wf.visualHook != nil && len(response) > 0 {
			wf.visualHook.OnMessageSent(wf.currentAgent, stepResult.NextAgent, response[len(response)-1].Content)
		}

		// Determine next agent
		nextAgent, shouldContinue := wf.routeToNextAgent(wf.currentAgent, messageHistory)
		stepResult.NextAgent = nextAgent

		// Store step result
		wf.stepResults = append(wf.stepResults, stepResult)
		result.Steps = append(result.Steps, stepResult)
		wf.currentStep++

		if !shouldContinue {
			wf.logTransition(wf.currentAgent, "end", "workflow complete")
			break
		}

		// Check for cycles
		if visited[nextAgent] {
			cycleCount[nextAgent]++
			reason := fmt.Sprintf("cycle detected (%d times)", cycleCount[nextAgent])
			wf.logTransition(wf.currentAgent, nextAgent, reason)

			// Notify visualization of cycle detection
			if wf.visualHook != nil {
				wf.visualHook.OnCycleDetected(wf.currentAgent, nextAgent, cycleCount[nextAgent])
			}

			switch wf.cycleHandling {
			case StopOnCycle:
				break
			case ContinueOnCycle:
				if wf.cycleCallback != nil {
					shouldContinue, err := wf.cycleCallback(wf.currentAgent, nextAgent)
					if err != nil {
						result.Error = fmt.Errorf("cycle callback error: %v", err)
						result.EndTime = time.Now()
						return result, result.Error
					}
					if !shouldContinue {
						result.EndTime = time.Now()
						result.FinalOutput = messageHistory
						return result, nil
					}
				}
				// Continue with the cycle
				wf.currentAgent = nextAgent
				continue
			}
			break
		}

		// Log transition and update current agent
		wf.logTransition(wf.currentAgent, nextAgent, "normal routing")
		wf.currentAgent = nextAgent
		visited[nextAgent] = true
	}

	result.EndTime = time.Now()
	result.FinalOutput = messageHistory

	// Notify visualization of workflow end
	if wf.visualHook != nil {
		wf.visualHook.OnWorkflowEnd(wf)
	}

	return result, nil
}

// GetStepResult returns the result of a specific step
func (wf *Workflow) GetStepResult(stepNumber int) (*StepResult, error) {
	if stepNumber < 1 || stepNumber > len(wf.stepResults) {
		return nil, fmt.Errorf("invalid step number: %d", stepNumber)
	}
	return &wf.stepResults[stepNumber-1], nil
}

// GetAllStepResults returns all step results
func (wf *Workflow) GetAllStepResults() []StepResult {
	return wf.stepResults
}

// GetLastStepResult returns the result of the last executed step
func (wf *Workflow) GetLastStepResult() (*StepResult, error) {
	if len(wf.stepResults) == 0 {
		return nil, errors.New("no steps executed yet")
	}
	return &wf.stepResults[len(wf.stepResults)-1], nil
}

// executeAgent executes a single agent and manages its state
func (wf *Workflow) executeAgent(agentName string, messageHistory []llm.Message) ([]llm.Message, error) {
	agent := wf.agents[agentName]
	fmt.Printf("\033[95mAgent %s processing message...\033[0m\n", agentName)

	// Prepare agent state
	var state map[string]interface{}
	if wf.workflowType == CollaborativeWorkflow {
		state = wf.sharedState
	} else {
		state = wf.agentStates[agentName]
	}

	// Execute agent
	response, err := wf.swarm.Run(
		context.Background(),
		agent,
		messageHistory,
		state,
		"",
		false,
		false,
		0,
		true,
	)
	if err != nil {
		fmt.Printf("\033[91mError executing agent %s: %v\033[0m\n", agentName, err)
		return nil, err
	}

	fmt.Printf("\033[92mAgent %s completed processing\033[0m\n", agentName)

	// Update state
	if wf.workflowType == CollaborativeWorkflow {
		wf.sharedState = state
	} else {
		wf.agentStates[agentName] = state
	}

	return response.Messages, nil
}

// routeToNextAgent determines the next agent based on workflow type and message content
func (wf *Workflow) routeToNextAgent(currentAgent string, messageHistory []llm.Message) (string, bool) {
	lastMessage := messageHistory[len(messageHistory)-1]
	//state := wf.agentStates[currentAgent]

	// Check for explicit routing instructions
	if containsRoutingInstruction(lastMessage.Content) {
		nextAgent := extractRoutingAgent(lastMessage.Content)
		if _, exists := wf.agents[nextAgent]; exists {
			return nextAgent, true
		}
	}

	// Handle different workflow types
	switch wf.workflowType {
	case SupervisorWorkflow:
		return wf.handleSupervisorRouting(currentAgent, messageHistory)
	case HierarchicalWorkflow:
		return wf.handleHierarchicalRouting(currentAgent, messageHistory)
	case CollaborativeWorkflow:
		return wf.handleCollaborativeRouting(currentAgent, messageHistory)
	}

	return "", false
}

func (wf *Workflow) handleSupervisorRouting(currentAgent string, messageHistory []llm.Message) (string, bool) {
	lastMessage := messageHistory[len(messageHistory)-1]
	content := strings.ToLower(lastMessage.Content)

	// Get supervisor name from team leaders
	supervisorName := wf.teamLeaders[SupervisorTeam]

	if currentAgent == supervisorName {
		// Task classification patterns
		taskTeams := map[string]TeamType{
			`(?i)(research|search|find|look up|scrape|collect)`:          ResearchTeam,
			`(?i)(write|draft|compose|create|document|chart|generate)`:   DocumentTeam,
			`(?i)(analyze|evaluate|assess|interpret|review|investigate)`: AnalysisTeam,
			`(?i)(code|develop|implement|program|debug|test|build)`:      DeveloperTeam,
		}

		// Determine appropriate team based on task
		for pattern, team := range taskTeams {
			re := regexp.MustCompile(pattern)
			if re.MatchString(content) {
				if leader, exists := wf.teamLeaders[team]; exists {
					return leader, true
				}
				// If no leader, try to find any team member
				if agents, exists := wf.teams[team]; exists && len(agents) > 0 {
					return agents[0].Name, true
				}
			}
		}
	} else {
		// Non-supervisor agents report back to supervisor
		return supervisorName, true
	}

	return "", false
}

func (wf *Workflow) handleHierarchicalRouting(currentAgent string, messageHistory []llm.Message) (string, bool) {
	lastMessage := messageHistory[len(messageHistory)-1]

	// If task is complete, route back to team leader or supervisor
	if isTaskComplete(lastMessage.Content) {
		// Find which team the current agent belongs to
		for team, agents := range wf.teams {
			for _, agent := range agents {
				if agent.Name == currentAgent {
					// Route to team leader if exists
					if leader, exists := wf.teamLeaders[team]; exists {
						return leader, true
					}
					// Otherwise route to supervisor
					return "supervisor", true
				}
			}
		}
	}

	// Check for specific tool/function calls
	if strings.Contains(lastMessage.Content, "function") || strings.Contains(lastMessage.Content, "tool") {
		// Route to appropriate specialized agent based on function type
		functionPatterns := map[string][]string{
			`(?i)(search|api)`:                   {"searcher", "web_scraper"},
			`(?i)(write|text)`:                   {"writer", "note_taker"},
			`(?i)(chart|graph|plot)`:             {"chart_generator"},
			`(?i)(analyze|evaluate|assess)`:      {"analyzer", "evaluator"},
			`(?i)(interpret|review|investigate)`: {"reviewer", "investigator"},
			`(?i)(code|program|implement)`:       {"developer", "programmer"},
			`(?i)(test|debug|fix)`:               {"tester", "debugger"},
			`(?i)(build|deploy|release)`:         {"builder", "deployer"},
			`(?i)(optimize|refactor|improve)`:    {"optimizer", "refactorer"},
		}

		for pattern, agents := range functionPatterns {
			re := regexp.MustCompile(pattern)
			if re.MatchString(lastMessage.Content) {
				for _, agentName := range agents {
					if _, exists := wf.agents[agentName]; exists {
						return agentName, true
					}
				}
			}
		}
	}

	return "", false
}

func (wf *Workflow) handleCollaborativeRouting(currentAgent string, messageHistory []llm.Message) (string, bool) {
	// In collaborative mode, agents share context and can work together
	lastMessage := messageHistory[len(messageHistory)-1]

	// Check if any connected agent hasn't processed this message
	for _, nextAgent := range wf.connections[currentAgent] {
		if !hasProcessedMessage(wf.agentStates[nextAgent], messageHistory) {
			return nextAgent, true
		}
	}

	// If all connected agents have processed, check if we need more processing
	if !isFinalAnswer(lastMessage.Content) {
		// Find an agent that might have relevant capabilities
		for name, _ := range wf.agents {
			if name != currentAgent && !hasProcessedMessage(wf.agentStates[name], messageHistory) {
				return name, true
			}
		}
	}

	return "", false
}

// Helper functions for message analysis
func containsRoutingInstruction(content string) bool {
	// Check for routing keywords like "route to", "send to", "forward to"
	routingKeywords := []string{
		"route to",
		"send to",
		"forward to",
		"delegate to",
		"assign to",
		"@",
	}

	content = strings.ToLower(content)
	for _, keyword := range routingKeywords {
		if strings.Contains(content, keyword) {
			return true
		}
	}
	return false
}

func extractRoutingAgent(content string) string {
	// Extract agent name after routing keywords
	routingPatterns := []string{
		`(?i)route to (\w+)`,
		`(?i)send to (\w+)`,
		`(?i)forward to (\w+)`,
		`(?i)delegate to (\w+)`,
		`(?i)assign to (\w+)`,
		`@(\w+)`,
	}

	content = strings.TrimSpace(content)
	for _, pattern := range routingPatterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(content); len(matches) > 1 {
			return matches[1]
		}
	}
	return ""
}

func isTaskComplete(content string) bool {
	// Check for task completion indicators
	completionKeywords := []string{
		"task complete",
		"completed",
		"finished",
		"done",
		"task accomplished",
		"objective achieved",
		"✓",
		"✔",
	}

	content = strings.ToLower(content)
	for _, keyword := range completionKeywords {
		if strings.Contains(content, keyword) {
			return true
		}
	}
	return false
}

func isFinalAnswer(content string) bool {
	// Check for final answer indicators
	finalKeywords := []string{
		"final answer",
		"final response",
		"final solution",
		"final result",
		"end workflow",
		"complete workflow",
		"FINAL:",
	}

	content = strings.ToLower(content)
	for _, keyword := range finalKeywords {
		if strings.Contains(content, keyword) {
			return true
		}
	}
	return false
}

func hasProcessedMessage(state map[string]interface{}, history []llm.Message) bool {
	if state == nil {
		return false
	}

	// Get processed message IDs from state
	processedIDs, ok := state["processed_message_ids"].([]string)
	if !ok {
		processedIDs = []string{}
	}

	// Generate ID for the last message
	lastMsg := history[len(history)-1]
	msgID := generateMessageID(lastMsg)

	// Check if message has been processed
	for _, id := range processedIDs {
		if id == msgID {
			return true
		}
	}

	// Mark message as processed
	state["processed_message_ids"] = append(processedIDs, msgID)
	return false
}

// generateMessageID creates a unique ID for a message based on its content and role
func generateMessageID(msg llm.Message) string {
	data := fmt.Sprintf("%s:%s", msg.Role, msg.Content)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash[:8]) // Use first 8 bytes for shorter ID
}

func (wf *Workflow) findParentAgent(agentName string) string {
	// Check all agents' connections to find the parent
	for agent, connections := range wf.connections {
		for _, conn := range connections {
			if conn == agentName {
				return agent // This agent is the parent
			}
		}
	}
	return ""
}

func (wf *Workflow) routeSupervisorToWorker(messageHistory []llm.Message) (string, bool) {
	lastMessage := messageHistory[len(messageHistory)-1]
	content := strings.ToLower(lastMessage.Content)

	// Define task-agent mappings
	taskPatterns := map[string][]string{
		`(?i)(research|search|find|look up)`:      {"researcher", "analyst"},
		`(?i)(write|draft|compose|create)`:        {"writer", "creator"},
		`(?i)(review|check|validate|verify)`:      {"reviewer", "validator"},
		`(?i)(analyze|evaluate|assess|interpret)`: {"analyzer", "evaluator"},
		`(?i)(investigate|examine|study|explore)`: {"investigator", "researcher"},
		`(?i)(calculate|compute|analyze)`:         {"calculator", "analyst"},
		`(?i)(summarize|summarise|recap)`:         {"summarizer", "writer"},
		`(?i)(code|develop|program|implement)`:    {"developer", "programmer"},
		`(?i)(test|debug|fix|resolve)`:            {"tester", "debugger"},
		`(?i)(build|deploy|release|package)`:      {"builder", "deployer"},
		`(?i)(optimize|refactor|improve)`:         {"optimizer", "refactorer"},
	}

	// Check each pattern and return appropriate agent
	for pattern, agents := range taskPatterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(content) {
			// Find first available agent
			for _, agent := range agents {
				if _, exists := wf.agents[agent]; exists {
					return agent, true
				}
			}
		}
	}

	// If no specific task pattern matched, try to extract agent name
	if agent := extractRoutingAgent(content); agent != "" {
		if _, exists := wf.agents[agent]; exists {
			return agent, true
		}
	}

	// Default to first available worker if no specific routing found
	for name := range wf.agents {
		if name != "supervisor" {
			return name, true
		}
	}

	return "", false
}

// ConnectAgents creates a connection between two agents.
func (wf *Workflow) ConnectAgents(fromAgent, toAgent string) error {
	if _, exists := wf.agents[fromAgent]; !exists {
		return errors.New("fromAgent does not exist")
	}
	if _, exists := wf.agents[toAgent]; !exists {
		return errors.New("toAgent does not exist")
	}
	wf.connections[fromAgent] = append(wf.connections[fromAgent], toAgent)
	return nil
}

// StepResult represents the outcome of a single workflow step
type StepResult struct {
	AgentName  string
	Input      []llm.Message
	Output     []llm.Message
	Error      error
	StartTime  time.Time
	EndTime    time.Time
	NextAgent  string
	StepNumber int
}

// WorkflowResult represents the complete workflow execution result
type WorkflowResult struct {
	Steps       []StepResult
	FinalOutput []llm.Message
	Error       error
	StartTime   time.Time
	EndTime     time.Time
}
