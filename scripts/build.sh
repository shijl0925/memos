#!/bin/bash

# Usage: ./scripts/build.sh

set -e

cd "$(dirname "$0")/../"

backup_dir="$(mktemp -d)"

cleanup() {
  rm -rf ./server/dist
  cp -R "$backup_dir/dist" ./server/dist
  rm -rf "$backup_dir"
}

cp -R ./server/dist "$backup_dir/dist"
trap cleanup EXIT

echo "Start building frontend..."

(
  cd ./web
  yarn install --frozen-lockfile
  yarn build
)

rm -rf ./server/dist
cp -R ./web/dist ./server/dist

echo "Start building backend..."

go build -o ./build/memos ./bin/server/main.go

echo "Backend built!"
