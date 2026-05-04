package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"

	"github.com/rayan-aguiar/video-processor/internal/db"
)

func main() {
	_ = godotenv.Load() // Carrega as variáveis de ambiente do arquivo .env, se existir

	dbPath := os.Getenv("SQLITE_PATH")
	if dbPath == "" {
		dbPath = "./data/app.db" // Caminho padrão para o banco de dados
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		log.Fatalf("criar pasta do db: %v", err)
	}

	conn, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("conectar ao db: %v", err)
	}
	defer conn.Close()

	log.Println("🚀 SQLite conectado com sucesso!")
}