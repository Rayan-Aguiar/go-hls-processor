# Roadmap / TODO inicial

1. [x] Criar README e TODO inicial
   - Critério: documentação básica do projeto e roadmap disponível.

2. [x] Estruturar projeto
   - Criar pastas: `cmd/`, `internal/handler`, `internal/service`, `internal/ffmpeg`, `tmp/`.
   - Critério: layout de pastas criado e commit inicial.

3. [ ] Endpoint de upload
   - Receber arquivo via multipart, validar tipo e tamanho.
   - Critério: upload salva arquivo temporário e retorna job-id.
   - Status atual: logica de validacao + salvamento + criacao de job no SQLite implementada em service; falta expor via endpoint HTTP.

4. [x] Job e persistência mínima
   - Modelar entidade `Job` e persistir metadados em SQLite.
   - Critério: existir registro de job com status e paths vazios inicialmente.

5. [ ] Wrapper `ffmpeg` e geração HLS
   - Compor comandos `ffmpeg` para gerar HLS em múltiplas qualidades.
   - Critério: gerar `.m3u8` e segments para pelo menos 2 qualidades localmente.

6. [ ] Geração de thumbnails
   - Criar função para extrair thumbnails (configurável: tempo/quantidade).
   - Critério: thumbnail(s) gerado(s) e referenciados no SQLite.

7. [ ] Worker pool e concorrência
   - Implementar workers que consumam jobs e respeitem limites de concorrência.
   - Critério: processar N uploads em paralelo sem saturar CPU/memória.

8. [ ] Testes e scripts locais
   - Testes unitários e scripts para rodar `ffmpeg` localmente (ou container).
   - Critério: pipeline de testes básicos passando localmente.

9. [ ] Documentação e hardening
   - Documentar configurações, limits, cleanup e handling de falhas.
   - Critério: checklist de segurança e operação disponível.

Notas:
- Estimativas de complexidade podem ser adicionadas por tarefa.
- Priorizar segurança de uploads (validação e limites) antes do processamento em massa.
