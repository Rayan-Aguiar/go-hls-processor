#!/bin/bash

echo "🚀 Iniciando Redis..."
docker-compose up -d redis

echo "⏳ Aguardando Redis ficar pronto..."
sleep 3

echo "✅ Verificando conexão com Redis..."
docker-compose exec redis redis-cli -a videoproc2024 ping

if [ $? -eq 0 ]; then
  echo "✅ Redis está rodando e acessível!"
  echo ""
  echo "Informações de conexão:"
  echo "  Host: localhost"
  echo "  Port: 6379"
  echo "  Password: videoproc2024"
  echo "  DB: 0"
else
  echo "❌ Falha ao conectar com Redis"
  exit 1
fi