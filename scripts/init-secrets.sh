#!/bin/sh
set -eu

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
secret_dir="$project_root/secrets"
umask 077
mkdir -p "$secret_dir"

for name in postgres_password redis_password admin_password; do
  target="$secret_dir/$name"
  if [ ! -f "$target" ]; then
    openssl rand -base64 32 | tr '+/' '-_' | tr -d '=\n' > "$target"
    printf 'Created secrets/%s\n' "$name"
  else
    printf 'Kept existing secrets/%s\n' "$name"
  fi
done

printf '%s\n' 'Secrets are ready. The generated owner password is stored in secrets/admin_password.'
