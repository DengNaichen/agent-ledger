#!/usr/bin/env bash
# Release origin/main to the production instance at
# https://harnessagent.tokensinfinity.com. See deploy/README.md.
#
#   deploy/release.sh [--dry-run] <ssh-host>
#
# <ssh-host> is the production host alias from the maintainer's ~/.ssh/config.
# It has no default: the alias "harness" is a different server. --dry-run
# prints every local and remote command in order and runs none of them, so it
# opens no network connection.
#
# The script never rolls back. When the release fails after compose.yaml was
# changed, it prints the previous image, the backup directory and the manual
# rollback steps.
set -euo pipefail

# Production server layout, as /opt/inftask/RUNBOOK.txt describes it.
REMOTE_BASE=/opt/inftask
COMPOSE_FILE=$REMOTE_BASE/compose.yaml
# Mode 0600 credentials for Compose; this script only checks that it exists.
ENV_FILE=$REMOTE_BASE/runtime.env
RUNBOOK=$REMOTE_BASE/RUNBOOK.txt
DEPLOYMENT_JSON=$REMOTE_BASE/deployment.json
# Bind-mounted as /data in the app container and owned by UID 10001.
DATA_DIR=$REMOTE_BASE/data
# One build context per short sha.
RELEASES=$REMOTE_BASE/releases
# One backup directory per update, as the AL-39 update started.
BACKUPS=$REMOTE_BASE/migrations
# Both services use pull_policy: never; the host cannot reach Docker Hub.
# The app image is the only "    image: inftask:<tag>" line in compose.yaml.
# The proxy service and its volumes are never touched.
COMPOSE="docker compose --env-file $ENV_FILE -p inftask -f $COMPOSE_FILE"
SERVER_CA_BUNDLE=/etc/ssl/certs/ca-certificates.crt

PUBLIC_URL=https://harnessagent.tokensinfinity.com
HEALTH_URL=$PUBLIC_URL/api/health
MCP_URL=$PUBLIC_URL/w/default/mcp

ssh_opts=(-o BatchMode=yes -o ConnectTimeout=15 -o ServerAliveInterval=15 -o ServerAliveCountMax=4)

# Remote script: check that this is the production host and print the app
# image that compose.yaml names now.
preflight_script() {
	cat <<EOF
for f in $RUNBOOK $DEPLOYMENT_JSON $COMPOSE_FILE $ENV_FILE $DATA_DIR/inftask.db; do
  test -f "\$f" || { echo "missing \$f; is this the production host?" >&2; exit 1; }
done
test -d $BACKUPS || { echo "missing $BACKUPS" >&2; exit 1; }
command -v sha256sum >/dev/null || { echo "sha256sum is missing" >&2; exit 1; }
python3 -c 'import sqlite3' || { echo "the backup needs python3 with its sqlite3 module" >&2; exit 1; }
test "\$(grep -c '^    image: inftask:' $COMPOSE_FILE)" = 1 || { echo "$COMPOSE_FILE must have exactly one '    image: inftask:' line" >&2; exit 1; }
sed -n 's/^    image: \(inftask:[^[:space:]]*\)[[:space:]]*\$/\1/p' $COMPOSE_FILE
EOF
}

# Remote script: add the server's CA bundle to the uploaded build context.
context_script() {
	cat <<EOF
cp $SERVER_CA_BUNDLE $release_dir/ca-certificates.crt
test "\$(LC_ALL=C ls -A $release_dir | tr '\n' ' ')" = 'Dockerfile ca-certificates.crt inftask ' || { echo "$release_dir must hold exactly Dockerfile, ca-certificates.crt and inftask" >&2; exit 1; }
EOF
}

# Remote script: keep the files this release changes and take a consistent
# copy of the live database with SQLite's online backup API.
backup_script() {
	cat <<EOF
umask 077
mkdir -m 700 $backup_dir
cp $COMPOSE_FILE $DEPLOYMENT_JSON $RUNBOOK $backup_dir/
python3 - $DATA_DIR/inftask.db $backup_dir/before-update.db <<'PY'
import sqlite3, sys
src = sqlite3.connect("file:" + sys.argv[1] + "?mode=ro", uri=True)
dst = sqlite3.connect(sys.argv[2])
src.backup(dst)
src.close()
# The copy inherits WAL mode; switch it back so the single file opens
# read-only later without -wal/-shm files.
dst.execute("PRAGMA journal_mode=DELETE")
check = dst.execute("PRAGMA integrity_check").fetchall()
dst.close()
if check != [("ok",)]:
    sys.exit("integrity_check of %s: %s" % (sys.argv[2], check))
PY
chmod 600 $backup_dir/before-update.db
sha256sum $backup_dir/before-update.db
EOF
}

