package shared

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type ToolHandler interface {
	GetTools() []ToolDef
	CallTool(tool string, arguments map[string]any) ToolResponse
	Healthy() error // optional custom check
}

func NewRouter(h ToolHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"message": "MCP HTTP Server", "status": "Running"})
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := h.Healthy(); err != nil {
			log.Printf("health check failed: %v", err)
			http.Error(w, `{"status":"unhealthy"}`, http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]string{"status": "healthy"})
	})

	r.Get("/tools", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"tools": h.GetTools()})
	})

	r.Post("/tools/call", func(w http.ResponseWriter, r *http.Request) {
		var req ToolRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, ToolResponse{Success: false, Error: "invalid request body"})
			return
		}
		writeJSON(w, h.CallTool(req.Tool, req.Arguments))
	})

	return r
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
