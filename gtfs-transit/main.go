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

	otp2URL := os.Getenv("OTP2_URL")
	if otp2URL == "" {
		otp2URL = "http://otp2-server:8080"
	}

	port := os.Getenv("GTFS_PORT")
	if port == "" {
		port = "9007"
	}

	h := newGTFSHandler(otp2URL)
	r := shared.NewRouter(h)

	log.Printf("GTFS Transit MCP server listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
