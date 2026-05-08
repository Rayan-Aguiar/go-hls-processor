# Go HLS Processor (estudo em Go)

## Objetivo
Projeto de estudo focado em backend com Go para pipeline de video:
- upload de arquivo
- enfileiramento assincrono
- processamento FFmpeg (HLS multiqualidade)
- geracao de thumbnail
- status e artefatos servidos por API HTTP
- dashboard web para acompanhar processamento e assistir videos

## Estado atual (implementado)
- Upload multipart com validacao e persistencia de job no PostgreSQL.
- Fila Redis com produtor e consumidor desacoplados por interface.
- Worker pool com concorrencia configuravel.
- Retry com backoff exponencial + dead-letter queue.
- Recovery loop para jobs presos em `processing` e tambem jobs orfaos em `pending`.
- Conversao HLS em 4 qualidades: 360p, 480p, 720p, 1080p.
- Geracao de thumbnail JPG por job.
- Progresso em tempo real via SSE + Redis Pub/Sub.
- Dashboard web com upload, listagem, progresso e player HLS com seletor de qualidade.
- Script de desenvolvimento unico (`./dev.sh`) subindo server + worker com logs prefixados.
- Stack de observabilidade com Prometheus + Grafana + exporters de PostgreSQL/Redis.
- Dashboards provisionados automaticamente para fila, jobs, API e worker.

## Arquitetura resumida
- PostgreSQL: fonte da verdade dos jobs (status, paths, timestamps).
- Redis:
  - fila principal (`LIST`)
  - fila de retry com atraso (`ZSET`)
  - dead-letter queue (`LIST`)
  - pub/sub de progresso (`video:progress:{jobID}`)
- Worker:
  - dispatcher bloqueante
  - pool concorrente
  - timeout por job
  - retry e recovery
- HTTP Server:
  - upload, status, listagem e streaming de assets HLS
  - endpoint SSE por job
  - dashboard web

## Fluxo ponta a ponta
1. Cliente envia arquivo em `POST /jobs/upload`.
2. API valida arquivo, salva em disco, cria job `pending` no PostgreSQL e publica na fila Redis.
3. Worker consome a fila, muda status para `processing` e roda FFmpeg por qualidade.
4. Worker publica progresso (5, 20, 40, 60, 80, 90, 100).
5. Frontend recebe via SSE (`/jobs/{id}/events`) e atualiza barra/etapa em tempo real.
6. Ao concluir, status vira `completed`, thumbnail e playlists ficam disponiveis.

## Endpoints HTTP
- `GET /`
  - Dashboard web.
- `GET /health`
  - Healthcheck basico.
- `POST /jobs/upload`
  - Upload multipart (`file`).
- `GET /jobs/{id}`
  - Status de um job.
- `GET /jobs/{id}/events`
  - SSE de progresso do job.
- `GET /videos?limit=200`
  - Lista jobs/videos.
- `GET /videos/{id}/{asset...}`
  - Serve `master.m3u8`, playlists por qualidade, `.ts` e `thumbnail.jpg`.
- `GET /metrics`
  - Metrica Prometheus da API.

## Documentação da API

### Swagger UI
- Acesso: `http://localhost:8000/swagger`
- Arquivo JSON bruto: `http://localhost:8000/swagger.json`
- Documentação interativa de todos os endpoints com parâmetros, respostas e exemplos

## Observabilidade
### Componentes
- Prometheus (`http://localhost:9090`)
- Grafana (`http://localhost:3000`)
- PostgreSQL Exporter (`http://localhost:9187/metrics`)
- Redis Exporter (`http://localhost:9121/metrics`)

### Endpoints de metricas da aplicacao
- API: `http://localhost:8000/metrics`
- Worker: `http://localhost:9101/metrics`

### Dashboard
- Dashboard provisionado: `Video Processor Overview`
- Caminho no repo: `observability/grafana/dashboards/video-processor-overview.json`

### O que monitorar (prioridade)
- Backlog e saude da fila:
  - `videoproc_queue_depth` (main/retry/dead)
  - `videoproc_recovery_reenqueued_total`
  - `videoproc_retry_promoted_total`
- Estado dos jobs:
  - `videoproc_jobs_total{status=...}` para `pending`, `processing`, `completed`, `failed`
- Performance do worker:
  - `videoproc_job_processing_duration_seconds` (p50/p95)
  - `videoproc_jobs_processed_total{result=...}`
  - `videoproc_worker_active_jobs`
  - `videoproc_retry_scheduled_total` e `videoproc_dead_letter_total`
- Saude da API:
  - `videoproc_http_request_duration_seconds` (p95)
  - `videoproc_http_requests_total` (throughput e 5xx)
- Infra de suporte:
  - métricas nativas do PostgreSQL exporter
  - métricas nativas do Redis exporter

## Frontend dashboard
Arquivo: `web/index.html`

Funcionalidades:
- Upload de video.
- Lista de videos com status (`pending`, `processing`, `completed`, `failed`).
- Barra de progresso e descricao de etapa.
- Atualizacao por SSE (sem polling fixo).
- Player com hls.js priorizado para troca manual de qualidade.
- Fallback para HLS nativo quando hls.js nao estiver disponivel.
- Reconexao automatica de SSE em queda transitora.

## Execucao local
### Pre-requisitos
- Go 1.26+
- Docker + Docker Compose
- FFmpeg instalado na maquina
- Air instalado (`go install github.com/air-verse/air@latest`)

