# UploadVideo (estudo em Go)

## Objetivo

Projeto de estudo para aprender Go na pratica, focado em backend: upload de video, processamento com ffmpeg (HLS), geracao de thumbnails, organizacao de saida por job e persistencia de metadados em PostgreSQL.

## Visão geral

Fluxo básico:
- Cliente envia um arquivo de vídeo para a API.
- A API salva o arquivo temporariamente e enfileira um job de processamento.
- Um worker executa `ffmpeg` para gerar múltiplas qualidades (360p → 1080p), HLS (`.m3u8` + segments) e thumbnails.
- Os artefatos sao organizados por job e os caminhos sao salvos em PostgreSQL.

## Principais funcionalidades

- Upload de video via HTTP (multipart)
- Geracao de HLS em multiplas qualidades
- Geracao de thumbnails
- Persistencia de metadados em PostgreSQL
- Fila Redis para distribuicao dos jobs
- Worker pool concorrente para processamento assincrono

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

- Base de upload validando e persistindo job
- Enqueue do job em Redis apos persistencia
- Dispatcher e worker pool concorrente implementados
- Processamento HLS + thumbnail implementado
- Testes unitarios e de integracao da camada de fila

## Como rodar local

1. Subir infraestrutura:
	- `docker compose up -d`
2. Aplicar migrations:
	- `go run ./cmd/migrate`
3. Subir servidor:
	- `go run ./cmd/server`
4. Subir worker:
	- `go run ./cmd/worker`

## Proximos passos

- Retry/backoff e dead-letter (fase 5)
- Recuperacao de jobs presos (fase 6)
- Observabilidade de fila e workers
- Endpoint de upload para fluxo fim a fim via HTTP
