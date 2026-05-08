# Video Processor

> Pipeline de video em Go com HLS adaptativo, processamento assincrono e observabilidade completa de ponta a ponta.

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-336791?logo=postgresql&logoColor=white)](https://www.postgresql.org/)
[![Redis](https://img.shields.io/badge/Redis-Queue%20%26%20Pub%2FSub-DC382D?logo=redis&logoColor=white)](https://redis.io/)
[![FFmpeg](https://img.shields.io/badge/FFmpeg-HLS%20Pipeline-000000?logo=ffmpeg&logoColor=white)](https://ffmpeg.org/)
[![Prometheus](https://img.shields.io/badge/Prometheus-Metrics-E6522C?logo=prometheus&logoColor=white)](https://prometheus.io/)
[![Grafana](https://img.shields.io/badge/Grafana-Dashboards-F46800?logo=grafana&logoColor=white)](https://grafana.com/)

## Visao Geral

Video Processor foi desenhado para transformar uploads de video em uma experiencia pronta para consumo: o arquivo entra por uma API HTTP, e em seguida segue para uma fila assincrona, onde um worker processa o material com FFmpeg, gera HLS em multiplas qualidades, cria thumbnail e disponibiliza todos os artefatos para reproducao imediata.

O valor central do projeto e entregar um fluxo completo e confiavel para processamento de midia, com separacao clara entre API, fila, worker e camada de observabilidade. O resultado e uma base tecnica robusta, preparada para crescimento, monitoramento e evolucao sem acoplamento desnecessario.

## Preview

> Pipeline de upload, processamento e playback com atualizacao em tempo real do progresso.

## Principais Funcionalidades

- Upload multipart com validacao e persistencia imediata do job no banco, reduzindo o tempo entre a acao do usuario e o inicio do processamento.
- Fila assincrona em Redis com produtor e consumidor desacoplados, permitindo absorver picos de demanda sem travar a experiencia da API.
- Worker pool com concorrencia configuravel, timeout por job e controle de buffer para manter previsibilidade operacional.
- Retry com backoff exponencial e dead-letter queue, oferecendo resiliencia diante de falhas temporarias no processamento.
- Recovery automatizado para jobs presos em processing e para casos de pending orfao, reduzindo operacao manual e evitando backlog estagnado.
- Conversao HLS em multiplas qualidades, com playlists e segmentos prontos para playback adaptativo.
- Geracao automatica de thumbnail por job, melhorando a navegacao visual na lista de videos.
- Atualizacao em tempo real via SSE e Redis Pub/Sub, eliminando polling fixo e deixando a interface mais fluida.
- Dashboard web para acompanhar status, progresso e playback em uma unica tela.
- Swagger UI e arquivo OpenAPI para exploracao rapida da API e integracao com outros sistemas.
- Stack de observabilidade com metricas da aplicacao, exporters e dashboards provisionados automaticamente.

## Diferenciais Tecnicos

| Area | Implementacao | Impacto |
| --- | --- | --- |
| Arquitetura | Separacao entre API, worker, fila e storage em disco | Facilita evolucao, manutencao e escala independente |
| Processamento | FFmpeg isolado em um pipeline assinc com timeout e retry | Mantem o servidor responsivo mesmo sob carga |
| Confiabilidade | Recovery loop para jobs stuck e orfaos | Reduz inconsistencias operacionais e perda de trabalho |
| Fluxo de dados | PostgreSQL como fonte da verdade e Redis como camada de orquestracao | Evita acoplamento e simplifica reprocessamento |
| Experiencia em tempo real | SSE com reconexao automatica | Feedback imediato sem polling agressivo |
| Observabilidade | Prometheus, Grafana e exporters para Postgres e Redis | Diagnostico rapido de gargalos e regressao de performance |
| Operacao local | `dev.sh` sobe server e worker juntos com logs prefixados | Desenvolvimento mais previsivel e ergonomico |
| Qualidade de API | Endpoints documentados e contratos bem definidos | Integracao mais simples para clientes e colaboradores |
| Frontend | Dashboard leve, responsivo e com fallback de playback | Boa experiencia mesmo em navegadores com suporte variavel |

## Stack Utilizada

| Camada | Tecnologias |
| --- | --- |
| Linguagem | Go 1.26 |
| API HTTP | `net/http`, SSE, middleware de metrics |
| Banco de dados | PostgreSQL via `pgx/v5` e `database/sql` |
| Fila e eventos | Redis + `go-redis/v9` |
| Midia | FFmpeg, HLS, geracao de thumbnail |
| Observabilidade | Prometheus, Grafana, PostgreSQL Exporter, Redis Exporter |
| Frontend | HTML, CSS, JavaScript e `hls.js` no dashboard |
| Ferramentas | Docker Compose, Air, Swagger UI, load test CLI |

## Como Rodar Localmente

### Pre-requisitos

- Go 1.26+
- Docker e Docker Compose
- FFmpeg instalado no sistema
- Air instalado para o fluxo de desenvolvimento com reload automatico

Instale o Air, se necessario:

```bash
go install github.com/air-verse/air@latest
```

### 1. Configure o ambiente

Copie o arquivo de exemplo e ajuste os valores conforme sua maquina:

```bash
cp .env.example .env
```

Campos importantes que o servidor e o worker realmente consomem:

```env
DATABASE_URL=postgres://videoproc:videoproc@localhost:5432/video_processor?sslmode=disable
DATA_DIR=./data
MIGRATIONS_DIR=./migrations
PORT=8000

REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=videoproc2024
REDIS_DB=0

QUEUE_NAME=video:jobs
RETRY_QUEUE=video:jobs:retry
DEAD_LETTER_QUEUE=video:jobs:dead
WORKER_POOL_SIZE=4
WORKER_TIMEOUT_MINUTES=30
MAX_RETRIES=3
RETRY_BACKOFF_SECONDS=5
RETRY_BACKOFF_MAX_SECONDS=300
RETRY_SWEEP_INTERVAL_SECONDS=1
RECOVERY_SWEEP_INTERVAL_SECONDS=5
RECOVERY_STUCK_AFTER_SECONDS=120
RECOVERY_BATCH_SIZE=100

WORKER_METRICS_PORT=9101
METRICS_SAMPLE_INTERVAL_SECONDS=5
```

### 2. Suba a infraestrutura

```bash
docker compose up -d
```

Esse comando sobe PostgreSQL, Redis, Prometheus, Grafana e os exporters de suporte.

### 3. Rode as migrations

```bash
go run ./cmd/migrate
```

### 4. Inicie a aplicacao

Recomendado para desenvolvimento:

```bash
./dev.sh
```

Isso inicia server e worker juntos, com logs prefixados e encerramento coordenado.

Se preferir executar manualmente:

```bash
go run ./cmd/server
```

Em outro terminal:

```bash
go run ./cmd/worker
```

### 5. Valide o sistema

Teste a API e os fluxos principais:

```bash
go test ./... -count=1
```

Execute o load test com um video real para medir responsividade sob processamento de verdade:

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

### Acesso rapido

- Aplicacao: `http://localhost:8000`
- Swagger UI: `http://localhost:8000/swagger`
- OpenAPI JSON: `http://localhost:8000/swagger.json`
- Prometheus: `http://localhost:9090`
- Grafana: `http://localhost:3000`
- Metrics da API: `http://localhost:8000/metrics`
- Metrics do worker: `http://localhost:9101/metrics`

### Problemas comuns

- Porta 8000 ocupada: verifique com `ss -ltnp '( sport = :8000 )'` e pare o processo conflitante antes de rodar `./dev.sh`.
- Worker nao processa jobs: confirme se o `ffmpeg` esta instalado e se o worker esta ativo no terminal separado ou no `./dev.sh`.
- Interface nao atualiza progresso: verifique se a conexao SSE para `/jobs/{id}/events` nao foi bloqueada por proxy, extensao ou politica de rede.
- Grafana ou Prometheus nao sobem: confirme se o Docker Compose concluiu o bootstrap e se as portas locais nao estao em uso.

## Estrutura do Projeto

```text
.
├── cmd/
│   ├── server/       # API HTTP, dashboard, SSE, Swagger e assets de video
│   ├── worker/       # Consumidor de fila, pool de processamento e metrics
│   ├── migrate/      # Aplicacao de migrations do banco
│   └── loadtest/     # Simulacao de carga e prova de responsividade
├── internal/
│   ├── db/           # Conexao, repositorios e consultas
│   ├── ffmpeg/       # Runner, HLS e thumbnail
│   ├── handler/      # Handlers HTTP
│   ├── models/       # Modelos e estados de dominio
│   ├── observability/# Metricas e instrumentacao
│   ├── progress/     # Pub/Sub e SSE de progresso
│   ├── queue/        # Adaptadores Redis, producer e mensagens
│   ├── service/      # Regras de negocio e orquestracao
│   ├── validator/    # Validacoes de upload e video
│   └── worker/       # Pool concorrente e lifecycle
├── migrations/       # Schema e evolucao do banco
├── observability/    # Prometheus, Grafana e provisioning
├── web/              # Dashboard HTML/CSS/JS
├── data/             # Artefatos gerados por job
├── docs/             # Swagger JSON e documentacao da API
└── scripts/          # Utilitarios e automacoes auxiliares
```

## API e Fluxos

### Endpoints Principais

| Metodo | Rota | Finalidade |
| --- | --- | --- |
| `GET` | `/` | Dashboard web |
| `GET` | `/health` | Healthcheck da API |
| `GET` | `/jobs/ping-db` | Valida conexao com o banco |
| `POST` | `/jobs/upload` | Upload multipart do video |
| `GET` | `/jobs/{id}` | Detalhe e status do job |
| `GET` | `/jobs/{id}/events` | Stream SSE de progresso |
| `GET` | `/videos` | Lista de jobs/videos |
| `GET` | `/videos/{id}/{asset...}` | Serve playlists, segmentos e thumbnail |
| `GET` | `/metrics` | Metrics da API para Prometheus |
| `GET` | `/swagger` | Swagger UI |
| `GET` | `/swagger.json` | Especificacao OpenAPI |

### Fluxo de Execucao

```mermaid
flowchart LR
    A[POST /jobs/upload] --> B[Validacao e persistencia]
    B --> C[(PostgreSQL: job pending)]
    B --> D[(Redis: fila principal)]
    D --> E[Worker pool]
    E --> F[FFmpeg: HLS multiqualidade]
    E --> G[Thumbnail JPG]
    E --> H[Eventos de progresso via Redis Pub/Sub]
    H --> I[GET /jobs/{id}/events SSE]
    F --> J[GET /videos/{id}/{asset...}]
    G --> J
    E --> K[(Dead-letter queue / Retry queue)]
```

### O que esse fluxo entrega

- Entrada simples por upload multipart.
- Processamento desacoplado da API, evitando bloqueio da experiencia do usuario.
- Atualizacao em tempo real do estado do job.
- Artefatos prontos para playback adaptativo e compartilhamento.

## Experiencia do Usuario

- Interface limpa e objetiva, com foco em status, progresso e consumo rapido do video.
- Feedback visual imediato para pending, processing, completed e failed.
- Atualizacao sem polling fixo, o que reduz latencia perceptivel e ruido de rede.
- Player com suporte a troca de qualidade via hls.js, com fallback quando necessario.
- Layout responsivo, pensado para telas maiores e para acompanhamento rapido em navegadores comuns.

## Possibilidades Futuras

- Persistencia dos artefatos em storage externo, como S3 ou compatibilidade similar.
- Upload resumivel para arquivos muito grandes.
- Autenticacao e controle de acesso por usuario ou workspace.
- Webhooks para notificar sistemas externos quando o job concluir.
- Pagina dedicada por video com metadados, historico e insights de processamento.
- Novos perfis de qualidade e presets customizaveis por tipo de conteudo.
- Painel operacional com filtros, busca e visao consolidada de filas e falhas.

## Conclusão

Video Processor combina engenharia pragmatica, processamento de midia e observabilidade real em uma mesma base. O projeto entrega uma experiencia completa para upload, transcodificacao, monitoramento e playback, com uma arquitetura preparada para crescer sem perder clareza tecnica.

O resultado e uma plataforma com valor real de produto: responsiva, monitoravel, resiliente e organizada para evoluir com baixo atrito.