### Subir infraestrutura
```bash
docker compose up -d
```

Isso sobe PostgreSQL, Redis, Prometheus, Grafana e exporters.

### Aplicar migrations
```bash
go run ./cmd/migrate
```

### Subir server + worker juntos (recomendado)
```bash
./dev.sh
```

### Alternativa (manual)
Terminal 1:
```bash
go run ./cmd/server
```

Terminal 2:
```bash
go run ./cmd/worker
```

### Rodar testes
```bash
go test ./... -count=1
```

### Rodar teste de carga (responsividade do servidor)
Use um video real para que o worker faca processamento de verdade durante a carga.

Exemplo:
```bash
go run ./cmd/loadtest \
  -base-url http://localhost:8000 \
  -file /caminho/para/video.mp4 \
  -uploads 120 \
  -concurrency 16 \
  -probe-every 500ms \
  -settle-after 60s \
  -max-p95 400ms \
  -max-error-rate 0.02
```

O comando faz:
- carga concorrente em `POST /jobs/upload`
- probes continuos em `GET /health` e `GET /videos`
- calculo de p50/p95/p99 e taxa de erro
- resultado final `PASS`/`FAIL` com base nos criterios configurados

## `dev.sh` (melhorias recentes)
- Sobe server e worker juntos.
- Prefixa logs por processo:
  - `[server]`
  - `[worker]`
- Falha rapida se a porta 8000 estiver ocupada.
- Encerra os dois processos juntos ao sair.
- Se um processo cair, o script encerra o outro e finaliza.

## Variaveis de ambiente importantes
Arquivo base: `.env`

- API:
  - `PORT`
  - `DATA_DIR`
  - `DATABASE_URL`
- Redis:
  - `REDIS_HOST`
  - `REDIS_PORT`
  - `REDIS_PASSWORD`
  - `REDIS_DB`
- Fila/worker:
  - `QUEUE_NAME`
  - `WORKER_POOL_SIZE`
  - `WORKER_TIMEOUT_MINUTES`
  - `MAX_RETRIES`
  - `RETRY_BACKOFF_SECONDS`
  - `RETRY_BACKOFF_MAX_SECONDS`
  - `RETRY_QUEUE`
  - `DEAD_LETTER_QUEUE`
  - `RETRY_SWEEP_INTERVAL_SECONDS`
- Recovery:
  - `RECOVERY_SWEEP_INTERVAL_SECONDS` (atual: 5)
  - `RECOVERY_STUCK_AFTER_SECONDS`
  - `RECOVERY_BATCH_SIZE`
- Observabilidade:
  - `WORKER_METRICS_PORT`
  - `METRICS_SAMPLE_INTERVAL_SECONDS`
  - `PROMETHEUS_PORT`
  - `GRAFANA_PORT`
  - `POSTGRES_EXPORTER_PORT`
  - `REDIS_EXPORTER_PORT`

## Troubleshooting rapido
### `erro: porta 8000 ja esta em uso`
Use:
```bash
ss -ltnp '( sport = :8000 )'
```
Mate o PID e suba novamente `./dev.sh`.

### Job fica em `pending`
- Verifique se worker esta rodando.
- Com o recovery atual, jobs orfaos em `pending` sao reenfileirados automaticamente quando a fila principal estiver vazia (evita duplicacao sob backlog alto).

### Barra de progresso para antes do fim
- SSE agora possui reconexao automatica no cliente.
- O servidor envia `retry` para reconexao rapida.

### Nao consigo trocar qualidade no player
- O frontend prioriza hls.js para permitir troca manual.
- Se cair no modo nativo, o seletor pode ficar indisponivel dependendo do navegador.

### Como validar "servidor responsivo sob carga"
- Suba o ambiente completo com `./dev.sh`.
- Rode `go run ./cmd/loadtest ...` com arquivo de video real.
- Considere aprovado quando o teste terminar com `PASS` (p95 e error_rate dentro dos limites definidos).

### Playbook de troubleshooting (fila parada, jobs stuck, saturacao)
- Fila parada (`pending` cresce e `processing` nao sobe):
  - Verifique painel `Profundidade das Filas Redis` no Grafana.
  - Se `videoproc_queue_depth{queue="video:jobs"}` sobe continuamente e `videoproc_worker_active_jobs` fica em zero, confirme worker ativo e endpoint `http://localhost:9101/metrics`.
- Jobs stuck em `processing`:
  - Observe `videoproc_jobs_total{status="processing"}` sem queda por janela longa.
  - Verifique aumento em `videoproc_recovery_reenqueued_total` e `videoproc_recovery_runs_total` para validar atuação do recovery.
- Saturacao do worker:
  - `videoproc_worker_active_jobs` constante no teto e p95 de `videoproc_job_processing_duration_seconds` crescente.
  - Acoes: reduzir concorrencia de upload, aumentar `WORKER_POOL_SIZE` com cautela e acompanhar latencia da API.
- Saturacao da API:
  - p95 de `videoproc_http_request_duration_seconds` sobe e taxa 5xx aumenta.
  - Acoes: checar recursos de host, backlog da fila e gargalo de banco/redis via exporters.

## Proximos passos sugeridos
- Observabilidade (metricas de fila, retries, tempos de processamento).
- Endpoint/admin para visualizar DLQ e reenfileirar manualmente.
- Testes de integracao end-to-end do fluxo upload -> completed.
- Limpeza/retencao de artefatos antigos no `data/`.
- Hardening (auth, limites por usuario, rate limit, quotas).
