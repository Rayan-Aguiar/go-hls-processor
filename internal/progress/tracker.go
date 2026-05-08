package progress

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

const channelPrefix = "video:progress:"

// Update é o payload publicado a cada etapa do processamento.
type Update struct {
	JobID   string `json:"job_id"`
	Percent int    `json:"percent"`
	Stage   string `json:"stage"`
	Done    bool   `json:"done"`
	Failed  bool   `json:"failed,omitempty"`
}

// Publisher publica atualizações de progresso via Redis Pub/Sub.
// É usado pelo worker process.
type Publisher struct {
	client *redis.Client
}

func NewPublisher(client *redis.Client) *Publisher {
	return &Publisher{client: client}
}

func (p *Publisher) Report(ctx context.Context, jobID string, percent int, stage string) {
	p.publish(ctx, Update{JobID: jobID, Percent: percent, Stage: stage})
}

func (p *Publisher) Done(ctx context.Context, jobID string) {
	p.publish(ctx, Update{JobID: jobID, Percent: 100, Stage: "concluído", Done: true})
}

func (p *Publisher) Fail(ctx context.Context, jobID string, reason string) {
	p.publish(ctx, Update{
		JobID:   jobID,
		Percent: -1,
		Stage:   reason,
		Failed:  true,
		Done:    true,
	})
}

func (p *Publisher) publish(ctx context.Context, u Update) {
	b, err := json.Marshal(u)
	if err != nil {
		log.Printf("progress: marshal error: %v", err)
		return
	}

	pubCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := p.client.Publish(pubCtx, channelPrefix+u.JobID, string(b)).Err(); err != nil {
		log.Printf("progress: redis publish error for job %s: %v", u.JobID, err)
		return
	}

	log.Printf("progress: published job=%s percent=%d stage=%q done=%v failed=%v", u.JobID, u.Percent, u.Stage, u.Done, u.Failed)
}

// ServeSSE assina o canal Redis do job e faz streaming SSE para o cliente.
// Encerra quando o job termina (done=true) ou o cliente desconecta.
func ServeSSE(ctx context.Context, client *redis.Client, w http.ResponseWriter, r *http.Request, jobID string) {
	_ = ctx
	log.Printf("progress: sse connected job=%s", jobID)
	defer log.Printf("progress: sse disconnected job=%s", jobID)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming não suportado por este servidor", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Sugere ao browser reconectar rápido em caso de queda transitória.
	fmt.Fprint(w, "retry: 2000\n")

	// heartbeat inicial para confirmar conexão
	connected, _ := json.Marshal(Update{JobID: jobID, Percent: 0, Stage: "conectado", Done: false})
	fmt.Fprintf(w, "data: %s\n\n", connected)
	flusher.Flush()

	sub := client.Subscribe(r.Context(), channelPrefix+jobID)
	defer sub.Close()

	msgCh := sub.Channel()
	for {
		select {
		case <-r.Context().Done():
			return
		case msg, ok := <-msgCh:
			if !ok {
				return
			}
			log.Printf("progress: sse event job=%s payload=%s", jobID, msg.Payload)

			fmt.Fprintf(w, "data: %s\n\n", msg.Payload)
			flusher.Flush()

			var u Update
			if err := json.Unmarshal([]byte(msg.Payload), &u); err == nil && u.Done {
				return
			}
		}
	}
}
