package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/wasp/helixtrace-api/internal/config"
)

type GeocodeHandler struct {
	Conn clickhouse.Conn
	Cfg  *config.Config
	mu   sync.Map
}

type geocodeCacheEntry struct {
	results   []GeocodeResult
	fetchedAt time.Time
}

type GeocodeResult struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	DisplayName string  `json:"display_name"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	Type        string  `json:"type"`
}

type GeocodeResponse struct {
	Results []GeocodeResult `json:"results"`
}

// Photon GeoJSON structures
type photonFeatureCollection struct {
	Features []photonFeature `json:"features"`
}

type photonFeature struct {
	Type       string           `json:"type"`
	Geometry   photonGeometry   `json:"geometry"`
	Properties photonProperties `json:"properties"`
}

type photonGeometry struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

type photonProperties struct {
	OSMType  string `json:"osm_type"`
	OSMID    int64  `json:"osm_id"`
	Name     string `json:"name"`
	Street   string `json:"street"`
	City     string `json:"city"`
	District string `json:"district"`
	State    string `json:"state"`
	Country  string `json:"country"`
	Type     string `json:"type"`
	OSMKey   string `json:"osm_key"`
	OSMValue string `json:"osm_value"`
}

func (h *GeocodeHandler) Geocode(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		WriteError(w, http.StatusBadRequest, "q query parameter is required")
		return
	}

	limit := h.Cfg.GeocodeMaxResults
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil {
			if parsed < 1 {
				parsed = 1
			}
			if parsed > h.Cfg.GeocodeMaxResults {
				parsed = h.Cfg.GeocodeMaxResults
			}
			limit = parsed
		}
	}

	email, _ := EmailFromContext(r.Context())

	cacheKey := fmt.Sprintf("%s:%d", query, limit)
	if h.Cfg.SearchInPoints {
		cacheKey = fmt.Sprintf("%s:%s", email, cacheKey)
	}

	if entry, ok := h.mu.Load(cacheKey); ok {
		ce := entry.(geocodeCacheEntry)
		if time.Since(ce.fetchedAt) < time.Duration(h.Cfg.GeocodeCacheTTLMinutes)*time.Minute {
			if h.Cfg.Debug {
				log.Printf("[geocode] CACHE HIT for %q", query)
			}
			WriteJSON(w, http.StatusOK, GeocodeResponse{Results: ce.results})
			return
		}
	}

	if h.Cfg.Debug {
		log.Printf("[geocode] CACHE MISS for %q", query)
	}

	// Fetch Photon results
	baseURL := strings.TrimSuffix(h.Cfg.PhotonServer, "/")
	reqURL := fmt.Sprintf("%s/api/?q=%s&limit=%d", baseURL, url.QueryEscape(query), limit)

	resp, err := http.Get(reqURL)
	if err != nil {
		WriteError(w, http.StatusBadGateway, fmt.Sprintf("failed to call geocode service: %v", err))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		WriteError(w, http.StatusBadGateway, "failed to read geocode service response")
		return
	}

	var fc photonFeatureCollection
	if err := json.Unmarshal(body, &fc); err != nil {
		WriteError(w, http.StatusBadGateway, "invalid response from geocode service")
		return
	}

	results := make([]GeocodeResult, 0, len(fc.Features))
	for _, f := range fc.Features {
		if len(f.Geometry.Coordinates) < 2 {
			continue
		}

		osmType := mapPhotonOSMType(f.Properties.OSMType)
		id := fmt.Sprintf("osm:%s:%d", osmType, f.Properties.OSMID)

		typ := f.Properties.Type
		if typ == "" {
			typ = f.Properties.OSMValue
		}

		results = append(results, GeocodeResult{
			ID:          id,
			Name:        f.Properties.Name,
			DisplayName: buildDisplayName(f.Properties),
			Lat:         f.Geometry.Coordinates[1],
			Lon:         f.Geometry.Coordinates[0],
			Type:        typ,
		})
	}

	if h.Cfg.SearchInPoints {
		local, err := h.searchLocalPoints(r.Context(), email, query)
		if err != nil {
			log.Printf("[geocode] local points search error: %v", err)
		} else if len(local) > 0 {
			results = append(local, results...)
		}
	}

	if len(results) > limit {
		results = results[:limit]
	}

	if results == nil {
		results = []GeocodeResult{}
	}

	h.mu.Store(cacheKey, geocodeCacheEntry{
		results:   results,
		fetchedAt: time.Now().UTC(),
	})

	WriteJSON(w, http.StatusOK, GeocodeResponse{Results: results})
}

func (h *GeocodeHandler) searchLocalPoints(ctx context.Context, email, query string) ([]GeocodeResult, error) {
	pattern := "%" + query + "%"
	rows, err := h.Conn.Query(ctx, `
		SELECT p.id, p.lat, p.lon, p.label, c.name
		FROM points AS p FINAL
		LEFT JOIN point_categories AS c FINAL ON p.category_id = c.id
		WHERE p.user = ? AND p.deleted = false AND ilike(p.label, ?)
		ORDER BY p.updated_at DESC
	`, email, pattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []GeocodeResult
	for rows.Next() {
		var id, label, catName string
		var lat, lon float64
		if err := rows.Scan(&id, &lat, &lon, &label, &catName); err != nil {
			continue
		}
		typ := catName
		if typ == "" {
			typ = "point"
		}
		results = append(results, GeocodeResult{
			ID:          id,
			Name:        label,
			DisplayName: label,
			Lat:         lat,
			Lon:         lon,
			Type:        typ,
		})
	}
	return results, nil
}

func mapPhotonOSMType(t string) string {
	switch t {
	case "N":
		return "node"
	case "W":
		return "way"
	case "R":
		return "relation"
	default:
		return strings.ToLower(t)
	}
}

func buildDisplayName(p photonProperties) string {
	parts := []string{}
	if p.Name != "" {
		parts = append(parts, p.Name)
	}
	if p.Street != "" && p.Street != p.Name {
		parts = append(parts, p.Street)
	}
	if p.City != "" {
		parts = append(parts, p.City)
	}
	if p.District != "" && p.District != p.City {
		parts = append(parts, p.District)
	}
	if p.State != "" {
		parts = append(parts, p.State)
	}
	if p.Country != "" {
		parts = append(parts, p.Country)
	}

	if len(parts) == 0 {
		return p.Name
	}
	return strings.Join(parts, ", ")
}
