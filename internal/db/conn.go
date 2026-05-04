package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Open abre uma conexão configurada com SQLite.
// path pode ser um caminho de arquivo (./data/app.db) ou :memory: para testes.
func Open(path string) (*sql.DB, error) {
    dsn := fmt.Sprintf("%s?_foreign_keys=1", path)

    db, err := sql.Open("sqlite3", dsn)
    if err != nil {
        return nil, err
    }

    // Recomendações para SQLite: limitar conexões para evitar locking
    db.SetMaxOpenConns(1)
    db.SetMaxIdleConns(1)
    db.SetConnMaxLifetime(5 * time.Minute)

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := db.PingContext(ctx); err != nil {
        _ = db.Close()
        return nil, err
    }

    return db, nil
}
