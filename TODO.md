# Roadmap / TODO inicial (historico preservado)

1. [x] Criar README e TODO inicial
  - Critério: documentação básica do projeto e roadmap disponível.

2. [x] Estruturar projeto
  - Criar pastas: `cmd/`, `internal/handler`, `internal/service`, `internal/ffmpeg`, `tmp/`.
  - Critério: layout de pastas criado e commit inicial.

3. [x] Endpoint de upload
  - Receber arquivo via multipart, validar tipo e tamanho.
  - Critério: upload salva arquivo temporário e retorna job-id.
  - Status atual: endpoint HTTP exposto com upload multipart + consulta de status por job + listagem de videos + endpoint de watch + streaming de assets HLS por job.

4. [x] Job e persistência mínima
  - Modelar entidade `Job` e persistir metadados em PostgreSQL.
  - Critério: existir registro de job com status e paths vazios inicialmente.

5. [x] Wrapper `ffmpeg` e geração HLS
  - Compor comandos `ffmpeg` para gerar HLS em múltiplas qualidades.
  - Critério: gerar `.m3u8` e segments para pelo menos 2 qualidades localmente.

6. [x] Geração de thumbnails
  - Criar função para extrair thumbnails (configurável: tempo/quantidade).
  - Critério: thumbnail(s) gerado(s) e referenciados no PostgreSQL.

7. [x] Worker pool e concorrência
   - Implementar workers que consumam jobs e respeitem limites de concorrência.
   - Critério: processar N uploads em paralelo sem saturar CPU/memória.
  - Status atual: orquestração de processamento implementada (ProcessJob com atualização de status), integração de enqueue no upload concluída, consumer/dispatcher + worker pool concorrente implementados e base da fila Redis pronta (message + adapter + producer).
   - Estratégia validada:
     - PostgreSQL continua como fonte da verdade dos jobs (status, paths, tentativas).
     - Redis entra para fila/distribuição de trabalho entre workers.
     - Upload/enfileiramento não bloqueia processamento pesado no request HTTP.
     - Worker pool com limite fixo de concorrência (evitar saturar CPU/RAM).
     - Backpressure com buffer controlado para não crescer memória com picos.
   - Fases de implementação:
     - [x] Fase 1 - Infra Redis local com Docker Compose.
     - [x] Fase 2 - Contrato de fila em Go (producer/consumer desacoplados por interface).
     - [x] Fase 3 - Enfileirar job no Redis ao criar job no PostgreSQL.
     - [x] Fase 4 - Dispatcher + worker pool (concorrência limitada).
     - [x] Fase 5 - Retry com backoff + limite de tentativas + dead-letter.
     - [x] Fase 6 - Recuperação de jobs presos em processing (reaper).
   - Regras de capacidade (inicial):
     - [ ] pool size inicial: max(2, min(6, CPU/2)).
     - [ ] buffer interno: 2x ou 3x o tamanho do pool.
     - [ ] timeout por job de processamento.
     - [ ] sem ffmpeg rodando na thread da API.
   - Metas de robustez para volume (200 a 1000 jobs):
     - [x] backlog fica na fila (não em memória da API).
     - [ ] servidor continua responsivo sob carga.
     - [x] jobs falhos não travam fila inteira.
     - [x] reinício da aplicação não perde job pendente.

8. [ ] Testes e scripts locais
  - Testes unitários e scripts para rodar `ffmpeg` localmente (ou container).
  - Critério: pipeline de testes básicos passando localmente.
  - Status atual: testes unitários implementados e passando (`go test ./... -v`); falta definir script operacional para execução local de processamento com ffmpeg (ou alternativa via container).
  - Escopo adicional alinhado para fila:
     - [x] testes unitários do queue adapter (Redis).
     - [x] testes de integração do queue adapter (Redis, build tag integration).
     - [x] testes do worker pool (concorrência e throughput).
     - [x] testes de retry/backoff (falha transitória e falha definitiva).
     - [x] testes de recuperação após restart (jobs presos em processing).
     - [x] script local para subir Redis e worker (dev).

9. [ ] Documentação e hardening
  - Documentar configurações, limits, cleanup e handling de falhas.
  - Critério: checklist de segurança e operação disponível.
  - Escopo adicional alinhado para operação:
     - [x] variáveis de ambiente de fila e concorrência (pool size, retry, timeout).
     - [ ] playbook de troubleshooting (fila parada, jobs stuck, saturação).
     - [ ] estratégia de observabilidade (logs e métricas básicas de fila/worker).

