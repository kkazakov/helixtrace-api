package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/wasp/helixtrace-api/internal/config"
)

type TracePathHandler struct {
	Conn clickhouse.Conn
	Cfg  *config.Config
	mu   sync.Map
}

type TracePathPoint struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
	Elv float64 `json:"elv"`
}

type TracePathResponse struct {
	Points                []TracePathPoint `json:"points"`
	Count                 int              `json:"count"`
	DistanceBetweenPoints float64          `json:"distance_between_points"`
	Status                string           `json:"status"`
}

type OpenTopoResult struct {
	Elevation float64 `json:"elevation"`
	Location  struct {
		Lat float64 `json:"lat"`
		Lng float64 `json:"lng"`
	} `json:"location"`
}

type OpenTopoResponse struct {
	Results []OpenTopoResult `json:"results"`
	Status  string           `json:"status"`
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000.0
	dLat := (lat2 - lat1) * math.Pi / 180.0
	dLon := (lon2 - lon1) * math.Pi / 180.0
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180.0)*math.Cos(lat2*math.Pi/180.0)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

func makePathHash(fromLat, fromLon, toLat, toLon, pointDist float64) string {
	h := sha256.New()
	fmt.Fprintf(h, "%.7f,%.7f,%.7f,%.7f,%.2f", fromLat, fromLon, toLat, toLon, pointDist)
	return hex.EncodeToString(h.Sum(nil))
}

