package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/zZOo26/mcp-servers/shared"
)

type gtfsClient struct {
	otp2URL    string
	graphqlURL string
	httpClient *http.Client
	cache      sync.Map
}

func newGTFSClient(otp2URL string) *gtfsClient {
	return &gtfsClient{
		otp2URL:    otp2URL,
		graphqlURL: otp2URL + "/otp/routers/default/index/graphql",
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *gtfsClient) healthCheck() error {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(c.otp2URL + "/")
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("OTP2 returned %d", resp.StatusCode)
	}
	return nil
}

func (c *gtfsClient) graphql(query string, variables map[string]any) (map[string]any, error) {
	body, _ := json.Marshal(map[string]any{"query": query, "variables": variables})
	req, err := http.NewRequest(http.MethodPost, c.graphqlURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *gtfsClient) geocodeNominatim(locationName string) (map[string]float64, error) {
	variations := []string{locationName}

	// Remove generic suffixes
	clean := locationName
	for _, suffix := range []string{" Shopping Centre", " Shopping Center", " Mall", " shopping mall"} {
		if strings.Contains(strings.ToLower(locationName), strings.ToLower(suffix)) {
			trimmed := strings.TrimSpace(locationName[:strings.Index(strings.ToLower(locationName), strings.ToLower(suffix))])
			if trimmed != "" && trimmed != locationName {
				clean = trimmed
				variations = append(variations, clean)
				break
			}
		}
	}

	// "One X" → "1 X"
	if strings.HasPrefix(strings.ToLower(clean), "one ") {
		numeric := "1" + clean[3:]
		variations = append(variations, numeric)
	}

	log.Printf("Trying Nominatim variations: %v", variations)

	nominatimClient := &http.Client{Timeout: 10 * time.Second}
	for _, q := range variations {
		params := url.Values{
			"q":              {q + ", Malaysia"},
			"format":         {"json"},
			"limit":          {"5"},
			"countrycodes":   {"my"},
			"addressdetails": {"1"},
		}
		req, _ := http.NewRequest(http.MethodGet, "https://nominatim.openstreetmap.org/search?"+params.Encode(), nil)
		req.Header.Set("User-Agent", "OTP2-Transit-MCP/2.0")

		resp, err := nominatimClient.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		data, _ := io.ReadAll(resp.Body)
		var results []map[string]any
		if err := json.Unmarshal(data, &results); err != nil || len(results) == 0 {
			continue
		}

		// Prioritize POIs
		poiTypes := map[string]bool{"building": true, "retail": true, "commercial": true, "shopping_centre": true, "hotel": true, "mall": true}
		poiClasses := map[string]bool{"building": true, "shop": true, "amenity": true, "tourism": true}

		for _, r := range results {
			rType := fmt.Sprintf("%v", r["type"])
			rClass := fmt.Sprintf("%v", r["class"])
			if poiTypes[rType] || poiClasses[rClass] {
				lat := parseFloat(r["lat"])
				lon := parseFloat(r["lon"])
				log.Printf("Nominatim POI found '%s' → %v (%s) at (%f, %f)", q, r["display_name"], rType, lat, lon)
				return map[string]float64{"lat": lat, "lon": lon}, nil
			}
		}

		// Fallback to first result
		lat := parseFloat(results[0]["lat"])
		lon := parseFloat(results[0]["lon"])
		log.Printf("Nominatim found '%s' → %v at (%f, %f)", q, results[0]["display_name"], lat, lon)
		return map[string]float64{"lat": lat, "lon": lon}, nil
	}

	return nil, fmt.Errorf("no Nominatim results for '%s'", locationName)
}

func (c *gtfsClient) findLocationCoords(locationName string) (map[string]float64, error) {
	cacheKey := strings.ToLower(strings.TrimSpace(locationName))
	if val, ok := c.cache.Load(cacheKey); ok {
		log.Printf("Cache hit for '%s'", locationName)
		return val.(map[string]float64), nil
	}

	stopQuery := `query($name: String!) { stops(name: $name) { gtfsId name lat lon } }`

	result, err := c.graphql(stopQuery, map[string]any{"name": locationName})
	if err != nil {
		log.Printf("OTP2 stop search error for '%s': %v", locationName, err)
	} else {
		stops := getStops(result)
		if len(stops) > 0 {
			coords := stopCoords(stops[0])
			c.cache.Store(cacheKey, coords)
			log.Printf("Found transit stop: %v at (%f, %f)", stops[0]["name"], coords["lat"], coords["lon"])
			return coords, nil
		}

		// Fuzzy: strip station/mode suffixes
		locationLower := strings.ToLower(locationName)
		userWantsMRT := strings.Contains(locationLower, "mrt")
		userWantsLRT := strings.Contains(locationLower, "lrt")

		cleanName := locationName
		for _, suffix := range []string{" Station", " station", " LRT", " MRT", " lrt", " mrt"} {
			cleanName = strings.ReplaceAll(cleanName, suffix, "")
		}
		cleanName = strings.TrimSpace(cleanName)

		if cleanName != locationName {
			result2, err2 := c.graphql(stopQuery, map[string]any{"name": cleanName})
			if err2 == nil {
				stops2 := getStops(result2)
				if len(stops2) > 0 {
					var best map[string]any
					if userWantsMRT || userWantsLRT {
						for _, s := range stops2 {
							name := strings.ToUpper(fmt.Sprintf("%v", s["name"]))
							gtfsID := fmt.Sprintf("%v", s["gtfsId"])
							if strings.Contains(name, "TERMINAL") || strings.Contains(name, "BUS") {
								continue
							}
							if strings.HasPrefix(gtfsID, "2:") {
								best = s
								break
							}
						}
					}
					if best == nil {
						for _, s := range stops2 {
							if !strings.Contains(strings.ToUpper(fmt.Sprintf("%v", s["name"])), "TERMINAL") {
								best = s
								break
							}
						}
					}
					if best == nil {
						best = stops2[0]
					}
					coords := stopCoords(best)
					c.cache.Store(cacheKey, coords)
					log.Printf("Fuzzy matched '%s' → '%v' at (%f, %f)", locationName, best["name"], coords["lat"], coords["lon"])
					return coords, nil
				}
			}
		}
	}

	// Fallback to Nominatim
	log.Printf("Transit stop not found for '%s', trying Nominatim...", locationName)
	coords, err := c.geocodeNominatim(locationName)
	if err != nil {
		return nil, err
	}
	c.cache.Store(cacheKey, coords)
	return coords, nil
}

func (c *gtfsClient) getOTP2Route(fromCoords, toCoords map[string]float64, numItineraries int, includeGeometry bool) (map[string]any, error) {
	geometryField := ""
	if includeGeometry {
		geometryField = "\n            legGeometry { length points }"
	}

	query := fmt.Sprintf(`
	query($fromLat: Float!, $fromLon: Float!, $toLat: Float!, $toLon: Float!, $numItineraries: Int!) {
	  plan(
	    from: {lat: $fromLat, lon: $fromLon}
	    to: {lat: $toLat, lon: $toLon}
	    numItineraries: $numItineraries
	    transportModes: [{mode: TRANSIT}, {mode: WALK}]
	    walkReluctance: 2.5
	    maxWalkDistance: 750
	    walkSpeed: 1.1
	  ) {
	    itineraries {
	      duration
	      walkDistance
	      numberOfTransfers
	      legs {
	        mode
	        from { name lat lon }
	        to { name lat lon }
	        distance
	        duration
	        route { shortName longName }%s
	      }
	    }
	  }
	}`, geometryField)

	return c.graphql(query, map[string]any{
		"fromLat":        fromCoords["lat"],
		"fromLon":        fromCoords["lon"],
		"toLat":          toCoords["lat"],
		"toLon":          toCoords["lon"],
		"numItineraries": numItineraries,
	})
}

func (c *gtfsClient) getTransitDirections(args map[string]any) shared.ToolResponse {
	toStation, _ := args["to_station"].(string)
	if toStation == "" {
		return shared.ToolResponse{Success: false, Error: "to_station is required"}
	}

	fromStation, _ := args["from_station"].(string)
	fromLat, hasFromLat := args["from_lat"]
	fromLon, hasFromLon := args["from_lon"]
	includeMap, _ := args["include_map"].(bool)

	var fromCoords map[string]float64
	fromDisplayName := fromStation

	if hasFromLat && hasFromLon {
		fromCoords = map[string]float64{
			"lat": parseFloat(fromLat),
			"lon": parseFloat(fromLon),
		}
		fromDisplayName = "Your Current Location"
		log.Printf("Using GPS coordinates for start: %f, %f", fromCoords["lat"], fromCoords["lon"])
	} else if fromStation != "" {
		var err error
		fromCoords, err = c.findLocationCoords(fromStation)
		if err != nil {
			return shared.ToolResponse{Success: false, Error: fmt.Sprintf("Could not find location: %s. Please provide a known station or landmark name.", fromStation)}
		}
	} else {
		return shared.ToolResponse{Success: false, Error: "Either from_station or both from_lat/from_lon are required"}
	}

	toCoords, err := c.findLocationCoords(toStation)
	if err != nil {
		return shared.ToolResponse{Success: false, Error: fmt.Sprintf("Could not find location: %s. Please provide a known station or landmark name.", toStation)}
	}

	log.Printf("Routing: %s (%.4f, %.4f) → %s (%.4f, %.4f)",
		fromDisplayName, fromCoords["lat"], fromCoords["lon"],
		toStation, toCoords["lat"], toCoords["lon"])

	otpResult, err := c.getOTP2Route(fromCoords, toCoords, 5, includeMap)
	if err != nil {
		return shared.ToolResponse{Success: false, Error: "OTP2 request failed: " + err.Error()}
	}

	if errs, ok := otpResult["errors"].([]any); ok && len(errs) > 0 {
		errMsg := "unknown error"
		if e, ok := errs[0].(map[string]any); ok {
			errMsg = fmt.Sprintf("%v", e["message"])
		}
		return shared.ToolResponse{Success: false, Error: "OTP2 error: " + errMsg}
	}

	plan, _ := otpResult["data"].(map[string]any)["plan"].(map[string]any)
	itinerariesRaw, _ := plan["itineraries"].([]any)

	if len(itinerariesRaw) == 0 {
		return shared.ToolResponse{
			Success: false,
			Error:   fmt.Sprintf("No route found in GTFS data (MRT/LRT/buses only). This route may require KLIA Ekspres, ETS trains, or other services not in GTFS. RECOMMENDATION: Use search_web tool to find '%s to %s public transport Malaysia' for complete transit options.", fromStation, toStation),
		}
	}

	// Sort by duration (fastest first)
	itineraries := make([]map[string]any, 0, len(itinerariesRaw))
	for _, it := range itinerariesRaw {
		if m, ok := it.(map[string]any); ok {
			itineraries = append(itineraries, m)
		}
	}
	sortByDuration(itineraries)

	// Deduplicate routes
	uniqueRoutes := []map[string]any{itineraries[0]}
	for _, it := range itineraries[1:] {
		isDifferent := true
		for _, existing := range uniqueRoutes {
			durationDiff := absDiff(getFloat(it, "duration"), getFloat(existing, "duration")) / getFloat(existing, "duration")
			walkDiff := absDiff(getFloat(it, "walkDistance"), getFloat(existing, "walkDistance"))
			if durationDiff < 0.05 && walkDiff < 500 {
				isDifferent = false
				break
			}
		}
		if isDifferent {
			uniqueRoutes = append(uniqueRoutes, it)
			if len(uniqueRoutes) >= 3 {
				break
			}
		}
	}

	best := uniqueRoutes[0]
	duration := formatDuration(getFloat(best, "duration"))
	walkDist := formatDistance(getFloat(best, "walkDistance"))
	transfers := int(getFloat(best, "numberOfTransfers"))

	log.Printf("Returning fastest route: %s, %s walking, %d transfers", duration, walkDist, transfers)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🚇 **%s → %s**\n\n", fromDisplayName, toStation))
	sb.WriteString(fmt.Sprintf("⏱️ **%s** | 🚶 %s walking | 🔄 %d transfer(s)\n\n", duration, walkDist, transfers))

	legs, _ := best["legs"].([]any)
	for _, legRaw := range legs {
		leg, ok := legRaw.(map[string]any)
		if !ok {
			continue
		}
		mode := fmt.Sprintf("%v", leg["mode"])
		fromName := getNestedString(leg, "from", "name")
		toName := getNestedString(leg, "to", "name")
		legDuration := formatDuration(getFloat(leg, "duration"))
		dist := formatDistance(getFloat(leg, "distance"))

		switch mode {
		case "WALK":
			sb.WriteString(fmt.Sprintf("🚶 Walk %s (%s)\n", dist, legDuration))
		case "BUS", "SUBWAY", "RAIL", "TRAM":
			route, _ := leg["route"].(map[string]any)
			routeName := fmt.Sprintf("%v", route["shortName"])
			if routeName == "" || routeName == "<nil>" {
				routeName = fmt.Sprintf("%v", route["longName"])
			}
			if routeName == "" || routeName == "<nil>" {
				routeName = "Transit"
			}
			emoji := "🚇"
			if mode == "BUS" {
				emoji = "🚌"
			}
			sb.WriteString(fmt.Sprintf("%s **%s**: %s → %s (%s)\n", emoji, routeName, fromName, toName, legDuration))
		default:
			sb.WriteString(fmt.Sprintf("%s: %s → %s (%s)\n", mode, fromName, toName, legDuration))
		}
	}
	sb.WriteString("\n\n_⚡ This is the optimal public transport route (fastest with least walking)._")

	textResult := sb.String()

	if includeMap {
		var polylines []map[string]any
		for _, legRaw := range legs {
			leg, ok := legRaw.(map[string]any)
			if !ok {
				continue
			}
			geom, ok := leg["legGeometry"].(map[string]any)
			if !ok || geom == nil {
				continue
			}
			polylines = append(polylines, map[string]any{
				"mode":     leg["mode"],
				"polyline": geom["points"],
				"from": map[string]any{
					"lat":  getNestedFloat(leg, "from", "lat"),
					"lon":  getNestedFloat(leg, "from", "lon"),
					"name": getNestedString(leg, "from", "name"),
				},
				"to": map[string]any{
					"lat":  getNestedFloat(leg, "to", "lat"),
					"lon":  getNestedFloat(leg, "to", "lon"),
					"name": getNestedString(leg, "to", "name"),
				},
			})
		}
		return shared.ToolResponse{
			Success: true,
			Result: map[string]any{
				"text": textResult,
				"map_data": map[string]any{
					"route_polylines": polylines,
					"bounds": map[string]any{
						"from": map[string]any{"lat": fromCoords["lat"], "lon": fromCoords["lon"]},
						"to":   map[string]any{"lat": toCoords["lat"], "lon": toCoords["lon"]},
					},
				},
			},
		}
	}

	return shared.ToolResponse{Success: true, Result: textResult}
}

// --- helpers ---

func parseFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case string:
		var f float64
		fmt.Sscanf(x, "%f", &f)
		return f
	}
	return 0
}

