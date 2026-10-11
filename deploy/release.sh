#!/usr/bin/env bash
# Release origin/main to the production instance at
# https://harnessagent.tokensinfinity.com. See deploy/README.md.
#
#   deploy/release.sh [--dry-run] <ssh-host>
#
# <ssh-host> is an alias from your own ~/.ssh/config for the machine that
# harnessagent.tokensinfinity.com resolves to. It has no default: the alias
# "harness" is a different server. --dry-run prints every local and remote
# command in order and runs none of them, so it opens no network connection.
#
# The script never rolls back. When the release fails after the app was
# switched, it prints the previous image and the manual rollback steps.
set -euo pipefail

# ---------------------------------------------------------------------------
# Production server layout, as /opt/inftask/RUNBOOK.txt on the host describes
# it. A value that has not been checked against the RUNBOOK sits under a
# TODO(AL-47 RUNBOOK) comment, and a real release refuses to start while any
# such comment remains. --dry-run prints the plan regardless.
# ---------------------------------------------------------------------------

# Holds the Compose file, runtime.env, RUNBOOK.txt and deployment.json (AL-24).
REMOTE_BASE=/opt/inftask
DEPLOYMENT_JSON=$REMOTE_BASE/deployment.json
# TODO(AL-47 RUNBOOK): upload location of build contexts; AL-24 did not record it.
REMOTE_RELEASES=$REMOTE_BASE/releases
# TODO(AL-47 RUNBOOK): whether the login user runs docker directly or through sudo.
DOCKER=docker
# TODO(AL-47 RUNBOOK): Compose project name, Compose file and app service name.
COMPOSE_PROJECT=inftask
COMPOSE_FILE=$REMOTE_BASE/compose.yaml
APP_SERVICE=app
# TODO(AL-47 RUNBOOK): path of runtime.env (mode 0600) and whether Compose reads it via --env-file.
ENV_FILE=$REMOTE_BASE/runtime.env
# TODO(AL-47 RUNBOOK): named volume mounted at /data in the app container.
DATA_VOLUME=${COMPOSE_PROJECT}_data

compose() {
	printf '%s compose -p %s -f %s --env-file %s' "$DOCKER" "$COMPOSE_PROJECT" "$COMPOSE_FILE" "$ENV_FILE"
}

# Prints the image of the running app container.
app_image() {
	printf "%s ps --filter label=com.docker.compose.project=%s --filter label=com.docker.compose.service=%s --format '{{.Image}}'" \
		"$DOCKER" "$COMPOSE_PROJECT" "$APP_SERVICE"
}

# Remote script: run the app on image $1, leaving the proxy and volumes alone.
# TODO(AL-47 RUNBOOK): how the app image tag is switched. Assumed: the Compose file reads ${INFTASK_IMAGE} for the app, like compose.ssh.yaml.
switch_script() {
	printf 'INFTASK_IMAGE=%s %s up -d --no-deps %s' "$1" "$(compose)" "$APP_SERVICE"
}

# Remote script: consistent SQLite backup of the live database into
# /data/backups/$1, checked with integrity_check.
# TODO(AL-47 RUNBOOK): backup command. Assumed: host sqlite3 as root on the volume mountpoint, file owned by UID 10001 afterwards.
backup_script() {
	cat <<EOF
dir=\$($DOCKER volume inspect -f '{{.Mountpoint}}' $DATA_VOLUME)
sqlite3 "\$dir/inftask.db" ".backup '\$dir/backups/$1'"
test "\$(sqlite3 "\$dir/backups/$1" 'PRAGMA integrity_check')" = ok
chown 10001:10001 "\$dir/backups/$1"
EOF
}

# Remote script: record the release in deployment.json, replacing it atomically.
# TODO(AL-47 RUNBOOK): deployment.json fields. Assumed: version (short sha), commit, image, image_id, previous_image, deployed_at.
deployment_script() {
	cat <<EOF
id=\$($DOCKER image inspect -f '{{.Id}}' $image)
printf '{"version":"%s","commit":"%s","image":"%s","image_id":"%s","previous_image":"%s","deployed_at":"%s"}\n' \\
  '$version' '$commit' '$image' "\$id" '$previous' "\$(date -u +%Y-%m-%dT%H:%M:%SZ)" > $DEPLOYMENT_JSON.tmp
mv $DEPLOYMENT_JSON.tmp $DEPLOYMENT_JSON
EOF
}

# ---------------------------------------------------------------------------

PUBLIC_URL=https://harnessagent.tokensinfinity.com
HEALTH_URL=$PUBLIC_URL/api/health
MCP_URL=$PUBLIC_URL/w/default/mcp
# Trusted CA bundle copied into the image as /etc/ssl/certs/ca-certificates.crt.
CA_BUNDLE=${CA_BUNDLE:-/etc/ssl/cert.pem}

ssh_opts=(-o BatchMode=yes -o ConnectTimeout=15 -o ServerAliveInterval=15 -o ServerAliveCountMax=4)

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

