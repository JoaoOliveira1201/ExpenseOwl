#!/usr/bin/env bash

set -Eeuo pipefail

readonly SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
readonly IMAGE_NAME="expenseowl:local"
readonly CONTAINER_NAME="expenseowl"
readonly VOLUME_NAME="expenseowl"
readonly HOST_PORT="${1:-${EXPENSEOWL_PORT:-8080}}"

if [[ ! "$HOST_PORT" =~ ^[0-9]+$ ]] || ((HOST_PORT < 1 || HOST_PORT > 65535)); then
    echo "Usage: $0 [port]" >&2
    echo "The port must be a number between 1 and 65535." >&2
    exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
    echo "Docker is required but was not found in PATH." >&2
    exit 1
fi

if ! docker info >/dev/null 2>&1; then
    echo "Docker is installed, but the Docker daemon is not available." >&2
    exit 1
fi

cd "$SCRIPT_DIR"

echo "Building ExpenseOwl..."
docker build --tag "$IMAGE_NAME" .

if docker container inspect "$CONTAINER_NAME" >/dev/null 2>&1; then
    echo "Replacing the existing ExpenseOwl container..."
    docker container rm --force "$CONTAINER_NAME" >/dev/null
fi

echo "Starting ExpenseOwl..."
docker run \
    --detach \
    --restart unless-stopped \
    --name "$CONTAINER_NAME" \
    --publish "${HOST_PORT}:8080" \
    --volume "${VOLUME_NAME}:/app/data" \
    "$IMAGE_NAME" >/dev/null

sleep 1
if [[ "$(docker container inspect --format '{{.State.Running}}' "$CONTAINER_NAME")" != "true" ]]; then
    echo "ExpenseOwl failed to start. Container logs:" >&2
    docker container logs "$CONTAINER_NAME" >&2
    exit 1
fi

local_ip=""
if command -v ip >/dev/null 2>&1; then
    local_ip="$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for (i = 1; i <= NF; i++) if ($i == "src") {print $(i + 1); exit}}' || true)"
fi
if [[ -z "$local_ip" ]] && command -v hostname >/dev/null 2>&1; then
    local_ip="$(hostname -I 2>/dev/null | awk '{print $1}' || true)"
fi
local_ip="${local_ip:-127.0.0.1}"

echo
echo "ExpenseOwl is running."
echo "Note: Android standalone PWA installation still requires an HTTPS address."
echo "Local IP: $local_ip"
echo "Port: $HOST_PORT"
echo "URL: http://${local_ip}:${HOST_PORT}"
