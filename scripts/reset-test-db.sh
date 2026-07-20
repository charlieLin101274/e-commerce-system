#!/bin/sh
set -eu

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

docker compose \
  -p ecommerce-integration \
  -f "$repo_dir/docker-compose.yaml" \
  -f "$repo_dir/integration-tests/compose.override.yaml" \
  down -v --remove-orphans

docker compose \
  -p ecommerce-integration \
  -f "$repo_dir/docker-compose.yaml" \
  -f "$repo_dir/integration-tests/compose.override.yaml" \
  up -d --build
