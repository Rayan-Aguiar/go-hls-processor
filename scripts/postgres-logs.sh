#!/bin/bash
set -euo pipefail

echo "📋 Logs do PostgreSQL:"
docker compose logs -f postgres
