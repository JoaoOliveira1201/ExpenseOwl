#!/bin/sh
set -eu

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$project_dir"

if ! git diff --quiet || ! git diff --cached --quiet; then
    echo "Deployment stopped: the working tree has uncommitted changes." >&2
    exit 1
fi

branch=$(git branch --show-current)
if [ -z "$branch" ]; then
    echo "Deployment stopped: check out a branch before deploying." >&2
    exit 1
fi

echo "Updating $branch from origin..."
git fetch --prune origin "$branch"
git pull --ff-only origin "$branch"

echo "Building the ExpenseOwl image..."
docker compose build --pull app

echo "Starting the Compose stack..."
docker compose up -d
docker compose ps
