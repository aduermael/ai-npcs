package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
)

func serveAPI(port string) {
	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()
	router.POST("/agents", createAgent)

	fmt.Println("Serving API... (" + port + ")")
	router.Run(port)
}

type CreateAgentReq struct {
	System       string `json:"system,omitempty"` // system prompt for the agent
	Name         string `json:"name,omitempty"`
	IsNPC        bool   `json:"is-npc,omitempty"`
	BehaviorCode string `json:"bahavior-code,omitempty"` // initial NPC behavior code
}

func createAgent(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Here is some data.",
	})
}
