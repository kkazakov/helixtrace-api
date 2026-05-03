package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"github.com/wasp/helixtrace-api/internal/config"
	"github.com/wasp/helixtrace-api/internal/models"
)

type contextKey string

const emailKey contextKey = "email"

func ContextWithEmail(ctx context.Context, email string) context.Context {
	return context.WithValue(ctx, emailKey, email)
}

func EmailFromContext(ctx context.Context) (string, bool) {
	email, ok := ctx.Value(emailKey).(string)
	return email, ok
}

type PointHandler struct {
	Conn clickhouse.Conn
	Cfg  *config.Config
}

type createPointRequest struct {
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
	Public     bool    `json:"public"`
	Label      string  `json:"label"`
	CategoryID uint8   `json:"category_id"`
}

type updatePointRequest struct {
	Lat        *float64 `json:"lat"`
	Lon        *float64 `json:"lon"`
	Elevation  *float64 `json:"elevation"`
	Public     *bool    `json:"public"`
	Label      *string  `json:"label"`
	CategoryID *uint8   `json:"category_id"`
}

type pointResponse struct {
	ID         string  `json:"id"`
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
	Elevation  float64 `json:"elevation"`
	Public     bool    `json:"public"`
	Label      string  `json:"label"`
	CategoryID uint8   `json:"category_id"`
}

type pointDetailResponse struct {
	ID         string  `json:"id"`
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
	Elevation  float64 `json:"elevation"`
	Public     bool    `json:"public"`
	Label      string  `json:"label"`
	CategoryID uint8   `json:"category_id"`
	User       string  `json:"user,omitempty"`
}

type dataResponse struct {
	Data any `json:"data"`
}

type meshcoreRepeaterResponse struct {
	Status    string             `json:"status"`
	Repeaters []meshcoreRepeater `json:"repeaters"`
}

type meshcoreRepeater struct {
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
	LastHeard string `json:"last_heard"`
	Lat       string `json:"lat"`
	Lon       string `json:"lon"`
}

type externalPointResponse struct {
	ID         string  `json:"id"`
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
	Elevation  float64 `json:"elevation,omitempty"`
	Public     bool    `json:"public"`
	External   bool    `json:"external"`
	Label      string  `json:"label"`
	CategoryID uint8   `json:"category_id"`
}

type filteredRepeater struct {
	Name string
	ID   string
	Lat  float64
	Lon  float64
}

type cacheEntry struct {
	data      []json.RawMessage
	fetchedAt time.Time
}

var (
	meshcoreCache    sync.Map
	meshcoreCacheTTL = 1 * time.Hour
)

func (h *PointHandler) CreatePoint(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	defer r.Body.Close()

	var req createPointRequest
	if err := json.Unmarshal(body, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if req.Lat < -90 || req.Lat > 90 {
		WriteError(w, http.StatusBadRequest, "latitude must be between -90 and 90")
		return
	}
	if req.Lon < -180 || req.Lon > 180 {
		WriteError(w, http.StatusBadRequest, "longitude must be between -180 and 180")
		return
	}
	if req.CategoryID == 0 {
		WriteError(w, http.StatusBadRequest, "category_id is required")
		return
	}

	email, _ := EmailFromContext(r.Context())

	var catCount uint64
	err = h.Conn.QueryRow(r.Context(), `
		SELECT count() FROM point_categories FINAL WHERE id = ?
	`, req.CategoryID).Scan(&catCount)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to validate category")
		return
	}
	if catCount == 0 {
		WriteError(w, http.StatusBadRequest, "invalid category_id")
		return
	}

	elevation, err := h.fetchElevation(r.Context(), req.Lat, req.Lon)
	if err != nil {
		WriteError(w, http.StatusBadGateway, fmt.Sprintf("failed to fetch elevation: %v", err))
		return
	}

	id := uuid.New().String()
	now := time.Now().UTC()

	err = h.Conn.Exec(r.Context(), `
		INSERT INTO points (id, lat, lon, elevation, user, public, label, category_id, deleted, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, false, ?)
	`, id, req.Lat, req.Lon, elevation, email, req.Public, req.Label, req.CategoryID, now)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to create point")
		return
	}

	WriteJSON(w, http.StatusCreated, pointResponse{
		ID:         id,
		Lat:        req.Lat,
		Lon:        req.Lon,
		Elevation:  elevation,
		Public:     req.Public,
		Label:      req.Label,
		CategoryID: req.CategoryID,
	})
}

