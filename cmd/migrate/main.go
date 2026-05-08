package main

import (
	"database/sql"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/rayan-aguiar/video-processor/internal/db"
)

func main() {
	_ = godotenv.Load()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://videoproc:videoproc@localhost:5432/video_processor?sslmode=disable"
	}

	conn, err := db.Open(databaseURL)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	migrationsDir := os.Getenv("MIGRATIONS_DIR")
	if migrationsDir == "" {
		migrationsDir = "./migrations"
	}

	files, err := ioutil.ReadDir(migrationsDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("migrations dir does not exist: %s", migrationsDir)
			return
		}
		log.Fatalf("read migrations dir: %v", err)
	}

	// collect .up.sql files
	var ups []string
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		if strings.HasSuffix(name, ".up.sql") {
			ups = append(ups, filepath.Join(migrationsDir, name))
		}
	}

	sort.Strings(ups)

	// ensure schema_migrations table exists
	if _, err := conn.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		log.Fatalf("ensure schema_migrations: %v", err)
	}

	for _, path := range ups {
		ver := migrationVersion(path)
		applied, err := isApplied(conn, ver)
		if err != nil {
			log.Fatalf("check applied: %v", err)
		}
		if applied {
			log.Printf("migration %s already applied, skipping", ver)
			continue
		}

		log.Printf("applying migration %s", ver)
		content, err := os.ReadFile(path)
		if err != nil {
			log.Fatalf("read migration %s: %v", path, err)
		}

		tx, err := conn.Begin()
		if err != nil {
			log.Fatalf("begin tx: %v", err)
		}

		// execute the SQL file content; sqlite3 supports multiple statements
		if _, err := tx.Exec(string(content)); err != nil {
			_ = tx.Rollback()
			log.Fatalf("exec migration %s: %v", ver, err)
		}

		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES($1, $2)`, ver, time.Now()); err != nil {
			_ = tx.Rollback()
			log.Fatalf("record migration %s: %v", ver, err)
		}

		if err := tx.Commit(); err != nil {
			log.Fatalf("commit migration %s: %v", ver, err)
		}

		log.Printf("migration %s applied", ver)
	}

	log.Println("all migrations processed")
}

func migrationVersion(path string) string {
	base := filepath.Base(path)
	// e.g. 0001_init.up.sql -> 0001_init
	return strings.TrimSuffix(base, ".up.sql")
}

func isApplied(conn *sql.DB, version string) (bool, error) {
	var v string
	row := conn.QueryRow(`SELECT version FROM schema_migrations WHERE version = $1`, version)
	if err := row.Scan(&v); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
