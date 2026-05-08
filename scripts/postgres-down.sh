#!/bin/bash
set -euo pipefail

echo "🛑 Parando PostgreSQL..."
docker compose stop postgres

echo "✅ PostgreSQL parado"
