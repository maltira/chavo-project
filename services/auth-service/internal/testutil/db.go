// Package testutil содержит хелперы интеграционных тестов.
package testutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool возвращает пул к чистой схеме с применённой миграцией. Каждый пакет тестов получает
// свою схему, поэтому пакеты можно гонять параллельно на одной БД. Без TEST_DATABASE_URL тест пропускается.
func Pool(t *testing.T, schema string) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()

	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	// pg_trgm общий для всех схем, поэтому ставится в public; возможная гонка двух пакетов лечится повтором.
	for attempt := 0; ; attempt++ {
		if _, err = admin.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS pg_trgm SCHEMA public`); err == nil || attempt == 3 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	if _, err = admin.Exec(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS %q CASCADE; CREATE SCHEMA %q`, schema, schema)); err != nil {
		t.Fatal(err)
	}
	admin.Close()

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	_, file, _, _ := runtime.Caller(0)
	files, err := filepath.Glob(filepath.Join(filepath.Dir(file), "../../../../migrations/auth/*.up.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("find auth migrations: %v (%d files)", err, len(files))
	}
	sort.Strings(files)
	for _, f := range files {
		migration, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("apply %s: %v", filepath.Base(f), err)
		}
	}
	return pool
}
