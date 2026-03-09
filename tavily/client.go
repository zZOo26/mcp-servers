package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/zZOo26/mcp-servers/shared"
)

const tavilyAPIURL = "https://api.tavily.com/search"

type tavilyClient struct {
	apiKey     string
	httpClient *http.Client
}

func newTavilyClient(apiKey string) *tavilyClient {
	return &tavilyClient{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *tavilyClient) search(payload map[string]any) (map[string]any, error) {
	payload["api_key"] = c.apiKey

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, tavilyAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Tavily API error: %d - %s", resp.StatusCode, string(respBody))
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *tavilyClient) webSearch(args map[string]any) shared.ToolResponse {
	query, _ := args["query"].(string)
	if query == "" {
		return shared.ToolResponse{Success: false, Error: "query is required"}
	}

	maxResults := 5
	if v, ok := args["max_results"]; ok {
		if n, ok := v.(float64); ok {
			maxResults = int(n)
		}
	}

	searchDepth, _ := args["search_depth"].(string)
	if searchDepth == "" {
		searchDepth = "basic"
	}

	payload := map[string]any{
		"query":        query,
		"max_results":  maxResults,
		"search_depth": searchDepth,
		"topic":        "general",
	}

	if v, ok := args["include_domains"]; ok {
		payload["include_domains"] = v
	}
	if v, ok := args["exclude_domains"]; ok {
		payload["exclude_domains"] = v
	}
	if v, ok := args["days"]; ok {
		payload["days"] = v
	}

	log.Printf("TAVILY: web search query=%s depth=%s", query, searchDepth)

	result, err := c.search(payload)
	if err != nil {
		return shared.ToolResponse{Success: false, Error: err.Error()}
	}

	var parts []string
	if results, ok := result["results"].([]any); ok {
		for _, item := range results {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			title, _ := m["title"].(string)
			content, _ := m["content"].(string)
			if title != "" && content != "" {
				parts = append(parts, fmt.Sprintf("**%s**\n%s", title, content))
			}
		}
	}

	return shared.ToolResponse{
		Success: true,
		Result: map[string]any{
			"content": strings.Join(parts, "\n\n"),
			"results": result["results"],
			"query":   query,
		},
	}
}

func (c *tavilyClient) answerSearch(args map[string]any) shared.ToolResponse {
	query, _ := args["query"].(string)
	if query == "" {
		return shared.ToolResponse{Success: false, Error: "query is required"}
	}

	maxResults := 5
	if v, ok := args["max_results"]; ok {
		if n, ok := v.(float64); ok {
			maxResults = int(n)
		}
	}

	searchDepth, _ := args["search_depth"].(string)
	if searchDepth == "" {
		searchDepth = "basic"
	}

	payload := map[string]any{
		"query":        query,
		"max_results":  maxResults,
		"search_depth": searchDepth,
		"topic":        "general",
	}

	log.Printf("TAVILY: answer search query=%s", query)

	result, err := c.search(payload)
	if err != nil {
		return shared.ToolResponse{Success: false, Error: err.Error()}
	}

	answer, _ := result["answer"].(string)
	if answer == "" {
		if results, ok := result["results"].([]any); ok && len(results) > 0 {
			if m, ok := results[0].(map[string]any); ok {
				answer, _ = m["content"].(string)
			}
		}
	}

	return shared.ToolResponse{
		Success: true,
		Result: map[string]any{
			"answer":  answer,
			"results": result["results"],
			"query":   query,
		},
	}
}

func (c *tavilyClient) newsSearch(args map[string]any) shared.ToolResponse {
	query, _ := args["query"].(string)
	if query == "" {
		return shared.ToolResponse{Success: false, Error: "query is required"}
	}

	maxResults := 5
	if v, ok := args["max_results"]; ok {
		if n, ok := v.(float64); ok {
			maxResults = int(n)
		}
	}

	days := 7
	if v, ok := args["days"]; ok {
		if n, ok := v.(float64); ok {
			days = int(n)
		}
	}

	payload := map[string]any{
		"query":        query,
		"max_results":  maxResults,
		"search_depth": "basic",
		"topic":        "news",
		"days":         days,
	}

	log.Printf("TAVILY: news search query=%s days=%d", query, days)

	result, err := c.search(payload)
	if err != nil {
		return shared.ToolResponse{Success: false, Error: err.Error()}
	}

	var parts []string
	if results, ok := result["results"].([]any); ok {
		for _, item := range results {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			title, _ := m["title"].(string)
			content, _ := m["content"].(string)
			url, _ := m["url"].(string)
			date, _ := m["published_date"].(string)

			if title == "" || content == "" {
				continue
			}

			entry := fmt.Sprintf("**%s**", title)
			if date != "" {
				entry += fmt.Sprintf(" (%s)", date)
			}
			entry += "\n" + content
			if url != "" {
				entry += "\nSource: " + url
			}
			parts = append(parts, entry)
		}
	}

	return shared.ToolResponse{
		Success: true,
		Result: map[string]any{
			"content": strings.Join(parts, "\n\n---\n\n"),
			"results": result["results"],
			"query":   query,
		},
	}
}
