#!/bin/sh
set -eu

test_root=$(mktemp -d "${TMPDIR:-/tmp}/mypanel-update-test.XXXXXX")
trap 'rm -rf "$test_root"' EXIT HUP INT TERM

mkdir -p "$test_root/project/scripts" "$test_root/project/.git" "$test_root/bin"
cp "$(dirname "$0")/update.sh" "$test_root/project/scripts/update.sh"

cat > "$test_root/bin/git" <<'EOF'
#!/bin/sh
case "$*" in
  'status --porcelain --untracked-files=normal') exit 0 ;;
  'symbolic-ref --quiet --short HEAD') printf '%s\n' 'feat/test' ;;
  'remote get-url origin') printf '%s\n' 'https://example.invalid/repo.git' ;;
  fetch*) exit 0 ;;
  'show-ref --verify --quiet refs/remotes/origin/feat/test') exit 0 ;;
  'merge-base --is-ancestor HEAD refs/remotes/origin/feat/test') exit 0 ;;
  'rev-parse HEAD'|'rev-parse refs/remotes/origin/feat/test') printf '%s\n' '1111111111111111111111111111111111111111' ;;
  'rev-parse --git-path mypanel-deployed') printf '%s\n' '.git/mypanel-deployed' ;;
  'rev-parse --short HEAD') printf '%s\n' '1111111' ;;
  'status --short') exit 0 ;;
  *) printf 'unexpected git command: %s\n' "$*" >&2; exit 2 ;;
esac
EOF

cat > "$test_root/bin/docker" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >> "$TEST_DOCKER_LOG"
if [ "$*" = 'compose build --pull' ] && [ "${FAIL_BUILD:-0}" = '1' ]; then
  exit 1
fi
exit 0
EOF
chmod +x "$test_root/bin/git" "$test_root/bin/docker"

docker_log=$test_root/docker.log
export TEST_DOCKER_LOG=$docker_log
test_path=$test_root/bin:$PATH

if PATH=$test_path USER=test FAIL_BUILD=1 sh "$test_root/project/scripts/update.sh" >/dev/null 2>&1; then
  printf '%s\n' 'expected the first deployment to fail' >&2
  exit 1
fi
if [ -e "$test_root/project/.git/mypanel-deployed" ]; then
  printf '%s\n' 'failed deployment wrote the success marker' >&2
  exit 1
fi

PATH=$test_path USER=test sh "$test_root/project/scripts/update.sh" >/dev/null
if [ "$(cat "$test_root/project/.git/mypanel-deployed")" != '1111111111111111111111111111111111111111' ]; then
  printf '%s\n' 'successful deployment did not write the expected marker' >&2
  exit 1
fi

build_count=$(grep -c '^compose build --pull$' "$docker_log")
if [ "$build_count" -ne 2 ]; then
  printf 'deployment was not retried after failure; build count=%s\n' "$build_count" >&2
  exit 1
fi

PATH=$test_path USER=test sh "$test_root/project/scripts/update.sh" >/dev/null
if [ "$(grep -c '^compose build --pull$' "$docker_log")" -ne 2 ]; then
  printf '%s\n' 'already-deployed commit was deployed again' >&2
  exit 1
fi

printf '%s\n' 'update retry tests passed'
