package database

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/wasp/helixtrace-api/internal/config"
)

func Connect(cfg *config.Config) (clickhouse.Conn, error) {
	opts := &clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", cfg.ClickHouseHost, cfg.ClickHousePort)},
		Auth: clickhouse.Auth{
			Database: cfg.ClickHouseDatabase,
			Username: cfg.ClickHouseUser,
			Password: cfg.ClickHousePassword,
		},
	}

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open clickhouse connection: %w", err)
	}

	if err := conn.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping clickhouse: %w", err)
	}

	return conn, nil
}

func InitDB(conn clickhouse.Conn, sqlDir string) error {
	files, err := filepath.Glob(filepath.Join(sqlDir, "*.sql"))
	if err != nil {
		return fmt.Errorf("failed to glob sql files: %w", err)
	}

	sort.Strings(files)

	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", f, err)
		}

		var cleaned strings.Builder
		scanner := bufio.NewScanner(strings.NewReader(string(content)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "--") {
				continue
			}
			cleaned.WriteString(line)
			cleaned.WriteString(" ")
		}

		statements := strings.Split(cleaned.String(), ";")
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}

			if err := conn.Exec(context.Background(), stmt); err != nil {
				return fmt.Errorf("failed to execute %s: %w", f, err)
			}
		}
	}

	return nil
}