# Lists the TODO(AL-47 RUNBOOK) comments still in this file.
unconfirmed() { grep -n '# TODO[(]AL-47 RUNBOOK[)]:' "$self" || true; }

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
maintainer decides how to roll back. $DEPLOYMENT_JSON was not updated.

  previous app image:  $previous
  pre-release backup:  /data/backups/$backup_name in volume $DATA_VOLUME

Manual rollback (deploy/README.md):
  1. Compare PRAGMA user_version of the backup with /data/inftask.db.
     Equal: no migration ran; keep the current database and go to step 3.
  2. Different: read docs/rules/RELIABILITY.md for the new migrations. If
     $previous cannot run on the upgraded database, stop the app with
       ssh $host '$(compose) stop $APP_SERVICE'
     keep inftask.db and its -wal/-shm for investigation, restore the backup as
     /data/inftask.db, remove the old -wal/-shm and give UID 10001 ownership.
     Writes made after the backup are lost.
  3. ssh $host '$(switch_script "$previous")'
  4. Check that $HEALTH_URL returns 200 and $MCP_URL returns 401
     without credentials, then record $previous in $DEPLOYMENT_JSON.
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

self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/$(basename "${BASH_SOURCE[0]}")"
cd "$(dirname "$self")/.."

child=
work=
switched=0
previous=
backup_name=
trap on_exit EXIT
trap 'exit 130' INT TERM

if ((!dry_run)); then
	pending=$(unconfirmed)
	[ -z "$pending" ] || die "refusing a real release: these server values are not yet checked against $REMOTE_BASE/RUNBOOK.txt (AL-47). Until they are, update the instance by hand as the RUNBOOK says.
$pending"
	work=$(mktemp -d "${TMPDIR:-/tmp}/inftask-release.XXXXXX")
	ctx=$work/context
	mkdir "$ctx"
else
	ctx='<tmp>/context'
	echo "Dry run: printing the release plan for $host; nothing below is executed." >&3
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
release_dir=$REMOTE_RELEASES/$version
backup_name=before-$version.db

step 2 "Build the frontend and a static linux/amd64 binary"
run 900 "npm --prefix web ci" npm --prefix web ci
run 600 "npm --prefix web run build" npm --prefix web run build
run 900 "go build of the linux/amd64 binary" env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o "$ctx/inftask" ./cmd/inftask
run 30 "copying deploy/Dockerfile.binary" cp deploy/Dockerfile.binary "$ctx/Dockerfile.binary"
run 30 "copying the CA bundle $CA_BUNDLE" cp "$CA_BUNDLE" "$ctx/ca-certificates.crt"

step 3 "Check the server layout and upload the build context to $release_dir"
if ((dry_run)); then out=/dev/null; else out=$work/previous; fi
remote 60 "the layout check on $host" "for f in $REMOTE_BASE/RUNBOOK.txt $DEPLOYMENT_JSON $COMPOSE_FILE $ENV_FILE; do
  test -f \"\$f\" || { echo \"missing \$f; is this the production host?\" >&2; exit 1; }
done
$(app_image)" >"$out"
if ((dry_run)); then
	previous='<image the app runs now>'
else
	previous=$(cat "$out")
	[ -n "$previous" ] || die "no running $APP_SERVICE container in Compose project $COMPOSE_PROJECT on $host"
fi
remote 60 "mkdir on $host" "mkdir -p $release_dir"
run 900 "the upload to $host" scp "${ssh_opts[@]}" "$ctx/inftask" "$ctx/Dockerfile.binary" "$ctx/ca-certificates.crt" "$host:$release_dir/"

step 4 "Back up the database, build $image and switch the app on $host"
remote 600 "the SQLite backup on $host" "$(backup_script "$backup_name")"
remote 600 "docker build of $image on $host" "$DOCKER build -t $image -f $release_dir/Dockerfile.binary $release_dir"
switched=1
remote 300 "compose up of $APP_SERVICE on $host" "$(switch_script "$image")"
remote 60 "the running image check on $host" "running=\$($(app_image))
test \"\$running\" = $image || { echo \"$APP_SERVICE runs \$running, expected $image\" >&2; exit 1; }"

step 5 "Verify $PUBLIC_URL"
wait_for 120 200 "$HEALTH_URL"
wait_for 30 401 "$MCP_URL"
switched=0

step 6 "Record the release in $DEPLOYMENT_JSON"
remote 60 "the deployment.json update on $host" "$(deployment_script)"

if ((dry_run)); then
	pending=$(unconfirmed)
	if [ -n "$pending" ]; then
		printf '\nA real release refuses to start until these values are checked against %s/RUNBOOK.txt:\n%s\n' "$REMOTE_BASE" "$pending" >&3
	fi
else
	printf '\nReleased %s (%s); previous app image was %s.\n' "$image" "$commit" "$previous" >&3
fi
