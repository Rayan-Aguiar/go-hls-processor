# UploadVideo (estudo em Go)

## Objetivo

Projeto de estudo para aprender Go na pratica, focado em backend: upload de video, processamento com ffmpeg (HLS), geracao de thumbnails, organizacao de saida por job e persistencia de metadados em PostgreSQL.

## Visão geral

Fluxo implementado atualmente:
- A camada de upload valida arquivo, salva em disco e persiste o job no PostgreSQL.
- O job é enfileirado no Redis para processamento assíncrono.
- Um worker pool consome a fila e executa ffmpeg para HLS (360p → 1080p) e thumbnail.
- Os artefatos ficam organizados por job e os caminhos/status são atualizados no PostgreSQL.
- Em falhas, o worker aplica retry com backoff, dead-letter e recuperação de jobs presos em processing.

Observação:
- O endpoint HTTP de upload ainda não foi exposto no servidor; hoje o server possui healthcheck e ping de banco.

## Principais funcionalidades

- Upload service (validação, persistência e enqueue)
- Geracao de HLS em multiplas qualidades
- Geracao de thumbnails
- Persistencia de metadados em PostgreSQL
- Fila Redis para distribuicao dos jobs
- Worker pool concorrente para processamento assincrono
- Retry com backoff + dead-letter queue
- Recovery loop para jobs presos em processing

## Stack atual

- Go 1.26
- PostgreSQL 16 (container)
- Redis 7 (container)
- FFmpeg (instalado localmente)

## Estrutura principal

- `cmd/server` - processo HTTP
- `cmd/migrate` - aplicacao das migrations
- `cmd/worker` - consumidor da fila e pool de workers
- `internal/queue` - adapter Redis + producer
- `internal/worker` - dispatcher + worker pool
- `internal/service` - upload e processamento
- `internal/db` - conexao e repositorios
- `migrations` - SQL de schema

## Estado atual do projeto

- PostgreSQL como fonte da verdade de jobs (status e paths)
- Redis como fila principal + retry delayed (ZSET) + dead-letter
- Upload service implementado (validação, salvamento, persistência e enqueue com rollback)
- Dispatcher + worker pool com concorrência limitada e timeout por job
- Processamento HLS + thumbnail com atualização de status (pending/processing/completed/failed)
- Recovery de jobs stuck em processing após reinício/falha de worker
- Testes unitários passando para queue, worker, service, validator e ffmpeg

## Como rodar local

1. Subir infraestrutura:
	- docker compose up -d
2. Aplicar migrations:
	- go run ./cmd/migrate
3. Subir servidor:
	- go run ./cmd/server
4. Subir worker:
	- go run ./cmd/worker
5. Rodar testes:
	- go test ./... -count=1 -v

## Proximos passos

- Expor endpoint HTTP de upload (multipart) no server
- Criar endpoint para consulta de status do job
- Definir idempotência por job_id no processamento
- Implementar observabilidade (métricas e logs operacionais)
- Criar playbook de troubleshooting para operação local