10. [ ] Redis e arquitetura de fila (estudo guiado)
  - Objetivo: aprender gerenciamento de filas com Redis mantendo consistência com SQLite.
  - Tópicos:
     - [x] escolha da abordagem inicial (Redis List) e evolução futura (Redis Streams).
     - [x] contrato de mensagem do job (job_id, attempts, timestamps, metadata minima).
     - [x] semântica de ACK/NACK e requeue.
     - [x] política de dead-letter para jobs excedidos.
  - [x] estratégia de idempotência no processamento por job_id.

11. [ ] Observabilidade de fila e workers
  - Métricas mínimas:
    - [ ] jobs pending, processing, completed, failed.
    - [ ] tamanho da fila Redis.
    - [ ] tempo médio de processamento por job.
    - [ ] taxa de erro e taxa de retry.
  - Critério: conseguir identificar gargalo de CPU/fila com dados objetivos.

Notas:
- Estimativas de complexidade podem ser adicionadas por tarefa.
- Priorizar segurança de uploads (validação e limites) antes do processamento em massa.

12. [x] Migração de banco para PostgreSQL
   - Objetivo: substituir SQLite por PostgreSQL no runtime da aplicação, preparando para produção.
   - Fases:
    - [x] Infra local: container PostgreSQL no docker-compose.
    - [x] Configuração: variáveis de ambiente (`DATABASE_URL`) e ajuste de scripts locais.
    - [x] Camada de DB: conexão via driver PostgreSQL e tuning de pool.
    - [x] Repositórios: queries e placeholders compatíveis com PostgreSQL.
    - [x] Migrations: scripts/runner compatíveis com PostgreSQL.
    - [x] Comandos app: `cmd/server`, `cmd/worker` e `cmd/migrate` usando PostgreSQL.
    - [x] Validação final: migrations + testes + healthcheck com banco em container.
  - Status atual: migracao concluida e validada localmente.

13. [x] Progresso em tempo real + dashboard final
   - Objetivo: acompanhar processamento em tempo real e validar saída HLS no browser.
   - Entregas:
    - [x] endpoint SSE `GET /jobs/{id}/events` no server.
    - [x] publisher de progresso via Redis Pub/Sub no worker.
    - [x] callback de progresso por qualidade no conversor HLS.
    - [x] integração de `ProgressReporter` no `ProcessingService`.
    - [x] frontend com barra de progresso por job.
    - [x] frontend SSE-first (remoção de polling fixo).
    - [x] reconexão automática de SSE em falhas transitórias.
    - [x] player com hls.js priorizado para troca manual de qualidade.

14. [x] Operação local com um comando
   - Objetivo: subir ambiente de desenvolvimento completo sem abrir múltiplos terminais.
   - Entregas:
    - [x] `dev.sh` sobe server + worker juntos.
    - [x] logs com prefixo `[server]` e `[worker]`.
    - [x] fail-fast se a porta 8000 já estiver em uso.
    - [x] encerramento coordenado dos dois processos.
    - [x] fail-fast se um processo cair.

15. [x] Recovery expandido para jobs pending órfãos
   - Objetivo: evitar jobs presos indefinidamente em `pending` quando saem da fila.
   - Entregas:
    - [x] query de jobs `pending` antigos no repositório.
    - [x] recovery reenfileira `pending` órfão e atualiza `updated_at`.
    - [x] testes cobrindo pending stale e pending fresh.
    - [x] sweep de recovery ajustado para intervalo mais curto no dev (`RECOVERY_SWEEP_INTERVAL_SECONDS=5`).

16. [x] Idempotência do processamento por `job_id`
  - Objetivo: impedir processamento duplicado do mesmo job em cenários de retry/recovery/duplicação de mensagem.
  - Entregas:
   - [x] transição atômica para `processing` no repositório (`TryMarkJobProcessing`) somente a partir de `pending` ou `failed`.
   - [x] `ProcessingService` ignora duplicata com sucesso quando status atual já está `processing` ou `completed`.
   - [x] retries continuam válidos para jobs em `failed` (reentrada permitida).
   - [x] testes unitários cobrindo skip idempotente para jobs já `processing` e `completed`.
