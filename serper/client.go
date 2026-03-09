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

const serperAPIURL = "https://google.serper.dev"

type serperClient struct {
	apiKey     string
	httpClient *http.Client
}

func newSerperClient(apiKey string) *serperClient {
	return &serperClient{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *serperClient) googleSearch(args map[string]any) shared.ToolResponse {
	query, _ := args["query"].(string)
	if query == "" {
		return shared.ToolResponse{Success: false, Error: "query is required"}
	}

	searchType, _ := args["type"].(string)
	if searchType == "" {
		searchType = "search"
	}

	maxResults := 10
	if v, ok := args["max_results"]; ok {
		switch n := v.(type) {
		case float64:
			maxResults = int(n)
		case int:
			maxResults = n
		}
	}

	// Build query with operators
	searchQuery := query

	if site, _ := args["site"].(string); site != "" {
		op := "site:" + site
		if !strings.Contains(strings.ToLower(searchQuery), strings.ToLower(op)) {
			searchQuery = op + " " + searchQuery
			log.Printf("SERPER: added site restriction: %s", site)
		}
	}

	if filetype, _ := args["filetype"].(string); filetype != "" {
		searchQuery = "filetype:" + filetype + " " + searchQuery
	}

	payload := map[string]any{
		"q":   searchQuery,
		"num": maxResults,
	}

	// Date filter: after -> tbs=qdr:dN
	if after, _ := args["after"].(string); after != "" {
		afterDate, err := time.Parse("2006-01-02", after)
		if err == nil {
			daysAgo := int(time.Since(afterDate).Hours() / 24)
			payload["tbs"] = fmt.Sprintf("qdr:d%d", daysAgo)
			log.Printf("SERPER: date filter after=%s (%d days ago)", after, daysAgo)
		}
	}

	// Note: "before" has no native Serper support — no-op

	if location, _ := args["location"].(string); location != "" {
		payload["location"] = location
	}
	if gl, _ := args["gl"].(string); gl != "" {
		payload["gl"] = gl
	}
	if hl, _ := args["hl"].(string); hl != "" {
		payload["hl"] = hl
	}

	endpoint := serperAPIURL + "/" + searchType
	log.Printf("SERPER: calling %s with query: %s", endpoint, searchQuery)

	body, err := json.Marshal(payload)
	if err != nil {
		return shared.ToolResponse{Success: false, Error: err.Error()}
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return shared.ToolResponse{Success: false, Error: err.Error()}
	}
	req.Header.Set("X-API-KEY", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return shared.ToolResponse{Success: false, Error: err.Error()}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return shared.ToolResponse{Success: false, Error: err.Error()}
	}

	if resp.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("Serper API error: %d - %s", resp.StatusCode, string(respBody))
		log.Printf("SERPER: %s", msg)
		return shared.ToolResponse{Success: false, Error: msg}
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return shared.ToolResponse{Success: false, Error: err.Error()}
	}

	formatted := formatSerperResponse(result)

	return shared.ToolResponse{
		Success: true,
		Result: map[string]any{
			"content":     formatted,
			"raw_results": result,
			"query":       searchQuery,
		},
	}
}

func (c *serperClient) scrape(args map[string]any) shared.ToolResponse {
	url, _ := args["url"].(string)
	if url == "" {
		return shared.ToolResponse{Success: false, Error: "url is required"}
	}

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return shared.ToolResponse{Success: false, Error: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return shared.ToolResponse{Success: false, Error: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10_000))
	if err != nil {
		return shared.ToolResponse{Success: false, Error: err.Error()}
	}

	return shared.ToolResponse{
		Success: true,
		Result: map[string]any{
			"content":     string(body),
			"url":         url,
			"status_code": resp.StatusCode,
		},
	}
}

func formatSerperResponse(result map[string]any) string {
	var parts []string

	// Knowledge graph first
	if kg, ok := result["knowledgeGraph"].(map[string]any); ok {
		title, _ := kg["title"].(string)
		desc, _ := kg["description"].(string)
		if title != "" && desc != "" {
			parts = append(parts, fmt.Sprintf("**%s**\n%s", title, desc))
		}
	}

	// Organic results (top 5)
	if organic, ok := result["organic"].([]any); ok {
		for i, item := range organic {
			if i >= 5 {
				break
			}
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			title, _ := m["title"].(string)
			snippet, _ := m["snippet"].(string)
			link, _ := m["link"].(string)
			date, _ := m["date"].(string)

			entry := fmt.Sprintf("%d. **%s**", i+1, title)
			if date != "" {
				entry += fmt.Sprintf(" (%s)", date)
			}
			entry += "\n" + snippet
			if link != "" {
				entry += "\nSource: " + link
			}
			parts = append(parts, entry)
		}
	}

	// News results (top 3)
	if news, ok := result["news"].([]any); ok {
		for i, item := range news {
			if i >= 3 {
				break
			}
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			title, _ := m["title"].(string)
			snippet, _ := m["snippet"].(string)
			link, _ := m["link"].(string)
			date, _ := m["date"].(string)

			entry := fmt.Sprintf("📰 %d. **%s**", i+1, title)
			if date != "" {
				entry += fmt.Sprintf(" (%s)", date)
			}
			entry += "\n" + snippet
			if link != "" {
				entry += "\nSource: " + link
			}
			parts = append(parts, entry)
		}
	}

	if len(parts) == 0 {
		return "No results found."
	}
	return strings.Join(parts, "\n\n---\n\n")
}
