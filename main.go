package main

import (
	"bufio"
	"context"
	"fmt"
	ollama "github.com/ollama/ollama/api"
	"os"
)

const (
	CHROMA_DB_HOST_ADDR = "http://localhost:9999"
)

var (
	c chan int
)

func main() {
	c = make(chan int)

	client, _ := ollama.ClientFromEnvironment()
	client.Heartbeat(context.Background())

	stream := true

	var input string
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Printf("> ")
		if scanner.Scan() {
			input = scanner.Text()
		}

		go func() {
			gReq := &ollama.GenerateRequest{
				Model:  "llama3",
				Prompt: input,
				System: "Always give shortest possible answers, like when chatting on Discord",
				// Template: "",
				Stream: &stream,
				// Format: "",
				// Options: map[string]interface{}
			}

			client.Generate(context.Background(), gReq, onResponse)
		}()

		<-c
	}
}

func onResponse(r ollama.GenerateResponse) error {
	fmt.Printf("%s", r.Response)
	if r.Done {
		fmt.Printf("\n")
		c <- 0
	}
	return nil
}
