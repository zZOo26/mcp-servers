package main

import (
	"github.com/zZOo26/mcp-servers/shared"
)

var tools = []shared.ToolDef{
	{
		Name:        "google_search",
		Description: "Perform Google searches via Serper API with advanced filtering including date range, domain filtering and search operators",
		Parameters: shared.ToolParams{
			Type: "object",
			Properties: map[string]shared.PropDef{
				"query":       {Type: "string", Description: "Search query"},
				"type":        {Type: "string", Description: "Search type (search, images, news, places, videos)", Default: "search"},
				"site":        {Type: "string", Description: "Limit results to specific domain (e.g., 'mmu.edu.my')"},
				"after":       {Type: "string", Description: "Only results after this date in YYYY-MM-DD format"},
				"before":      {Type: "string", Description: "Only results before this date in YYYY-MM-DD format"},
				"filetype":    {Type: "string", Description: "Limit to specific file types (e.g., 'pdf', 'doc')"},
				"max_results": {Type: "integer", Description: "Maximum number of results", Default: 10},
				"location":    {Type: "string", Description: "Location for localized results"},
				"gl":          {Type: "string", Description: "Country code (e.g., 'my' for Malaysia)"},
				"hl":          {Type: "string", Description: "Language code (e.g., 'en' for English)"},
			},
			Required: []string{"query"},
		},
	},
	{
		Name:        "scrape",
		Description: "Extract content from web pages",
		Parameters: shared.ToolParams{
			Type: "object",
			Properties: map[string]shared.PropDef{
				"url": {Type: "string", Description: "URL to scrape"},
			},
			Required: []string{"url"},
		},
	},
}

type serperHandler struct {
	client *serperClient
}

func newSerperHandler(apiKey string) *serperHandler {
	return &serperHandler{client: newSerperClient(apiKey)}
}

func (h *serperHandler) GetTools() []shared.ToolDef {
	return tools
}

func (h *serperHandler) Healthy() error {
	return nil
}

func (h *serperHandler) CallTool(tool string, arguments map[string]any) shared.ToolResponse {
	switch tool {
	case "google_search":
		return h.client.googleSearch(arguments)
	case "scrape":
		return h.client.scrape(arguments)
	default:
		return shared.ToolResponse{Success: false, Error: "unknown tool: " + tool}
	}
}
