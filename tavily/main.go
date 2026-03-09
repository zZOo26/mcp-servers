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

	apiKey := os.Getenv("TAVILY_API_KEY")
	if apiKey == "" {
		log.Fatal("TAVILY_API_KEY is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "9003"
	}

	h := newTavilyHandler(apiKey)
	r := shared.NewRouter(h)

	log.Printf("Tavily MCP server listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
