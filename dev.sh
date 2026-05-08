#!/bin/bash
set -euo pipefail

export PATH="$PATH:$(go env GOPATH)/bin"

if ! command -v air >/dev/null 2>&1; then
	echo "erro: air não encontrado no PATH"
	exit 1
fi

if ss -ltn "( sport = :8000 )" | grep -q ":8000"; then
	echo "erro: porta 8000 já está em uso."
	echo "dica: pare o processo atual da porta 8000 ou troque a porta do server antes de iniciar o dev.sh"
	ss -ltnp "( sport = :8000 )" || true
	exit 1
fi

run_with_prefix() {
	local prefix="$1"
	local pid_var="$2"
	shift 2

	"$@" \
		> >(sed -u "s/^/[$prefix] /") \
		2> >(sed -u "s/^/[$prefix][err] /" >&2) &

	local pid=$!
	printf -v "$pid_var" "%s" "$pid"
}

SERVER_PID=""
WORKER_PID=""
run_with_prefix "server" SERVER_PID air -c .air.toml
run_with_prefix "worker" WORKER_PID air -c .air.worker.toml

echo "server pid=$SERVER_PID"
echo "worker pid=$WORKER_PID"

cleanup() {
	echo "encerrando server e worker..."
	kill "$SERVER_PID" "$WORKER_PID" 2>/dev/null || true
	wait "$SERVER_PID" "$WORKER_PID" 2>/dev/null || true
}

trap cleanup INT TERM EXIT

# Falha rápida: se server ou worker cair, encerra o outro e termina o script.
set +e
wait -n "$SERVER_PID" "$WORKER_PID"
status=$?
set -e

echo "um dos processos finalizou (status=$status), encerrando ambiente..."
exit "$status"
