// Package itest starts real Postgres, Redis, and Kafka via testcontainers-go for
// Fanout Worker integration tests. Tests skip when Docker is not running.
package itest

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/kafka"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

// PostgresRedisKafka starts the three backends the Fanout Worker talks to and
// applies Cascade SQL migrations to Postgres.
func PostgresRedisKafka(t *testing.T) (databaseURL, redisAddr, brokers string) {
	t.Helper()
	testcontainers.SkipIfProviderIsNotHealthy(t)
	ctx := context.Background()
	root := repoRoot(t)

	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("cascade"),
		postgres.WithUsername("cascade"),
		postgres.WithPassword("cascade"),
		postgres.WithInitScripts(migrationFiles(root)...),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("postgres container: %v", err)
	}
	testcontainers.CleanupContainer(t, pg)
	databaseURL, err = pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("postgres connection string: %v", err)
	}

	rd, err := tcredis.Run(ctx, "redis:7.4-alpine")
	if err != nil {
		t.Fatalf("redis container: %v", err)
	}
	testcontainers.CleanupContainer(t, rd)
	conn, err := rd.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("redis connection string: %v", err)
	}
	parsed, err := url.Parse(conn)
	if err != nil {
		t.Fatalf("parse redis URL %q: %v", conn, err)
	}

	k, err := kafka.Run(ctx, "confluentinc/confluent-local:7.5.0")
	if err != nil {
		t.Fatalf("kafka container: %v", err)
	}
	testcontainers.CleanupContainer(t, k)
	brokerList, err := k.Brokers(ctx)
	if err != nil {
		t.Fatalf("kafka brokers: %v", err)
	}
	return databaseURL, parsed.Host, strings.Join(brokerList, ",")
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for range 10 {
		candidate := filepath.Join(dir, "migrations", "000001_create_users_table.up.sql")
		if _, err := os.Stat(candidate); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("could not find repo root (migrations/*.up.sql)")
	return ""
}

func migrationFiles(root string) []string {
	return []string{
		filepath.Join(root, "migrations", "000001_create_users_table.up.sql"),
		filepath.Join(root, "migrations", "000002_create_follows_table.up.sql"),
		filepath.Join(root, "migrations", "000003_create_posts_table.up.sql"),
		filepath.Join(root, "migrations", "000004_create_engagements_table.up.sql"),
	}
}