func (h *TracePathHandler) lookupCache(ctx context.Context, pathHash string) ([]float64, []float64, []float64, uint32, error) {
	var lats, lngs, elvs []float64
	var count uint32
	err := h.Conn.QueryRow(ctx, `
		SELECT lats, lngs, elvs, count FROM trace_paths
		WHERE path_hash = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, pathHash).Scan(&lats, &lngs, &elvs, &count)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	return lats, lngs, elvs, count, nil
}

func (h *TracePathHandler) cacheExists(ctx context.Context, pathHash string) (bool, error) {
	var n uint64
	err := h.Conn.QueryRow(ctx, `
		SELECT count() FROM trace_paths WHERE path_hash = ?
	`, pathHash).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (h *TracePathHandler) saveCache(ctx context.Context, pathHash string, fromLat, fromLon, toLat, toLon, pointDist float64, lats, lngs, elvs []float64, count int) error {
	return h.Conn.Exec(ctx, `
		INSERT INTO trace_paths (path_hash, from_lat, from_lon, to_lat, to_lon, point_distance, count, lats, lngs, elvs, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, pathHash, fromLat, fromLon, toLat, toLon, pointDist, count, lats, lngs, elvs, time.Now().UTC())
}

func (h *TracePathHandler) buildResponse(lats, lngs, elvs []float64, count uint32, distance float64, numPoints int) TracePathResponse {
	resultPoints := make([]TracePathPoint, count)
	for i := uint32(0); i < count; i++ {
		resultPoints[i] = TracePathPoint{
			Lat: lats[i],
			Lng: lngs[i],
			Elv: elvs[i],
		}
	}
	return TracePathResponse{
		Points:                resultPoints,
		Count:                 int(count),
		DistanceBetweenPoints: math.Round(distance/float64(numPoints-1)*100) / 100,
		Status:                "ok",
	}
}

func (h *TracePathHandler) TracePath(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	if from == "" || to == "" {
		WriteError(w, http.StatusBadRequest, "from and to query parameters are required")
		return
	}

	fromParts := strings.Split(from, ",")
	toParts := strings.Split(to, ",")
	if len(fromParts) != 2 || len(toParts) != 2 {
		WriteError(w, http.StatusBadRequest, "from and to must be in lat,lon format")
		return
	}

	fromLat, err := strconv.ParseFloat(strings.TrimSpace(fromParts[0]), 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid from latitude")
		return
	}
	fromLon, err := strconv.ParseFloat(strings.TrimSpace(fromParts[1]), 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid from longitude")
		return
	}
	toLat, err := strconv.ParseFloat(strings.TrimSpace(toParts[0]), 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid to latitude")
		return
	}
	toLon, err := strconv.ParseFloat(strings.TrimSpace(toParts[1]), 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid to longitude")
		return
	}

	distance := haversine(fromLat, fromLon, toLat, toLon)
	numPoints := int(distance / h.Cfg.TracePathPointDistance)
	if numPoints < 2 {
		numPoints = 2
	}
	if numPoints > 1000 {
		numPoints = 1000
	}

	pathHash := makePathHash(fromLat, fromLon, toLat, toLon, h.Cfg.TracePathPointDistance)
	log.Printf("[trace-path] hash=%s", pathHash[:16])

	cachedLats, cachedLngs, cachedElvs, cachedCount, err := h.lookupCache(r.Context(), pathHash)
	log.Printf("[trace-path] lookupCache: err=%v count=%d", err, cachedCount)
	if err == nil && cachedCount > 0 {
		log.Printf("[trace-path] CACHE HIT")
		WriteJSON(w, http.StatusOK, h.buildResponse(cachedLats, cachedLngs, cachedElvs, cachedCount, distance, numPoints))
		return
	}

	log.Printf("[trace-path] CACHE MISS, acquiring lock")
	val, _ := h.mu.LoadOrStore(pathHash, &sync.Mutex{})
	mtx := val.(*sync.Mutex)
	mtx.Lock()
	defer mtx.Unlock()

	exists, _ := h.cacheExists(r.Context(), pathHash)
	log.Printf("[trace-path] after lock cacheExists=%v", exists)
	if exists {
		cachedLats, cachedLngs, cachedElvs, cachedCount, err = h.lookupCache(r.Context(), pathHash)
		if err == nil && cachedCount > 0 {
			log.Printf("[trace-path] CACHE HIT (after lock)")
			WriteJSON(w, http.StatusOK, h.buildResponse(cachedLats, cachedLngs, cachedElvs, cachedCount, distance, numPoints))
			return
		}
	}

	log.Printf("[trace-path] calling OpenTopoData with %d points", numPoints)

	points := make([]struct{ lat, lng float64 }, numPoints)
	for i := 0; i < numPoints; i++ {
		t := float64(i) / float64(numPoints-1)
		points[i].lat = fromLat + (toLat-fromLat)*t
		points[i].lng = fromLon + (toLon-fromLon)*t
	}

	baseURL := strings.TrimSuffix(h.Cfg.OpenTopoDataServer, "/")
	maxLocs := h.Cfg.OpenTopoDataMaxLocations

	var allResults []OpenTopoResult
	for i := 0; i < len(points); i += maxLocs {
		end := i + maxLocs
		if end > len(points) {
			end = len(points)
		}
		batch := points[i:end]
		var batchLocs []string
		for _, p := range batch {
			batchLocs = append(batchLocs, fmt.Sprintf("%.7f,%.7f", p.lat, p.lng))
		}
		reqURL := fmt.Sprintf("%s?locations=%s", baseURL, strings.Join(batchLocs, "|"))

		resp, err := http.Get(reqURL)
		if err != nil {
			WriteError(w, http.StatusBadGateway, fmt.Sprintf("failed to call elevation service: %v", err))
			return
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			WriteError(w, http.StatusBadGateway, "failed to read elevation service response")
			return
		}

		var topoResp OpenTopoResponse
		if err := json.Unmarshal(body, &topoResp); err != nil {
			WriteError(w, http.StatusBadGateway, "invalid response from elevation service")
			return
		}

		if topoResp.Status != "OK" {
			WriteError(w, http.StatusBadGateway, fmt.Sprintf("elevation service error: %s", topoResp.Status))
			return
		}

		allResults = append(allResults, topoResp.Results...)
	}

	resultPoints := make([]TracePathPoint, len(allResults))
	lats := make([]float64, len(allResults))
	lngs := make([]float64, len(allResults))
	elvs := make([]float64, len(allResults))
	for i, res := range allResults {
		elv := math.Round(res.Elevation*100) / 100
		resultPoints[i] = TracePathPoint{
			Lat: res.Location.Lat,
			Lng: res.Location.Lng,
			Elv: elv,
		}
		lats[i] = res.Location.Lat
		lngs[i] = res.Location.Lng
		elvs[i] = elv
	}

	h.saveCache(r.Context(), pathHash, fromLat, fromLon, toLat, toLon, h.Cfg.TracePathPointDistance, lats, lngs, elvs, len(resultPoints))
	log.Printf("[trace-path] saved to cache")

	WriteJSON(w, http.StatusOK, h.buildResponse(lats, lngs, elvs, uint32(len(resultPoints)), distance, numPoints))
}
