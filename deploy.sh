#!/usr/bin/env bash

set -Eeuo pipefail

readonly SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
readonly RUNTIME_DIR="$SCRIPT_DIR/.expenseowl"
readonly BINARY_PATH="$RUNTIME_DIR/expenseowl"
readonly NEW_BINARY_PATH="$RUNTIME_DIR/expenseowl.new"
readonly PID_PATH="$RUNTIME_DIR/expenseowl.pid"
readonly LOG_PATH="$RUNTIME_DIR/expenseowl.log"
readonly HOST_PORT="${1:-${EXPENSEOWL_PORT:-8080}}"

if [[ ! "$HOST_PORT" =~ ^[0-9]+$ ]] || ((HOST_PORT < 1 || HOST_PORT > 65535)); then
    echo "Usage: $0 [port]" >&2
    echo "The port must be a number between 1 and 65535." >&2
    exit 1
fi

if ! command -v go >/dev/null 2>&1; then
    echo "Go is required but was not found in PATH." >&2
    exit 1
fi

cd "$SCRIPT_DIR"
mkdir -p "$RUNTIME_DIR"

echo "Building ExpenseOwl..."
go build -o "$NEW_BINARY_PATH" ./cmd/expenseowl

if [[ -f "$PID_PATH" ]]; then
    old_pid="$(<"$PID_PATH")"
    if [[ "$old_pid" =~ ^[0-9]+$ ]] && kill -0 "$old_pid" 2>/dev/null; then
        process_command="$(ps -p "$old_pid" -o command= 2>/dev/null || true)"
        if [[ "$process_command" == "$BINARY_PATH"* ]]; then
            echo "Stopping the existing ExpenseOwl process ($old_pid)..."
            kill "$old_pid"
            for _ in {1..20}; do
                if ! kill -0 "$old_pid" 2>/dev/null; then
                    break
                fi
                sleep 0.25
            done
            if kill -0 "$old_pid" 2>/dev/null; then
                echo "The existing ExpenseOwl process did not stop in time." >&2
                exit 1
            fi
        else
            echo "Ignoring stale PID file; process $old_pid is not ExpenseOwl."
        fi
    fi
fi

mv "$NEW_BINARY_PATH" "$BINARY_PATH"

echo "Starting ExpenseOwl..."
nohup "$BINARY_PATH" -port "$HOST_PORT" >"$LOG_PATH" 2>&1 &
app_pid=$!
printf '%s\n' "$app_pid" >"$PID_PATH"

ready=false
for _ in {1..20}; do
    if ! kill -0 "$app_pid" 2>/dev/null; then
        break
    fi
    if command -v curl >/dev/null 2>&1 && curl --fail --silent --output /dev/null "http://127.0.0.1:${HOST_PORT}/"; then
        ready=true
        break
    elif ! command -v curl >/dev/null 2>&1 && (exec 3<>"/dev/tcp/127.0.0.1/${HOST_PORT}") 2>/dev/null; then
        ready=true
        break
    fi
    sleep 0.25
done

if [[ "$ready" != "true" ]]; then
    echo "ExpenseOwl failed to become ready. Application log:" >&2
    tail -n 30 "$LOG_PATH" >&2 || true
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
echo "PID: $app_pid"
echo "Local IP: $local_ip"
echo "Port: $HOST_PORT"
echo "URL: http://${local_ip}:${HOST_PORT}"
echo "Log: $LOG_PATH"
