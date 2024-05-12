package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/gin-gonic/gin"
	ollama "github.com/ollama/ollama/api"
	"io"
	"net/http"
	"strings"
)

const (
	system_prompt_format = `You're game entity. 
Your name is %s.
You always give shortest possible answers, like when chatting on Discord. (never use emojis though)
%s

Here's a list of things you've heard from other entities (linked with your answers when relevant):

%s

Here's a message from another entity (%s), what's your answer?

%s
`
)

var (
	// TODO: store agents in JSON file to resume simulation
	// all data is wiped when restarting server so far.
	agents       map[string]*Agent // indexed by ID
	chromaClient *ChromaClient
	ollamaClient *ollama.Client
)

type Agent struct {
	System string `json:"system,omitempty"` // system prompt for the agent
	Name   string `json:"name,omitempty"`
	// initial NPC behavior code
	// agents with no initial code will never try to update it
	BehaviorCode string `json:"behavior-code,omitempty"`
	ID           string `json:"id,omitempty"`
	// Full system prompt, assembled using generic agent system prompt,
	// provided system prompt, agent's name & behavior code.
	FullSystemPrompt string `json:"-"`
}

func serveAPI(port string) {
	var err error

	gin.SetMode(gin.ReleaseMode)

	chromaClient, err = NewChromaClient(CHROMA_DB_HOST_ADDR, CHROMA_DB_TENANT, CHROMA_DB_DATABASE)
	if err != nil {
		fmt.Println("❌", err.Error())
		return
	}

	ollamaClient, _ = ollama.ClientFromEnvironment()

	agents = make(map[string]*Agent)
	fmt.Println(agents)

	router := gin.Default()
	router.POST("/agents", createAgent)
	router.POST("/agents/:id/ask", askAgent)

	fmt.Println("Serving API... (" + port + ")")
	router.Run(port)
}

func createAgent(c *gin.Context) {
	var agent Agent
	if err := c.BindJSON(&agent); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	agentID := strings.TrimSpace(strings.ToLower(agent.Name))
	agentID = strings.ReplaceAll(agentID, " ", "_")

	if oldAgent, exists := agents[agentID]; exists {
		agent = *oldAgent
		fmt.Println("⚠️ Agent already exists (not replacing it)")

	} else {
		agent.ID = agentID

		_, err := chromaClient.GetCollection(agent.ID)
		if err != nil {
			fmt.Println("❌", err.Error())
			return
		}

		// Key does not exist, insert it
		agents[agentID] = &agent
		fmt.Println("✨ Agent", agent.Name, "created (ID:"+agent.ID+")")
	}

	c.JSON(http.StatusOK, agent)
}

type AskAgentReq struct {
	Sender string `json:"sender,omitempty"` // name of the sender
	Prompt string `json:"prompt,omitempty"`
}

// Agent respond can be something to say, but it can also be a update of its own behavior code
type AskAgentRes struct {
	AgentID            string `json:"agent,omitempty"` // name of responding agent
	Say                string `json:"say,omitempty"`
	BehaviorCodeUpdate string `json:"behavior-code-update,omitempty"`
}

func askAgent(c *gin.Context) {
	fmt.Println("askAgent")
	agentID := c.Param("id")

	var req AskAgentReq
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	agent, exists := agents[agentID]
	if exists == false {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown agent"})
		return
	}

	resp, err := ollamaClient.Embeddings(context.Background(), &ollama.EmbeddingRequest{
		Model:  "mxbai-embed-large",
		Prompt: req.Prompt,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	embedding := resp.Embedding

	agentMem, err := chromaClient.GetCollection(agent.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	embeddings, err := agentMem.Query(ChromaCollectionQuery{
		Embeddings: [][]float64{
			embedding,
		},
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	memories := ""
	for _, e := range embeddings {
		memories += "- " + e.Document + "\n"
	}

	completeInput := fmt.Sprintf(system_prompt_format, agent.Name, agent.System, memories, req.Sender, req.Prompt)

	fmt.Println("COMPLETE INPUT:\n", completeInput)

	stream := false

	gReq := &ollama.GenerateRequest{
		Model:  "llama3",
		Prompt: completeInput,
		// System: "Always give shortest possible answers, like when chatting on Discord. Use emojis when possible, but not too much.",
		// Template: "",
		Stream: &stream,
		// Format: "",
		// Options: map[string]interface{}
	}

	res := AskAgentRes{
		AgentID:            agent.ID,
		Say:                "",
		BehaviorCodeUpdate: "",
	}

	ollamaClient.Generate(context.Background(), gReq, func(r ollama.GenerateResponse) error {
		fmt.Printf("%s", r.Response)
		res.Say = r.Response
		return nil
	})

	memory := req.Sender + " said: " + req.Prompt + "\nYOUR ANSWER: " + res.Say
	hash := md5.New()
	io.WriteString(hash, memory)
	hashBytes := hash.Sum(nil)
	hashString := hex.EncodeToString(hashBytes)
	// fmt.Println("ID:", hashString)

	resp, err = ollamaClient.Embeddings(context.Background(), &ollama.EmbeddingRequest{
		Model:  "mxbai-embed-large",
		Prompt: memory,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	agentMem.Add([]ChromaCollectionEntry{
		{
			Embedding: &resp.Embedding,
			Document:  memory,
			// Metadatas: map[string]any{"createdAt": 1234},
			ID: hashString,
		},
	})

	c.JSON(http.StatusOK, res)
}

func onAgentResponse(r ollama.GenerateResponse) error {
	fmt.Printf("%s", r.Response)
	// if r.Done {
	// 	fmt.Printf("\n")
	// 	c <- 0
	// }
	return nil
}
