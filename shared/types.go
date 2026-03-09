package shared

type ToolDef struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Parameters  ToolParams `json:"parameters,omitempty"`
}

type ToolParams struct {
	Type       string             `json:"type"`
	Properties map[string]PropDef `json:"properties"`
	Required   []string           `json:"required,omitempty"`
}

type PropDef struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Default     any      `json:"default,omitempty"`
	Items       *PropDef `json:"items,omitempty"`
}

type ToolRequest struct {
	Tool      string         `json:"tool"`
	Arguments map[string]any `json:"arguments"`
}

type ToolResponse struct {
	Success bool   `json:"success"`
	Result  any    `json:"result,omitempty"`
	Error   string `json:"error,omitempty"`
}
