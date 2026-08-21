#!/bin/sh
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_root"

for command_name in git docker; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    printf 'Required command is unavailable: %s\n' "$command_name" >&2
    exit 1
  fi
done

if ! docker compose version >/dev/null 2>&1; then
  printf '%s\n' 'Docker Compose v2 is required.' >&2
  exit 1
fi

if [ -n "$(git status --porcelain --untracked-files=normal)" ]; then
  printf '%s\n' 'Update refused: the repository has uncommitted or untracked changes.' >&2
  printf '%s\n' 'Commit, move, or remove those changes before retrying.' >&2
  git status --short >&2
  exit 1
fi

branch=${MYPANEL_UPDATE_BRANCH:-$(git symbolic-ref --quiet --short HEAD || true)}
case "$branch" in
  ''|-*|*[!A-Za-z0-9._/-]*|/*|*/|*..*)
    printf 'Update refused: invalid branch name %s\n' "$branch" >&2
    exit 1
    ;;
esac

if ! git remote get-url origin >/dev/null 2>&1; then
  printf '%s\n' 'Update refused: Git remote origin is not configured.' >&2
  exit 1
fi

lock_dir=${TMPDIR:-/tmp}/mypanel-update-$USER.lock
if ! mkdir "$lock_dir" 2>/dev/null; then
  printf '%s\n' 'Another MyPanel update is already running.' >&2
  exit 1
fi
trap 'rmdir "$lock_dir" 2>/dev/null || true' EXIT HUP INT TERM

printf 'Fetching origin/%s...\n' "$branch"
git fetch --prune origin "refs/heads/$branch:refs/remotes/origin/$branch"

remote_ref=refs/remotes/origin/$branch
if ! git show-ref --verify --quiet "$remote_ref"; then
  printf 'Update refused: origin/%s does not exist.\n' "$branch" >&2
  exit 1
fi
if ! git merge-base --is-ancestor HEAD "$remote_ref"; then
  printf 'Update refused: origin/%s is not a fast-forward of the current commit.\n' "$branch" >&2
  exit 1
fi

current_commit=$(git rev-parse HEAD)
target_commit=$(git rev-parse "$remote_ref")
deployment_marker=$(git rev-parse --git-path mypanel-deployed)
deployed_commit=
if [ -f "$deployment_marker" ]; then
  IFS= read -r deployed_commit < "$deployment_marker" || true
fi
if [ "$current_commit" = "$target_commit" ] && [ "$deployed_commit" = "$target_commit" ] && [ "${MYPANEL_UPDATE_FORCE:-0}" != "1" ]; then
  printf 'MyPanel is already current at %s.\n' "$(git rev-parse --short HEAD)"
  exit 0
fi

if [ "$current_commit" != "$target_commit" ]; then
  git merge --ff-only "$remote_ref"
fi

docker compose config --quiet
docker compose build --pull

# Migrations are idempotent and additive. Run them explicitly because the
# normal migrate container is a completed one-shot service after first install.
docker compose run --rm migrate
docker compose up -d --remove-orphans

attempt=0
until docker compose exec -T web wget -qO- http://127.0.0.1:8080/api/v1/health/ready >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 30 ]; then
    printf '%s\n' 'MyPanel did not become ready within 60 seconds.' >&2
    docker compose ps >&2
    docker compose logs --tail=100 controller agent web >&2
    exit 1
  fi
  sleep 2
done

umask 077
marker_tmp=$deployment_marker.tmp.$$
printf '%s\n' "$target_commit" > "$marker_tmp"
mv "$marker_tmp" "$deployment_marker"

printf 'MyPanel updated successfully to %s.\n' "$(git rev-parse --short HEAD)"
docker compose ps
