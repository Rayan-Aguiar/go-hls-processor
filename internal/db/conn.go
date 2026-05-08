package db

import (
	"context"
	"database/sql"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
)

// Open abre conexao com o banco.
// Se o DSN for postgres:// ou postgresql:// usa PostgreSQL.
// Caso contrario, mantem compatibilidade com SQLite (usado em testes locais).
func Open(dsn string) (*sql.DB, error) {
	driver := "sqlite3"
	if isPostgresDSN(dsn) {
		driver = "pgx"
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}

	if driver == "pgx" {
		db.SetMaxOpenConns(20)
		db.SetMaxIdleConns(10)
		db.SetConnMaxLifetime(30 * time.Minute)
	} else {
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		db.SetConnMaxLifetime(5 * time.Minute)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func isPostgresDSN(dsn string) bool {
	return strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://")
}
