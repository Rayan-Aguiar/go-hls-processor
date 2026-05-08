package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type stats struct {
	mu        sync.Mutex
	latencies []time.Duration
	errors    int64
	total     int64
}

func (s *stats) add(latency time.Duration, err error) {
	s.mu.Lock()
	s.latencies = append(s.latencies, latency)
	s.mu.Unlock()
	atomic.AddInt64(&s.total, 1)
	if err != nil {
		atomic.AddInt64(&s.errors, 1)
	}
}

func (s *stats) snapshot() (count int, errors int, p50, p95, p99 time.Duration, max time.Duration) {
	s.mu.Lock()
	copyLat := append([]time.Duration(nil), s.latencies...)
	s.mu.Unlock()

	if len(copyLat) == 0 {
		return 0, int(atomic.LoadInt64(&s.errors)), 0, 0, 0, 0
	}

	sort.Slice(copyLat, func(i, j int) bool { return copyLat[i] < copyLat[j] })
	count = len(copyLat)
	errors = int(atomic.LoadInt64(&s.errors))
	p50 = percentile(copyLat, 0.50)
	p95 = percentile(copyLat, 0.95)
	p99 = percentile(copyLat, 0.99)
	max = copyLat[len(copyLat)-1]
	return
}

func percentile(values []time.Duration, p float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	idx := int(float64(len(values)-1) * p)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(values) {
		idx = len(values) - 1
	}
	return values[idx]
}

func main() {
	var (
		baseURL      = flag.String("base-url", "http://localhost:8000", "Base URL da API")
		filePath     = flag.String("file", "", "Caminho de um video real para upload (obrigatorio)")
		uploads      = flag.Int("uploads", 80, "Quantidade total de uploads")
		concurrency  = flag.Int("concurrency", 12, "Concorrencia de uploads")
		probeEvery   = flag.Duration("probe-every", 500*time.Millisecond, "Intervalo entre probes de responsividade")
		settleAfter  = flag.Duration("settle-after", 45*time.Second, "Tempo adicional de medicao apos terminar uploads")
		maxP95       = flag.Duration("max-p95", 400*time.Millisecond, "Limite de p95 para considerar o servidor responsivo")
		maxErrorRate = flag.Float64("max-error-rate", 0.02, "Limite de taxa de erro dos probes")
	)
	flag.Parse()

	if strings.TrimSpace(*filePath) == "" {
		exitf("erro: use -file com caminho de um video real")
	}
	if *uploads <= 0 || *concurrency <= 0 {
		exitf("erro: uploads e concurrency devem ser > 0")
	}

	if _, err := os.Stat(*filePath); err != nil {
		exitf("erro: arquivo informado em -file nao existe: %v", err)
	}

	base, err := url.Parse(*baseURL)
	if err != nil {
		exitf("erro: base-url invalida: %v", err)
	}

	uploadURL := base.ResolveReference(&url.URL{Path: "/jobs/upload"}).String()
	healthURL := base.ResolveReference(&url.URL{Path: "/health"}).String()
	videosURL := base.ResolveReference(&url.URL{Path: "/videos", RawQuery: "limit=20"}).String()

	client := &http.Client{Timeout: 30 * time.Second}

	probeStats := &stats{}
	uploadStats := &stats{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		probeLoop(ctx, client, *probeEvery, probeStats, healthURL, videosURL)
	}()

	fmt.Printf("Iniciando carga: uploads=%d concurrency=%d\n", *uploads, *concurrency)
	start := time.Now()
	runUploads(client, uploadURL, *filePath, *uploads, *concurrency, uploadStats)
	uploadElapsed := time.Since(start)
	fmt.Printf("Uploads finalizados em %s\n", uploadElapsed)

	fmt.Printf("Mantendo probes por mais %s para validar responsividade sob backlog...\n", *settleAfter)
	timer := time.NewTimer(*settleAfter)
	<-timer.C
	cancel()
	wg.Wait()

	uCount, uErr, uP50, uP95, uP99, uMax := uploadStats.snapshot()
	pCount, pErr, pP50, pP95, pP99, pMax := probeStats.snapshot()

	probeErrorRate := 0.0
	if pCount > 0 {
		probeErrorRate = float64(pErr) / float64(pCount)
	}

	fmt.Println("\n=== Resultado Uploads ===")
	fmt.Printf("requests=%d errors=%d p50=%s p95=%s p99=%s max=%s\n", uCount, uErr, uP50, uP95, uP99, uMax)

	fmt.Println("\n=== Resultado Responsividade (probes /health + /videos) ===")
	fmt.Printf("requests=%d errors=%d error_rate=%.2f%% p50=%s p95=%s p99=%s max=%s\n", pCount, pErr, probeErrorRate*100, pP50, pP95, pP99, pMax)
	fmt.Printf("criterios: p95 <= %s, error_rate <= %.2f%%\n", *maxP95, *maxErrorRate*100)

	if pCount == 0 {
		exitf("FAIL: nenhum probe foi coletado")
	}
	if pP95 > *maxP95 || probeErrorRate > *maxErrorRate {
		exitf("FAIL: servidor nao atendeu os criterios de responsividade")
	}

	fmt.Println("PASS: servidor responsivo sob carga segundo os criterios configurados")
}

func runUploads(client *http.Client, uploadURL, filePath string, total, concurrency int, st *stats) {
	jobs := make(chan int)
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for id := range jobs {
				start := time.Now()
				err := uploadOne(client, uploadURL, filePath, id)
				st.add(time.Since(start), err)
			}
		}(i + 1)
	}

	for i := 1; i <= total; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
}

func uploadOne(client *http.Client, uploadURL, filePath string, seq int) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", fmt.Sprintf("loadtest-%04d%s", seq, fileExt(filePath)))
	if err != nil {
		return fmt.Errorf("create form file: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("copy file: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close writer: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, uploadURL, &body)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("upload status=%d body=%s", resp.StatusCode, string(b))
	}

	return nil
}

func probeLoop(ctx context.Context, client *http.Client, every time.Duration, st *stats, urls ...string) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()

	probe := func(target string) {
		start := time.Now()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			st.add(0, err)
			return
		}

		resp, err := client.Do(req)
		if err != nil {
			st.add(time.Since(start), err)
			return
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		if resp.StatusCode >= 500 {
			st.add(time.Since(start), fmt.Errorf("status=%d", resp.StatusCode))
			return
		}
		st.add(time.Since(start), nil)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, u := range urls {
				probe(u)
			}
		}
	}
}

func fileExt(path string) string {
	idx := strings.LastIndex(path, ".")
	if idx <= 0 || idx == len(path)-1 {
		return ".mp4"
	}
	return path[idx:]
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