# Remote script: point the app at the new image and recreate only the app.
switch_script() {
	cat <<EOF
sed -i 's|^    image: inftask:[^[:space:]]*[[:space:]]*\$|    image: $image|' $COMPOSE_FILE
grep -qx '    image: $image' $COMPOSE_FILE || { echo "$COMPOSE_FILE does not name $image after the edit" >&2; exit 1; }
$COMPOSE up -d --no-deps app
EOF
}

# Remote script: confirm the running app container uses the new image.
running_script() {
	cat <<EOF
id=\$($COMPOSE ps -q app)
running=\$(docker inspect --format '{{.Config.Image}}' "\$id")
test "\$running" = $image || { echo "app runs \$running, expected $image" >&2; exit 1; }
EOF
}

# Remote script: record the release in deployment.json, keeping its other
# fields, and in RUNBOOK.txt.
record_script() {
	cat <<EOF
id=\$(docker image inspect --format '{{.Id}}' $image)
python3 - $DEPLOYMENT_JSON $commit $image "\$id" $backup_dir/before-update.db <<'PY'
import datetime, json, os, sys
path, commit, image, image_id, backup = sys.argv[1:]
with open(path, encoding="utf-8") as f:
    data = json.load(f)
data["source_commit"] = commit
data["images"][image] = image_id
data["latest_update_backup"] = backup
data["updated_at"] = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
with open(path + ".tmp", "w", encoding="utf-8") as f:
    json.dump(data, f, indent=2, ensure_ascii=False)
    f.write("\n")
os.chmod(path + ".tmp", os.stat(path).st_mode & 0o777)
PY
mv $DEPLOYMENT_JSON.tmp $DEPLOYMENT_JSON
sed -i -e 's|^\([[:space:]]*\)Source commit: .*\$|\1Source commit: $commit|' -e 's|^\([[:space:]]*\)Application image: .*\$|\1Application image: $image|' $RUNBOOK
grep -q '^[[:space:]]*Source commit: $commit\$' $RUNBOOK || { echo "no 'Source commit:' line in $RUNBOOK" >&2; exit 1; }
grep -q '^[[:space:]]*Application image: $image\$' $RUNBOOK || { echo "no 'Application image:' line in $RUNBOOK" >&2; exit 1; }
echo 'Release $version update backup: $backup_dir/before-update.db' >> $RUNBOOK
EOF
}

usage() {
	echo "usage: deploy/release.sh [--dry-run] <ssh-host>" >&2
	exit 2
}

die() {
	printf 'release: %s\n' "$*" >&2
	exit 1
}

# Commands and step headers go to fd 3, so remote output can be captured.
exec 3>&1
show() { printf '+ %s\n' "$*" >&3; }
step() { printf '\n==> [%s/6] %s\n' "$1" "$2" >&3; }

# Waits at most LIMIT seconds for COMMAND, then stops it and fails with WHAT.
bounded() {
	local limit=$1 what=$2 status=0 deadline
	shift 2
	deadline=$((SECONDS + limit))
	"$@" <&0 &
	child=$!
	while kill -0 "$child" 2>/dev/null; do
		if ((SECONDS >= deadline)); then
			kill "$child" 2>/dev/null || true
			wait "$child" 2>/dev/null || true
			child=
			die "timed out after ${limit}s waiting for ${what}"
		fi
		sleep 0.5
	done
	wait "$child" || status=$?
	child=
	((status == 0)) || die "${what} failed (exit ${status})"
}

run() {
	local limit=$1 what=$2
	shift 2
	show "$*"
	((dry_run)) || bounded "$limit" "$what" "$@"
}

remote() {
	local limit=$1 what=$2 script
	script="set -eu
$3"
	printf "+ ssh %s %s sh -s <<'REMOTE'\n%s\nREMOTE\n" "${ssh_opts[*]}" "$host" "$script" >&3
	((dry_run)) || bounded "$limit" "$what" ssh "${ssh_opts[@]}" "$host" sh -s <<<"$script"
}

# Polls URL until it returns STATUS, for at most LIMIT seconds.
wait_for() {
	local limit=$1 want=$2 url=$3 code deadline
	show "curl -s -o /dev/null -w '%{http_code}' --connect-timeout 5 --max-time 10 $url   # until $want, at most ${limit}s"
	if ((dry_run)); then return 0; fi
	deadline=$((SECONDS + limit))
	while :; do
		code=$(curl -s -o /dev/null -w '%{http_code}' --connect-timeout 5 --max-time 10 "$url" || true)
		if [ "$code" = "$want" ]; then return 0; fi
		((SECONDS < deadline)) || die "timed out after ${limit}s waiting for $url to return $want (last status: ${code:-none})"
		sleep 3
	done
}

