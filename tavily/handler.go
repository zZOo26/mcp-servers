package main

import (
	"github.com/zZOo26/mcp-servers/shared"
)

var tools = []shared.ToolDef{
	{
		Name:        "tavily_web_search",
		Description: "Perform comprehensive web searches with AI-powered content extraction",
		Parameters: shared.ToolParams{
			Type: "object",
			Properties: map[string]shared.PropDef{
				"query":           {Type: "string", Description: "Search query"},
				"max_results":     {Type: "integer", Description: "Maximum number of results", Default: 5},
				"search_depth":    {Type: "string", Description: "Search depth (basic or advanced)", Default: "basic"},
				"include_domains": {Type: "array", Description: "Include only these domains", Items: &shared.PropDef{Type: "string"}},
				"exclude_domains": {Type: "array", Description: "Exclude these domains", Items: &shared.PropDef{Type: "string"}},
			},
			Required: []string{"query"},
		},
	},
	{
		Name:        "tavily_answer_search",
		Description: "Generate direct answers with supporting web evidence",
		Parameters: shared.ToolParams{
			Type: "object",
			Properties: map[string]shared.PropDef{
				"query":        {Type: "string", Description: "Search query"},
				"max_results":  {Type: "integer", Description: "Maximum number of results", Default: 5},
				"search_depth": {Type: "string", Description: "Search depth (basic or advanced)", Default: "basic"},
			},
			Required: []string{"query"},
		},
	},
	{
		Name:        "tavily_news_search",
		Description: "Search recent news articles with publication dates",
		Parameters: shared.ToolParams{
			Type: "object",
			Properties: map[string]shared.PropDef{
				"query":       {Type: "string", Description: "Search query"},
				"max_results": {Type: "integer", Description: "Maximum number of results", Default: 5},
				"days":        {Type: "integer", Description: "Number of days to look back", Default: 7},
			},
			Required: []string{"query"},
		},
	},
}

type tavilyHandler struct {
	client *tavilyClient
}

func newTavilyHandler(apiKey string) *tavilyHandler {
	return &tavilyHandler{client: newTavilyClient(apiKey)}
}

func (h *tavilyHandler) GetTools() []shared.ToolDef {
	return tools
}

func (h *tavilyHandler) Healthy() error {
	return nil
}

func (h *tavilyHandler) CallTool(tool string, arguments map[string]any) shared.ToolResponse {
	switch tool {
	case "tavily_web_search":
		return h.client.webSearch(arguments)
	case "tavily_answer_search":
		return h.client.answerSearch(arguments)
	case "tavily_news_search":
		return h.client.newsSearch(arguments)
	default:
		return shared.ToolResponse{Success: false, Error: "unknown tool: " + tool}
	}
}
