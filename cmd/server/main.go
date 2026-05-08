package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/rayan-aguiar/video-processor/internal/db"
	"github.com/rayan-aguiar/video-processor/internal/progress"
	"github.com/rayan-aguiar/video-processor/internal/queue"
	"github.com/rayan-aguiar/video-processor/internal/service"
)

type serverDeps struct {
	conn         *sql.DB
	uploadSvc    *service.UploadService
	dataDir      string
	redisAdapter *queue.RedisAdapter
}

type uploadResponse struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
}

type jobStatusResponse struct {
	ID        string     `json:"id"`
	Status    string     `json:"status"`
	InputPath string     `json:"input_path"`
	OutputDir *string    `json:"output_dir,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

type videoListItem struct {
	ID         string     `json:"id"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
	Thumbnail  string     `json:"thumbnail_url,omitempty"`
	MasterM3U8 string     `json:"master_playlist_url,omitempty"`
	WatchURL   string     `json:"watch_url,omitempty"`
}

func main() {
	_ = godotenv.Load()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://videoproc:videoproc@localhost:5432/video_processor?sslmode=disable"
	}

	conn, err := db.Open(databaseURL)
	if err != nil {
		log.Fatalf("conectar ao db: %v", err)
	}
	defer conn.Close()

	log.Println("🚀 PostgreSQL conectado com sucesso!")

	redisAdapter, err := queue.NewRedisAdapter(queue.RedisConfig{
		Host:     envOrDefault("REDIS_HOST", "localhost"),
		Port:     envOrDefault("REDIS_PORT", "6379"),
		Password: envOrDefault("REDIS_PASSWORD", "videoproc2024"),
		DB:       envIntOrDefault("REDIS_DB", 0),
	})
	if err != nil {
		log.Fatalf("conectar no redis: %v", err)
	}
	defer redisAdapter.Close()

	queueName := envOrDefault("QUEUE_NAME", "video:jobs")
	producer := queue.NewProducer(redisAdapter, queueName)
	dataDir := envOrDefault("DATA_DIR", "./data")
	uploadSvc := service.New(dataDir, conn, producer)

	deps := &serverDeps{
		conn:         conn,
		uploadSvc:    uploadSvc,
		dataDir:      dataDir,
		redisAdapter: redisAdapter,
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /", deps.handleDashboard)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
			"db":     "connected",
		})
	})

	mux.HandleFunc("GET /jobs/ping-db", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := conn.PingContext(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "error",
				"error":  err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
			"db":     "pong",
		})
	})

	mux.HandleFunc("POST /jobs/upload", deps.handleUpload)
	mux.HandleFunc("GET /jobs/{id}", deps.handleGetJob)
	mux.HandleFunc("GET /jobs/{id}/events", deps.handleJobEvents)
	mux.HandleFunc("GET /videos", deps.handleListVideos)
	mux.HandleFunc("GET /videos/{id}/{asset...}", deps.handleVideoAsset)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           loggingMiddleware(mux),
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("🚀 HTTP server ouvindo em http://localhost:%s", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("erro no servidor: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("sinal recebido, encerrando servidor...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("erro no shutdown gracioso: %v", err)
		if errClose := server.Close(); errClose != nil {
			log.Printf("erro ao fechar servidor: %v", errClose)
		}
	}

	log.Println("servidor finalizado")
}

func (s *serverDeps) handleDashboard(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join("web", "index.html"))
}

func (s *serverDeps) handleJobEvents(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if jobID == "" {
		http.Error(w, "job id é obrigatório", http.StatusBadRequest)
		return
	}
	progress.ServeSSE(r.Context(), s.redisAdapter.Client(), w, r, jobID)
}

func (s *serverDeps) handleUpload(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "campo multipart 'file' é obrigatório",
		})
		return
	}
	defer file.Close()

	result, err := s.uploadSvc.UploadAndValidateFile(r.Context(), service.UploadFileInput{
		Filename: header.Filename,
		FileSize: header.Size,
		Reader:   file,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusAccepted, uploadResponse{
		JobID:  result.JobID,
		Status: result.Status,
	})
}

func (s *serverDeps) handleGetJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if jobID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "job id é obrigatório"})
		return
	}

	job, err := db.GetJobByID(r.Context(), s.conn, jobID)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "job não encontrado"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	response := jobStatusResponse{
		ID:        job.ID,
		Status:    job.Status,
		InputPath: job.InputPath,
		CreatedAt: job.CreatedAt,
	}

	if job.OutputDir.Valid {
		response.OutputDir = &job.OutputDir.String
	}
	if job.UpdatedAt.Valid {
		t := job.UpdatedAt.Time
		response.UpdatedAt = &t
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *serverDeps) handleListVideos(w http.ResponseWriter, r *http.Request) {
	limit := envIntOrDefault("VIDEOS_LIST_DEFAULT_LIMIT", 100)
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}

	jobs, err := db.ListJobs(r.Context(), s.conn, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	items := make([]videoListItem, 0, len(jobs))
	for _, job := range jobs {
		item := videoListItem{
			ID:        job.ID,
			Status:    job.Status,
			CreatedAt: job.CreatedAt,
		}
		if job.UpdatedAt.Valid {
			t := job.UpdatedAt.Time
			item.UpdatedAt = &t
		}

		if job.OutputDir.Valid && job.Status == "completed" {
			item.Thumbnail = fmt.Sprintf("/videos/%s/thumbnail.jpg", job.ID)
			item.MasterM3U8 = fmt.Sprintf("/videos/%s/master.m3u8", job.ID)
			item.WatchURL = "/"
		}

		items = append(items, item)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"count":  len(items),
		"videos": items,
	})
}

func (s *serverDeps) handleVideoAsset(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	asset := r.PathValue("asset")

	if jobID == "" || asset == "" {
		http.NotFound(w, r)
		return
	}

	if strings.Contains(asset, "..") {
		http.Error(w, "caminho inválido", http.StatusBadRequest)
		return
	}

	cleanAsset := filepath.Clean(asset)
	if cleanAsset == "." || strings.HasPrefix(cleanAsset, "/") {
		http.Error(w, "asset inválido", http.StatusBadRequest)
		return
	}

	job, err := db.GetJobByID(r.Context(), s.conn, jobID)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !job.OutputDir.Valid {
		http.NotFound(w, r)
		return
	}

	base := filepath.Clean(job.OutputDir.String)
	if base == "" {
		base = filepath.Join(s.dataDir, jobID)
	}

	fullPath := filepath.Join(base, cleanAsset)
	rel, err := filepath.Rel(base, fullPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		http.Error(w, "asset fora do diretório do job", http.StatusBadRequest)
		return
	}

	if strings.HasSuffix(fullPath, ".m3u8") {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	}
	if strings.HasSuffix(fullPath, ".ts") {
		w.Header().Set("Content-Type", "video/mp2t")
	}

	http.ServeFile(w, r, fullPath)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}

	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}

	return v
}
