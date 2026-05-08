package observability

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rayan-aguiar/video-processor/internal/db"
	"github.com/rayan-aguiar/video-processor/internal/queue"
)

var (
	HTTPInflightRequests = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "videoproc_http_inflight_requests",
		Help: "Quantidade de requests HTTP em andamento.",
	})

	HTTPRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "videoproc_http_requests_total",
		Help: "Total de requests HTTP por metodo, rota e status.",
	}, []string{"method", "route", "status"})

	HTTPRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "videoproc_http_request_duration_seconds",
		Help:    "Latencia de requests HTTP por metodo, rota e status.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route", "status"})

	JobsByStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "videoproc_jobs_total",
		Help: "Quantidade de jobs por status no banco.",
	}, []string{"status"})

	QueueDepth = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "videoproc_queue_depth",
		Help: "Tamanho das filas Redis relevantes para processamento.",
	}, []string{"queue"})

	JobProcessingDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "videoproc_job_processing_duration_seconds",
		Help:    "Tempo de processamento de um job no worker.",
		Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120, 300, 600, 1200, 1800},
	})

	JobsProcessedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "videoproc_jobs_processed_total",
		Help: "Total de jobs processados por resultado.",
	}, []string{"result"})

	WorkerActiveJobs = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "videoproc_worker_active_jobs",
		Help: "Quantidade de jobs sendo processados simultaneamente pelos workers.",
	})

	RetryScheduledTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "videoproc_retry_scheduled_total",
		Help: "Total de jobs agendados para retry.",
	})

	RetryPromotedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "videoproc_retry_promoted_total",
		Help: "Total de jobs movidos da fila de retry para a fila principal.",
	})

	DeadLetterTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "videoproc_dead_letter_total",
		Help: "Total de jobs enviados para dead-letter.",
	})

	RecoveryReenqueuedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "videoproc_recovery_reenqueued_total",
		Help: "Total de reenfileiramentos executados pelo recovery por origem.",
	}, []string{"source"})

	RecoveryRunsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "videoproc_recovery_runs_total",
		Help: "Total de execucoes do recovery por resultado.",
	}, []string{"result"})
)

func init() {
	prometheus.MustRegister(
		HTTPInflightRequests,
		HTTPRequestsTotal,
		HTTPRequestDuration,
		JobsByStatus,
		QueueDepth,
		JobProcessingDuration,
		JobsProcessedTotal,
		WorkerActiveJobs,
		RetryScheduledTotal,
		RetryPromotedTotal,
		DeadLetterTotal,
		RecoveryReenqueuedTotal,
		RecoveryRunsTotal,
	)
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HTTPInflightRequests.Inc()
		defer HTTPInflightRequests.Dec()

		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rec, r)

		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		status := strconv.Itoa(rec.statusCode)
		labels := []string{r.Method, route, status}
		HTTPRequestsTotal.WithLabelValues(labels...).Inc()
		HTTPRequestDuration.WithLabelValues(labels...).Observe(time.Since(start).Seconds())
	})
}

func ObserveWorkerJobStart() {
	WorkerActiveJobs.Inc()
}

func ObserveWorkerJobDone(duration time.Duration, success bool) {
	if duration > 0 {
		JobProcessingDuration.Observe(duration.Seconds())
	}
	if success {
		JobsProcessedTotal.WithLabelValues("success").Inc()
	} else {
		JobsProcessedTotal.WithLabelValues("failed").Inc()
	}
	WorkerActiveJobs.Dec()
}

func ObserveRetryScheduled() {
	RetryScheduledTotal.Inc()
}

func ObserveRetryPromoted(moved int64) {
	if moved > 0 {
		RetryPromotedTotal.Add(float64(moved))
	}
}

func ObserveDeadLettered() {
	DeadLetterTotal.Inc()
}

func ObserveRecoveryRun(ok bool) {
	if ok {
		RecoveryRunsTotal.WithLabelValues("success").Inc()
		return
	}
	RecoveryRunsTotal.WithLabelValues("error").Inc()
}

func ObserveRecoveryReenqueue(source string) {
	RecoveryReenqueuedTotal.WithLabelValues(source).Inc()
}

func StartStateSampler(ctx context.Context, sqlDB *sql.DB, adapter queue.Adapter, queueNames []string, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Second
	}

	setZeroStatuses()
	setZeroQueues(queueNames)
	sampleState(ctx, sqlDB, adapter, queueNames)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sampleState(ctx, sqlDB, adapter, queueNames)
		}
	}
}

func setZeroStatuses() {
	for _, s := range []string{"pending", "processing", "completed", "failed"} {
		JobsByStatus.WithLabelValues(s).Set(0)
	}
}

func setZeroQueues(queueNames []string) {
	for _, q := range queueNames {
		if q == "" {
			continue
		}
		QueueDepth.WithLabelValues(q).Set(0)
	}
}

func sampleState(ctx context.Context, sqlDB *sql.DB, adapter queue.Adapter, queueNames []string) {
	if sqlDB != nil {
		counts, err := db.GetJobStatusCounts(ctx, sqlDB)
		if err != nil {
			log.Printf("observability: erro ao coletar status de jobs: %v", err)
		} else {
			setZeroStatuses()
			for status, count := range counts {
				JobsByStatus.WithLabelValues(status).Set(float64(count))
			}
		}
	}

	if adapter != nil {
		for _, q := range queueNames {
			if q == "" {
				continue
			}
			n, err := adapter.Len(ctx, q)
			if err != nil {
				log.Printf("observability: erro ao coletar tamanho da fila %s: %v", q, err)
				continue
			}
			QueueDepth.WithLabelValues(q).Set(float64(n))
		}
	}
}