func (h *PointHandler) fetchElevation(ctx context.Context, lat, lon float64) (float64, error) {
	baseURL := strings.TrimSuffix(h.Cfg.OpenTopoDataServer, "/")
	reqURL := fmt.Sprintf("%s?locations=%.7f,%.7f", baseURL, lat, lon)

	resp, err := http.Get(reqURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var topoResp OpenTopoResponse
	if err := json.Unmarshal(body, &topoResp); err != nil {
		return 0, err
	}

	if topoResp.Status != "OK" || len(topoResp.Results) == 0 {
		return 0, fmt.Errorf("elevation service error: %s", topoResp.Status)
	}

	return math.Round(topoResp.Results[0].Elevation*100) / 100, nil
}

func (h *PointHandler) UpdatePoint(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteError(w, http.StatusBadRequest, "point id is required")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	defer r.Body.Close()

	var req updatePointRequest
	if err := json.Unmarshal(body, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid json")
		return
	}

	email, _ := EmailFromContext(r.Context())

	var existing models.Point
	err = h.Conn.QueryRow(r.Context(), `
		SELECT id, lat, lon, elevation, user, public, label, category_id, deleted
		FROM points FINAL
		WHERE id = ? AND user = ? AND deleted = false
	`, id, email).Scan(
		&existing.ID, &existing.Lat, &existing.Lon, &existing.Elevation,
		&existing.User, &existing.Public, &existing.Label, &existing.CategoryID, &existing.Deleted,
	)
	if err != nil {
		WriteError(w, http.StatusNotFound, "point not found")
		return
	}

	newLat := existing.Lat
	if req.Lat != nil {
		if *req.Lat < -90 || *req.Lat > 90 {
			WriteError(w, http.StatusBadRequest, "latitude must be between -90 and 90")
			return
		}
		newLat = *req.Lat
	}

	newLon := existing.Lon
	if req.Lon != nil {
		if *req.Lon < -180 || *req.Lon > 180 {
			WriteError(w, http.StatusBadRequest, "longitude must be between -180 and 180")
			return
		}
		newLon = *req.Lon
	}

	newElevation := existing.Elevation
	if req.Elevation != nil {
		newElevation = *req.Elevation
	}

	newPublic := existing.Public
	if req.Public != nil {
		newPublic = *req.Public
	}

	newLabel := existing.Label
	if req.Label != nil {
		newLabel = *req.Label
	}

	newCategoryID := existing.CategoryID
	if req.CategoryID != nil {
		if *req.CategoryID == 0 {
			WriteError(w, http.StatusBadRequest, "invalid category_id")
			return
		}
		var catCount uint64
		err = h.Conn.QueryRow(r.Context(), `
			SELECT count() FROM point_categories FINAL WHERE id = ?
		`, *req.CategoryID).Scan(&catCount)
		if err != nil || catCount == 0 {
			WriteError(w, http.StatusBadRequest, "invalid category_id")
			return
		}
		newCategoryID = *req.CategoryID
	}

	now := time.Now().UTC()

	err = h.Conn.Exec(r.Context(), `
		INSERT INTO points (id, lat, lon, elevation, user, public, label, category_id, deleted, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, false, ?)
	`, id, newLat, newLon, newElevation, email, newPublic, newLabel, newCategoryID, now)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to update point")
		return
	}

	WriteJSON(w, http.StatusOK, pointResponse{
		ID:         id,
		Lat:        newLat,
		Lon:        newLon,
		Elevation:  newElevation,
		Public:     newPublic,
		Label:      newLabel,
		CategoryID: newCategoryID,
	})
}

func (h *PointHandler) DeletePoint(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteError(w, http.StatusBadRequest, "point id is required")
		return
	}

	email, _ := EmailFromContext(r.Context())

	var existing models.Point
	err := h.Conn.QueryRow(r.Context(), `
		SELECT id, lat, lon, elevation, user, public, label, category_id, deleted
		FROM points FINAL
		WHERE id = ? AND user = ? AND deleted = false
	`, id, email).Scan(
		&existing.ID, &existing.Lat, &existing.Lon, &existing.Elevation,
		&existing.User, &existing.Public, &existing.Label, &existing.CategoryID, &existing.Deleted,
	)
	if err != nil {
		WriteError(w, http.StatusNotFound, "point not found")
		return
	}

	now := time.Now().UTC()

	err = h.Conn.Exec(r.Context(), `
		INSERT INTO points (id, lat, lon, elevation, user, public, label, category_id, deleted, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, true, ?)
	`, id, existing.Lat, existing.Lon, existing.Elevation, email, existing.Public, existing.Label, existing.CategoryID, now)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to delete point")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *PointHandler) GetPoint(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		WriteError(w, http.StatusBadRequest, "point id is required")
		return
	}

	email, _ := EmailFromContext(r.Context())

	var point models.Point
	err := h.Conn.QueryRow(r.Context(), `
		SELECT id, lat, lon, elevation, user, public, label, category_id, deleted
		FROM points FINAL
		WHERE id = ? AND deleted = false AND (user = ? OR public = true)
	`, id, email).Scan(
		&point.ID, &point.Lat, &point.Lon, &point.Elevation,
		&point.User, &point.Public, &point.Label, &point.CategoryID, &point.Deleted,
	)
	if err != nil {
		WriteError(w, http.StatusNotFound, "point not found")
		return
	}

	resp := pointDetailResponse{
		ID:         point.ID,
		Lat:        point.Lat,
		Lon:        point.Lon,
		Elevation:  point.Elevation,
		Public:     point.Public,
		Label:      point.Label,
		CategoryID: point.CategoryID,
	}
	if point.User == email {
		resp.User = point.User
	}

	WriteJSON(w, http.StatusOK, dataResponse{
		Data: resp,
	})
}

