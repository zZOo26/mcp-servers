package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/zZOo26/mcp-servers/shared"
)

func main() {
	_ = godotenv.Load()

	apiKey := os.Getenv("SERPER_API_KEY")
	if apiKey == "" {
		log.Fatal("SERPER_API_KEY is required")
	}

	port := os.Getenv("SERPER_PORT")
	if port == "" {
		port = "9006"
	}

	h := newSerperHandler(apiKey)
	r := shared.NewRouter(h)

	log.Printf("Serper MCP server listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