rollback_hint() {
	cat >&2 <<EOF

release: $image did not pass and nothing was rolled back.
The new binary may already have migrated the database, and an older binary
must not run against an upgraded database (docs/rules/RELIABILITY.md), so the
maintainer decides how to roll back. deployment.json and RUNBOOK.txt still
record the previous release.

  previous app image:  $previous
  backup directory:    $backup_dir
                       (compose.yaml, deployment.json, RUNBOOK.txt, before-update.db)

Manual rollback (deploy/README.md):
  1. Decide whether the database must go back. Compare PRAGMA user_version of
     $backup_dir/before-update.db and $DATA_DIR/inftask.db
     with python3 (the host has no sqlite3 CLI). Equal: no migration ran, keep
     the current database. Different: only if docs/rules/RELIABILITY.md says
     $previous cannot read the upgraded database, restore before-update.db
     as the Restore section of $RUNBOOK describes; writes made after
     the backup are lost. Never restore or delete inftask.db.sessions.
  2. Put the previous compose.yaml back and start the previous image:
       ssh $host 'cp $backup_dir/compose.yaml $COMPOSE_FILE && $COMPOSE up -d --no-deps app'
  3. Check that $HEALTH_URL returns 200 and
     $MCP_URL returns 401 without credentials.
EOF
}

on_exit() {
	local status=$?
	if [ -n "$child" ]; then kill "$child" 2>/dev/null || true; fi
	if [ -n "$work" ]; then rm -rf "$work"; fi
	if [ "$status" -ne 0 ] && ((switched)); then rollback_hint; fi
}

dry_run=0
host=
for arg in "$@"; do
	case $arg in
	--dry-run) dry_run=1 ;;
	-*) usage ;;
	*)
		[ -z "$host" ] || usage
		host=$arg
		;;
	esac
done
[ -n "$host" ] || usage

cd "$(dirname "${BASH_SOURCE[0]}")/.."

child=
work=
switched=0
previous=
backup_dir=
trap on_exit EXIT
trap 'exit 130' INT TERM

if ((dry_run)); then
	ctx='<tmp>/context'
	echo "Dry run: printing the release plan for $host; nothing below is executed." >&3
else
	work=$(mktemp -d "${TMPDIR:-/tmp}/inftask-release.XXXXXX")
	ctx=$work/context
	mkdir "$ctx"
fi

step 1 "Check that the worktree is clean and HEAD is origin/main"
show "git status --porcelain   # must print nothing"
if ((!dry_run)); then
	[ -z "$(git status --porcelain)" ] || die "the worktree has uncommitted or untracked changes; release from a clean main checkout"
fi
run 60 "git fetch of origin main" git fetch origin main
show "git rev-parse HEAD origin/main   # must print the same commit twice"
if ((!dry_run)); then
	[ "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)" ] || die "HEAD is not origin/main; run 'git switch main && git pull --ff-only' first"
fi
commit=$(git rev-parse HEAD)
version=$(git rev-parse --short HEAD)
show "git rev-parse --short HEAD   # $version"
image=inftask:$version
release_dir=$RELEASES/$version
backup_dir=$BACKUPS/$version-$(date -u +%Y%m%dT%H%M%SZ)

step 2 "Build the frontend and a static linux/amd64 binary"
run 900 "npm --prefix web ci" npm --prefix web ci
run 600 "npm --prefix web run build" npm --prefix web run build
run 900 "go build of the linux/amd64 binary" env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o "$ctx/inftask" ./cmd/inftask
run 30 "copying deploy/Dockerfile.binary" cp deploy/Dockerfile.binary "$ctx/Dockerfile"

step 3 "Check the server and upload the build context to $release_dir"
if ((dry_run)); then out=/dev/null; else out=$work/previous; fi
remote 60 "the server check on $host" "$(preflight_script)" >"$out"
if ((dry_run)); then
	previous='<image compose.yaml names now>'
else
	previous=$(cat "$out")
	[ -n "$previous" ] || die "could not read the app image from $COMPOSE_FILE on $host"
fi
remote 60 "mkdir on $host" "mkdir -p $release_dir"
run 900 "the upload to $host" scp "${ssh_opts[@]}" "$ctx/inftask" "$ctx/Dockerfile" "$host:$release_dir/"
remote 60 "the CA bundle copy on $host" "$(context_script)"

step 4 "Back up, build $image and switch the app on $host"
remote 600 "the backup into $backup_dir on $host" "$(backup_script)"
remote 600 "docker build of $image on $host" "docker build -t $image $release_dir"
switched=1
remote 300 "the switch of app to $image on $host" "$(switch_script)"
remote 60 "the running image check on $host" "$(running_script)"

step 5 "Verify $PUBLIC_URL"
wait_for 120 200 "$HEALTH_URL"
wait_for 30 401 "$MCP_URL"
switched=0

step 6 "Record the release in $DEPLOYMENT_JSON and $RUNBOOK"
remote 60 "the deployment.json and RUNBOOK.txt update on $host" "$(record_script)"

if ((!dry_run)); then
	printf '\nReleased %s (%s); previous app image was %s; backup in %s.\n' "$image" "$commit" "$previous" "$backup_dir" >&3
fi
