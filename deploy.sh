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

if ! command -v curl >/dev/null 2>&1; then
    echo "curl is required to verify the deployed ExpenseOwl version." >&2
    exit 1
fi

cd "$SCRIPT_DIR"
mkdir -p "$RUNTIME_DIR"

build_version="dev"
if command -v git >/dev/null 2>&1; then
    build_version="$(git rev-parse --short HEAD 2>/dev/null || printf 'dev')"
fi

systemd_service=""
service_binary=""
if command -v systemctl >/dev/null 2>&1 && systemctl cat expenseowl.service >/dev/null 2>&1; then
    systemd_service="expenseowl.service"
    service_binary="$(systemctl show "$systemd_service" --property=ExecStart --value \
        | sed -n 's/.*path=\([^ ;]*\).*/\1/p')"
    if [[ -z "$service_binary" || "$service_binary" != /* ]]; then
        echo "Could not determine ExecStart for $systemd_service." >&2
        exit 1
    fi
fi

echo "Building ExpenseOwl $build_version..."
go build -ldflags "-X main.version=$build_version" -o "$NEW_BINARY_PATH" ./cmd/expenseowl

if [[ -n "$systemd_service" ]]; then
    echo "Deploying to $service_binary managed by $systemd_service..."
    systemctl stop "$systemd_service"
    install -m 0755 "$NEW_BINARY_PATH" "$service_binary"
    rm -f "$NEW_BINARY_PATH" "$PID_PATH"
    systemctl start "$systemd_service"
    app_pid="$(systemctl show "$systemd_service" --property=MainPID --value)"
elif [[ -f "$PID_PATH" ]]; then
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

if [[ -z "$systemd_service" ]]; then
    mv "$NEW_BINARY_PATH" "$BINARY_PATH"

    echo "Starting ExpenseOwl..."
    nohup "$BINARY_PATH" -port "$HOST_PORT" >"$LOG_PATH" 2>&1 &
    app_pid=$!
    printf '%s\n' "$app_pid" >"$PID_PATH"
fi

ready=false
deployed_version=""
for _ in {1..20}; do
    if [[ -n "$systemd_service" ]]; then
        if ! systemctl is-active --quiet "$systemd_service"; then
            break
        fi
    elif ! kill -0 "$app_pid" 2>/dev/null; then
        break
    fi
    deployed_version="$(curl --fail --silent "http://127.0.0.1:${HOST_PORT}/version" 2>/dev/null || true)"
    if [[ "$deployed_version" == "$build_version" ]]; then
        # Give bind failures time to terminate before accepting a response that
        # could have come from an older process already using this port.
        sleep 0.25
        if { [[ -n "$systemd_service" ]] && systemctl is-active --quiet "$systemd_service"; } \
            || { [[ -z "$systemd_service" ]] && kill -0 "$app_pid" 2>/dev/null; }; then
            ready=true
            break
        fi
    fi
    sleep 0.25
done

if [[ "$ready" != "true" ]]; then
    rm -f "$PID_PATH"
    echo "ExpenseOwl failed to become ready as version $build_version." >&2
    if [[ -n "$deployed_version" ]]; then
        echo "Port $HOST_PORT responded with version $deployed_version; another process may already be using it." >&2
    fi
    echo "Application log:" >&2
    if [[ -n "$systemd_service" ]]; then
        journalctl --unit "$systemd_service" --lines 30 --no-pager >&2 || true
    else
        tail -n 30 "$LOG_PATH" >&2 || true
    fi
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
echo "Version: $build_version"
echo "PID: $app_pid"
if [[ -n "$systemd_service" ]]; then
    echo "Service: $systemd_service"
else
    echo "Log: $LOG_PATH"
fi
echo "Local IP: $local_ip"
echo "Port: $HOST_PORT"
echo "URL: http://${local_ip}:${HOST_PORT}"
