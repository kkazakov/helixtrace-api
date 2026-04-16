package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
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
	User       string  `json:"user"`
}

type dataResponse struct {
	Data any `json:"data"`
}

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
		WHERE id = ? AND user = ? AND deleted = false
	`, id, email).Scan(
		&point.ID, &point.Lat, &point.Lon, &point.Elevation,
		&point.User, &point.Public, &point.Label, &point.CategoryID, &point.Deleted,
	)
	if err != nil {
		WriteError(w, http.StatusNotFound, "point not found")
		return
	}

	WriteJSON(w, http.StatusOK, dataResponse{
		Data: pointDetailResponse{
			ID:         point.ID,
			Lat:        point.Lat,
			Lon:        point.Lon,
			Elevation:  point.Elevation,
			Public:     point.Public,
			Label:      point.Label,
			CategoryID: point.CategoryID,
			User:       point.User,
		},
	})
}

func (h *PointHandler) ListPoints(w http.ResponseWriter, r *http.Request) {
	email, _ := EmailFromContext(r.Context())
	includePublic := r.URL.Query().Get("include_public") == "true"

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

	WriteJSON(w, http.StatusOK, points)
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
