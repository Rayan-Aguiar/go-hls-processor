#!/bin/bash
set -euo pipefail

echo "🔧 Abrindo psql no container..."
docker compose exec postgres psql -U videoproc -d video_processor
