package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	OpenTopoDataServer       string
	OpenTopoDataMaxLocations int
	TracePathPointDistance   float64
	ClickHouseHost           string
	ClickHousePort           int
	ClickHouseDatabase       string
	ClickHouseUser           string
	ClickHousePassword       string
	APIHost                  string
	APIPort                  int
	CorsAllowedOrigins       []string
	Debug                    bool
	MeshcoreDashboardAPI     string
	PhotonServer             string
	GeocodeCacheTTLMinutes   int
	GeocodeMaxResults        int
	SearchInPoints           bool
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	port, _ := strconv.Atoi(getEnv("CLICKHOUSE_PORT", "9000"))
	apiPort, _ := strconv.Atoi(getEnv("API_PORT", "8000"))
	pointDistance, _ := strconv.ParseFloat(getEnv("TRACE_PATH_POINT_DISTANCE", "50"), 64)
	maxLocations, _ := strconv.Atoi(getEnv("OPENTOPADATA_MAX_LOCATIONS", "100"))
	geocodeCacheTTL, _ := strconv.Atoi(getEnv("GEOCODE_CACHE_TTL_MINUTES", "60"))
	geocodeMaxResults, _ := strconv.Atoi(getEnv("GEOCODE_MAX_RESULTS", "50"))

	return &Config{
		OpenTopoDataServer:       getEnv("OPENTOPADATA_SERVER", "https://api.opentopodata.org/v1/"),
		OpenTopoDataMaxLocations: maxLocations,
		TracePathPointDistance:   pointDistance,
		ClickHouseHost:           getEnv("CLICKHOUSE_HOST", "localhost"),
		ClickHousePort:           port,
		ClickHouseDatabase:       getEnv("CLICKHOUSE_DATABASE", "helixtrace"),
		ClickHouseUser:           getEnv("CLICKHOUSE_USER", "admin"),
		ClickHousePassword:       getEnv("CLICKHOUSE_PASSWORD", ""),
		APIHost:                  getEnv("API_HOST", "0.0.0.0"),
		APIPort:                  apiPort,
		CorsAllowedOrigins:       parseCorsOrigins(getEnv("CORS_ALLOWED_ORIGINS", "")),
		Debug:                    getEnv("DEBUG", "false") == "true",
		MeshcoreDashboardAPI:     getEnv("MESHCORE_DASHBOARD_API", ""),
		PhotonServer:             getEnv("PHOTON_SERVER", "https://photon.komoot.io"),
		GeocodeCacheTTLMinutes:   geocodeCacheTTL,
		GeocodeMaxResults:        geocodeMaxResults,
		SearchInPoints:           getEnv("SEARCH_IN_POINTS", "false") == "true",
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseCorsOrigins(value string) []string {
	if value == "" {
		return nil
	}
	var origins []string
	for _, origin := range strings.Split(value, ",") {
		trimmed := strings.TrimSpace(origin)
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}
