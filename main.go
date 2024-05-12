package main

const (
	CHROMA_DB_HOST_ADDR = "http://localhost:9999"
	CHROMA_DB_TENANT    = "npcs"
	CHROMA_DB_DATABASE  = "npcs"
	DEBUG               = true
)

func main() {
	// serveChatCLI()
	serveAPI(":7777")
}