func (h *PointHandler) ListPoints(w http.ResponseWriter, r *http.Request) {
	email, _ := EmailFromContext(r.Context())
	includePublic := r.URL.Query().Get("include_public") == "true"
	includeMeshcore := r.URL.Query().Get("include_meshcore_dashboard") == "true"

	var rows driver.Rows
	var err error

	if includePublic {
		rows, err = h.Conn.Query(r.Context(), `
			SELECT id, lat, lon, elevation, public, label, category_id
			FROM points FINAL
			WHERE deleted = false AND (user = ? OR public = true)
			ORDER BY updated_at DESC
		`, email)
	} else {
		rows, err = h.Conn.Query(r.Context(), `
			SELECT id, lat, lon, elevation, public, label, category_id
			FROM points FINAL
			WHERE user = ? AND deleted = false
			ORDER BY updated_at DESC
		`, email)
	}
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to list points")
		return
	}
	defer rows.Close()

	var points []pointResponse
	for rows.Next() {
		var p pointResponse
		if err := rows.Scan(&p.ID, &p.Lat, &p.Lon, &p.Elevation, &p.Public, &p.Label, &p.CategoryID); err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to scan point")
			return
		}
		points = append(points, p)
	}

	if points == nil {
		points = []pointResponse{}
	}

	if includeMeshcore && h.Cfg.MeshcoreDashboardAPI != "" {
		extPoints := h.fetchMeshcoreRepeaters()
		result := make([]any, 0, len(points)+len(extPoints))
		for _, p := range points {
			result = append(result, p)
		}
		for _, ep := range extPoints {
			var raw any
			json.Unmarshal(ep, &raw)
			result = append(result, raw)
		}
		WriteJSON(w, http.StatusOK, result)
		return
	}

	WriteJSON(w, http.StatusOK, points)
}

type pointInfoResponse struct {
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	Elevation float64 `json:"elevation"`
}

func (h *PointHandler) GetPointInfo(w http.ResponseWriter, r *http.Request) {
	latStr := r.URL.Query().Get("lat")
	lonStr := r.URL.Query().Get("lon")

	if latStr == "" || lonStr == "" {
		WriteError(w, http.StatusBadRequest, "lat and lon query parameters are required")
		return
	}

	var lat, lon float64
	if _, err := fmt.Sscanf(latStr, "%f", &lat); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid lat parameter")
		return
	}
	if _, err := fmt.Sscanf(lonStr, "%f", &lon); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid lon parameter")
		return
	}

	if lat < -90 || lat > 90 {
		WriteError(w, http.StatusBadRequest, "latitude must be between -90 and 90")
		return
	}
	if lon < -180 || lon > 180 {
		WriteError(w, http.StatusBadRequest, "longitude must be between -180 and 180")
		return
	}

	elevation, err := h.fetchElevation(r.Context(), lat, lon)
	if err != nil {
		WriteError(w, http.StatusBadGateway, fmt.Sprintf("failed to fetch elevation: %v", err))
		return
	}

	WriteJSON(w, http.StatusOK, dataResponse{
		Data: pointInfoResponse{
			Lat:       lat,
			Lon:       lon,
			Elevation: elevation,
		},
	})
}

func (h *PointHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Conn.Query(r.Context(), `
		SELECT id, name FROM point_categories FINAL ORDER BY id
	`)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to list categories")
		return
	}
	defer rows.Close()

	var categories []models.PointCategory
	for rows.Next() {
		var c models.PointCategory
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to scan category")
			return
		}
		categories = append(categories, c)
	}

	if categories == nil {
		categories = []models.PointCategory{}
	}

	WriteJSON(w, http.StatusOK, categories)
}

