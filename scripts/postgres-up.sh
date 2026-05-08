#!/bin/bash
set -euo pipefail

echo "🚀 Iniciando PostgreSQL..."
docker compose up -d postgres

echo "⏳ Aguardando PostgreSQL ficar saudável..."
docker compose ps

echo "✅ PostgreSQL disponível em localhost:5432"
