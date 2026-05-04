# UploadVideo (estudo em Go)

## Objetivo

Projeto de estudo para aprender Go na prática, focado em backend: upload de vídeo, processamento com `ffmpeg` (HLS), geração de thumbnails, organização de saída em `/tmp` e persistência de metadados em SQLite.

## Visão geral

Fluxo básico:
- Cliente envia um arquivo de vídeo para a API.
- A API salva o arquivo temporariamente e enfileira um job de processamento.
- Um worker executa `ffmpeg` para gerar múltiplas qualidades (360p → 1080p), HLS (`.m3u8` + segments) e thumbnails.
- Os artefatos são organizados em `/tmp/<job-id>/...` e os caminhos são salvos em um banco SQLite.

## Principais funcionalidades

- Upload de vídeo via HTTP (multipart)
- Geração de HLS em múltiplas qualidades
- Geração de thumbnails (configurável: 1 ou várias)
- Organização de arquivos em `/tmp` por job
- Persistência de metadados (SQLite)
- Processamento concorrente com worker pool

## Estrutura proposta (exemplo)

- `cmd/` — binários/executáveis
- `internal/handler` — handlers HTTP
- `internal/service` — lógica de orquestração (jobs)
- `internal/ffmpeg` — abstração para chamar ffmpeg
- `tmp/` — diretório base para saídas (configurável)
- `migrations/` — scripts SQLite (opcional)

## Etapas de desenvolvimento (visão rápida)

1. Organização do repositório e README/TODO
2. Endpoint de upload + validações (tamanho, tipo)
3. Estrutura mínima de job + persistência SQLite
4. Wrapper `ffmpeg` para gerar HLS e thumbnails
5. Worker pool para processar uploads de forma concorrente
6. Testes automatizados e scripts locais (ex.: `docker run` do ffmpeg)
7. Documentação e checklist de segurança

## Boas práticas e decisões técnicas

- Tratar `ffmpeg` como processo externo (não biblioteca), controlar via `context.Context`.
- Validar uploads (MIME type, tamanho máximo) antes de enfileirar o job.
- Escrever arquivos de forma atômica e usar diretórios temporários por job.
- Evitar armazenar arquivos binários no banco; guardar apenas metadados/paths.
- Usar um worker pool com limite de concorrência e retry/backoff em falhas.
- Registrar logs estruturados para debugar jobs (job-id em logs).

## Próximos passos sugeridos

- Concorda com a estrutura proposta? Se sim, posso gerar o esqueleto de pastas (`cmd/`, `internal/...`) e exemplos de arquivos de configuração.