func (h *PointHandler) fetchMeshcoreRepeaters() []json.RawMessage {
	now := time.Now().UTC()
	cutoff := now.Add(-2 * 7 * 24 * time.Hour)

	// Check cache first
	if entry, ok := meshcoreCache.Load("meshcore_repeaters"); ok {
		e := entry.(cacheEntry)
		if now.Sub(e.fetchedAt) < meshcoreCacheTTL {
			return e.data
		}
	}

	// Fetch from external API
	baseURL := strings.TrimSuffix(h.Cfg.MeshcoreDashboardAPI, "/")
	reqURL := fmt.Sprintf("%s/api/repeaters/companion", baseURL)
	log.Printf("[meshcore] fetching repeaters from %s", reqURL)

	resp, err := http.Get(reqURL)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	var apiResp meshcoreRepeaterResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil
	}
	if apiResp.Status != "ok" {
		return nil
	}

	filtered := filterRepeaters(apiResp.Repeaters, cutoff)
	if len(filtered) == 0 {
		return []json.RawMessage{}
	}

	elevations := fetchElevationsBatched(h.Cfg, filtered)

	var result []json.RawMessage
	for i, fr := range filtered {
		ep := externalPointResponse{
			ID:         fr.ID,
			Lat:        fr.Lat,
			Lon:        fr.Lon,
			Public:     true,
			External:   true,
			Label:      fr.Name,
			CategoryID: 2,
		}
		if i < len(elevations) {
			ep.Elevation = elevations[i]
		}

		raw, err := json.Marshal(ep)
		if err != nil {
			continue
		}
		result = append(result, raw)
	}

	if result == nil {
		result = []json.RawMessage{}
	}

	// Cache the final JSON response
	meshcoreCache.Store("meshcore_repeaters", cacheEntry{
		data:      result,
		fetchedAt: now,
	})

	return result
}

func filterRepeaters(repeaters []meshcoreRepeater, cutoff time.Time) []filteredRepeater {
	var result []filteredRepeater
	for _, r := range repeaters {
		lastHeard, err := time.Parse(time.RFC3339, r.LastHeard)
		if err != nil {
			continue
		}
		if lastHeard.Before(cutoff) {
			continue
		}

		var latVal, lonVal float64
		if _, err := fmt.Sscanf(r.Lat, "%f", &latVal); err != nil {
			continue
		}
		if _, err := fmt.Sscanf(r.Lon, "%f", &lonVal); err != nil {
			continue
		}

		result = append(result, filteredRepeater{
			Name: r.Name,
			ID:   r.PublicKey,
			Lat:  latVal,
			Lon:  lonVal,
		})
	}
	return result
}

func fetchElevationsBatched(cfg *config.Config, repeaters []filteredRepeater) []float64 {
	maxLocs := cfg.OpenTopoDataMaxLocations
	if maxLocs <= 0 {
		maxLocs = 100
	}

	baseURL := strings.TrimSuffix(cfg.OpenTopoDataServer, "/")

	var allElevations []float64
	for i := 0; i < len(repeaters); i += maxLocs {
		end := i + maxLocs
		if end > len(repeaters) {
			end = len(repeaters)
		}
		batch := repeaters[i:end]

		var locs []string
		for _, p := range batch {
			locs = append(locs, fmt.Sprintf("%.7f,%.7f", p.Lat, p.Lon))
		}
		reqURL := fmt.Sprintf("%s?locations=%s", baseURL, strings.Join(locs, "|"))

		resp, err := http.Get(reqURL)
		if err != nil {
			log.Printf("[meshcore] elevation fetch error: %v", err)
			for range batch {
				allElevations = append(allElevations, 0)
			}
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Printf("[meshcore] elevation read error: %v", err)
			for range batch {
				allElevations = append(allElevations, 0)
			}
			continue
		}

		var topoResp OpenTopoResponse
		if err := json.Unmarshal(body, &topoResp); err != nil {
			log.Printf("[meshcore] elevation parse error: %v", err)
			for range batch {
				allElevations = append(allElevations, 0)
			}
			continue
		}

		if topoResp.Status != "OK" {
			log.Printf("[meshcore] elevation service error: %s", topoResp.Status)
			for range batch {
				allElevations = append(allElevations, 0)
			}
			continue
		}

		for _, res := range topoResp.Results {
			elv := math.Round(res.Elevation*100) / 100
			allElevations = append(allElevations, elv)
		}
	}

	return allElevations
}
