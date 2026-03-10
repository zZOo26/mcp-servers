package main

import (
	"log"

	"github.com/zZOo26/mcp-servers/shared"
)

var tools = []shared.ToolDef{
	{
		Name:        "get_transit_directions",
		Description: "Get multi-transfer public transport directions for MRT/LRT/RapidKL buses in Malaysia using OpenTripPlanner. Coverage: MRT lines, LRT lines, RapidKL buses, MRT feeders. NOT included: KLIA Ekspres, ETS intercity trains, specialized services. If this tool returns 'no route found', use search_web to find alternative transit options. Supports GPS coordinates for automatic 'from current location' routing.",
		Parameters: shared.ToolParams{
			Type: "object",
			Properties: map[string]shared.PropDef{
				"from_station": {Type: "string", Description: "Starting location/station name (e.g., 'Bandar Tun Hussein Onn', 'TRX'). Omit if from_lat/from_lon provided."},
				"to_station":   {Type: "string", Description: "Destination location/station name (e.g., 'TRX', 'KL Sentral', 'Jaya One')"},
				"from_lat":     {Type: "number", Description: "Starting latitude (GPS). Use with from_lon for 'from current location'."},
				"from_lon":     {Type: "number", Description: "Starting longitude (GPS). Use with from_lat for 'from current location'."},
				"include_map":  {Type: "boolean", Description: "If true, returns route geometry for map visualization. Default: false."},
			},
			Required: []string{"to_station"},
		},
	},
}

type gtfsHandler struct {
	client *gtfsClient
}

func newGTFSHandler(otp2URL string) *gtfsHandler {
	return &gtfsHandler{client: newGTFSClient(otp2URL)}
}

func (h *gtfsHandler) GetTools() []shared.ToolDef {
	return tools
}

func (h *gtfsHandler) Healthy() error {
	// Match Python behavior: always return healthy even if OTP2 is unreachable
	// OTP2 connectivity is optional — server stays up to report the status
	if err := h.client.healthCheck(); err != nil {
		log.Printf("OTP2 not reachable: %v", err)
	}
	return nil
}

func (h *gtfsHandler) CallTool(tool string, arguments map[string]any) shared.ToolResponse {
	switch tool {
	case "get_transit_directions":
		return h.client.getTransitDirections(arguments)
	default:
		return shared.ToolResponse{Success: false, Error: "unknown tool: " + tool}
	}
}
