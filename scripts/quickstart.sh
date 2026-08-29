#!/usr/bin/env sh
# Create a safe local feedr setup and deploy the daemon.
set -eu

: "${FEEDR_BLUESKY_IDENTIFIER:?Set FEEDR_BLUESKY_IDENTIFIER to your Bluesky handle or email.}"
: "${FEEDR_BLUESKY_APP_PASSWORD:?Set FEEDR_BLUESKY_APP_PASSWORD to a Bluesky app password.}"

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_dir=$(CDPATH= cd -- "$script_dir/.." && pwd)
feedr_data_dir=${FEEDR_DATA_DIR:-"$HOME/.feedr"}
today=$(date +%Y_%m_%d)

export FEEDR_DATA_DIR="$feedr_data_dir"
export FEEDR_UID=${FEEDR_UID:-"$(id -u)"}
export FEEDR_GID=${FEEDR_GID:-"$(id -g)"}

mkdir -p "$FEEDR_DATA_DIR/$today"

# Preserve a user's generated content and publisher configuration on reruns.
if [ ! -f "$FEEDR_DATA_DIR/config.json" ]; then
  cp "$project_dir/examples/config.json" "$FEEDR_DATA_DIR/config.json"
  printf '%s\n' "Created $FEEDR_DATA_DIR/config.json"
fi
if [ ! -f "$FEEDR_DATA_DIR/$today/posts.json" ]; then
  cp "$project_dir/examples/posts.json" "$FEEDR_DATA_DIR/$today/posts.json"
  printf '%s\n' "Created $FEEDR_DATA_DIR/$today/posts.json"
fi

# Stop first so a deploy has an explicit, predictable handoff. `up` then
# rebuilds and force-recreates the service with the current image and config.
docker compose -f "$project_dir/compose.yaml" stop feedr || true
docker compose -f "$project_dir/compose.yaml" up --build --force-recreate --detach feedr
docker compose -f "$project_dir/compose.yaml" ps feedr