func getFloat(m map[string]any, key string) float64 {
	return parseFloat(m[key])
}

func getNestedString(m map[string]any, keys ...string) string {
	cur := m
	for i, k := range keys {
		if i == len(keys)-1 {
			return fmt.Sprintf("%v", cur[k])
		}
		next, ok := cur[k].(map[string]any)
		if !ok {
			return ""
		}
		cur = next
	}
	return ""
}

func getNestedFloat(m map[string]any, keys ...string) float64 {
	cur := m
	for i, k := range keys {
		if i == len(keys)-1 {
			return parseFloat(cur[k])
		}
		next, ok := cur[k].(map[string]any)
		if !ok {
			return 0
		}
		cur = next
	}
	return 0
}

func getStops(result map[string]any) []map[string]any {
	data, _ := result["data"].(map[string]any)
	stopsRaw, _ := data["stops"].([]any)
	stops := make([]map[string]any, 0, len(stopsRaw))
	for _, s := range stopsRaw {
		if m, ok := s.(map[string]any); ok {
			stops = append(stops, m)
		}
	}
	return stops
}

func stopCoords(stop map[string]any) map[string]float64 {
	return map[string]float64{
		"lat": parseFloat(stop["lat"]),
		"lon": parseFloat(stop["lon"]),
	}
}

func sortByDuration(its []map[string]any) {
	for i := 1; i < len(its); i++ {
		for j := i; j > 0 && getFloat(its[j], "duration") < getFloat(its[j-1], "duration"); j-- {
			its[j], its[j-1] = its[j-1], its[j]
		}
	}
}

func absDiff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}

func formatDuration(seconds float64) string {
	minutes := int(seconds / 60)
	hours := minutes / 60
	mins := minutes % 60
	if hours > 0 {
		return fmt.Sprintf("%dh %dmin", hours, mins)
	}
	return fmt.Sprintf("%d min", mins)
}

func formatDistance(meters float64) string {
	if meters >= 1000 {
		return fmt.Sprintf("%.1f km", meters/1000)
	}
	return fmt.Sprintf("%d m", int(meters))
}
