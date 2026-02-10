#!/usr/bin/env bash
# Build the gdql Docker image and run tests + query. Exits 0 only if all pass.
# Run from repo root: ./scripts/docker-build-and-test.sh
set -e
cd "$(dirname "$0")/.."
echo "Building gdql image (go build, go test, gdql -f query.gdql)..."
docker build -t gdql .
echo "Build OK. Running container to run query again..."
docker run --rm gdql
echo "Done."
