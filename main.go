package main

import (
	"bufio"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	ollama "github.com/ollama/ollama/api"
	"io"
	"os"
	"strings"
)

const (
	CHROMA_DB_HOST_ADDR = "http://localhost:9999"
	CHROMA_DB_TENANT    = "npcs"
	CHROMA_DB_DATABASE  = "npcs"
	DEBUG               = false
)

type RequestType int

const (
	RequestType_Store RequestType = iota
	RequestType_Question
	RequestType_Unknown
)

var (
	c           chan int
	requestType chan RequestType
)

func main() {
	chromaClient, err := NewChromaClient(CHROMA_DB_HOST_ADDR, CHROMA_DB_TENANT, CHROMA_DB_DATABASE)
	if err != nil {
		fmt.Println("❌", err.Error())
		return
	}

	err = chromaClient.Check()
	if err != nil {
		fmt.Println("❌", err.Error())
		return
	}

	memories, err := chromaClient.GetCollection("memories")
	if err != nil {
		fmt.Println("❌", err.Error())
		return
	}

	c = make(chan int)
	requestType = make(chan RequestType)

	client, _ := ollama.ClientFromEnvironment()
	client.Heartbeat(context.Background())

	fmt.Println("COMMANDS: /store, /ask")

	mode := "ask"
	fmt.Println("MODE:", mode)

	stream := true
	// noStream := false

	var input string
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Printf("> ")
		if scanner.Scan() {
			input = scanner.Text()
		}

		if strings.TrimSpace(input) == "/store" {
			mode = "store"
			fmt.Println("MODE:", mode)
			continue
		} else if strings.TrimSpace(input) == "/ask" {
			mode = "ask"
			fmt.Println("MODE:", mode)
			continue
		}

		req := &ollama.EmbeddingRequest{
			Model:  "mxbai-embed-large",
			Prompt: input,
			// KeepAlive
			// Options
		}

		resp, err := client.Embeddings(context.Background(), req)
		if err != nil {
			fmt.Println("❌", err.Error())
			continue
		}
		embedding := resp.Embedding
		// fmt.Println(embedding)

		if mode == "ask" {
			go func() {
				embeddings, err := memories.Query(ChromaCollectionQuery{
					Embeddings: [][]float64{
						embedding,
					},
				})
				if err != nil {
					fmt.Println("❌", err.Error())
					c <- 0
					return
				}

				// fmt.Println(embeddings)
				/*
									type ChromaCollectionEntry struct {
						Embedding *[]float64     `json:"embedding,omitempty"`
						Document  string         `json:"document,omitempty"`
						Metadatas map[string]any `json:"metadatas,omitempty"`
						ID        string         `json:"id,omitempty"`
						Distance  float64        `json:"distance,omitempty"`
					}
				*/

				completeInput := "Using this data (provided by the user):\n"
				for _, e := range embeddings {
					completeInput += "- " + e.Document + "\n"
				}

				completeInput += "\nRespond to this prompt:\n" + input

				// fmt.Println("COMPLETE input:", completeInput)

				gReq := &ollama.GenerateRequest{
					Model:  "llama3",
					Prompt: completeInput,
					System: "Always give shortest possible answers, like when chatting on Discord. Use emojis when possible, but not too much.",
					// Template: "",
					Stream: &stream,
					// Format: "",
					// Options: map[string]interface{}
				}
				client.Generate(context.Background(), gReq, onResponse)
			}()
			<-c
		} else if mode == "store" {
			hash := md5.New()
			io.WriteString(hash, input)
			hashBytes := hash.Sum(nil)
			hashString := hex.EncodeToString(hashBytes)
			fmt.Println("ID:", hashString)

			memories.Add([]ChromaCollectionEntry{
				{
					Embedding: &resp.Embedding,
					Document:  input,
					// Metadatas: map[string]any{"createdAt": 1234},
					ID: hashString,
				},
			})
		}

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

func onEmbeddingResponse(r ollama.GenerateResponse) error {
	fmt.Println(r.Response)
	c <- 0
	return nil
}

func checkChromaDB() error {
	fmt.Println("CHECKING CHROMA DB")
	return nil
}
