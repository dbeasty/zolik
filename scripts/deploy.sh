#!/usr/bin/env bash
#
# Deploy zolik to play.limidus.com on limi-mini (192.168.13.13).
#
#   ./scripts/deploy.sh
#
# Builds one image here — the Go server with the Expo web client compiled into
# it — ships it over SSH, and runs it there. The host holds a compose file, an
# env file and the image. It has no zolik source, no kdb checkout, no Go
# toolchain and no npm, and nothing is compiled on it.
#
# Options:
#   --init-env     overwrite server .env from deploy/env.production.example
#                  (generates fresh JWT secrets)
#   --snapshot     tar the database volume on the host before switching images
#   --skip-nginx   skip nginx vhost install (docker only)
#
# Environment:
#   ZOLIK_DEPLOY_HOST   default 192.168.13.13
#   ZOLIK_DEPLOY_SSH    default davja@192.168.13.13  (sudo/nginx steps)
#   ZOLIK_DEPLOY_USER   default zolik                 (runtime owner)
#   ZOLIK_PUBLIC_URL    default https://play.limidus.com
#   ZOLIK_SERVICE_IP    default 192.168.13.13         (nginx listen address)
#   ZOLIK_KEEP_IMAGES   default 3                     (release tags kept on the host)
#
#   Who the Terms and the Privacy Notice name. Baked into the web bundle at
#   build time; a deployment names its own operator. All three must be set or
#   both notices deploy carrying a "not yet in force" draft banner. A fork
#   deploying this source must override all three — leaving them would make a
#   claim about someone else's company.
#   ZOLIK_OPERATOR          default Limidus Corp
#   ZOLIK_OPERATOR_COUNTRY  default USA                  (jurisdiction governing the terms)
#   ZOLIK_OPERATOR_CONTACT  default support@limidus.com  (address deletion requests arrive at)
#
#   Where this deployment offers its source, as AGPL section 13 requires of a
#   thing served over a network. The default is right for deploying this source
#   unmodified; a fork must point at its own.
#   ZOLIK_SOURCE_URL        default https://github.com/dbeasty/zolik

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

DEPLOY_HOST="${ZOLIK_DEPLOY_HOST:-192.168.13.13}"
DEPLOY_SSH="${ZOLIK_DEPLOY_SSH:-davja@${DEPLOY_HOST}}"
DEPLOY_USER="${ZOLIK_DEPLOY_USER:-zolik}"
PUBLIC_URL="${ZOLIK_PUBLIC_URL:-https://play.limidus.com}"
SERVICE_IP="${ZOLIK_SERVICE_IP:-192.168.13.13}"
KEEP_IMAGES="${ZOLIK_KEEP_IMAGES:-3}"

# Who the legal notices name.
#
# These were empty on purpose until someone stated them: a wrong jurisdiction
# or an address nobody reads is worse than a visibly unfinished document, so
# the client shows a draft banner while any of the three is missing rather than
# confidently naming nobody. They are stated now, so the banner clears and the
# notices are documents rather than drafts.
#
# Defaults rather than required arguments because this repository has exactly
# one deployment and it is this one — the same reason ZOLIK_OPERATOR has always
# defaulted to the company name. A fork overrides all three; the header above
# says so.
OPERATOR="${ZOLIK_OPERATOR:-Limidus Corp}"
OPERATOR_COUNTRY="${ZOLIK_OPERATOR_COUNTRY:-USA}"
OPERATOR_CONTACT="${ZOLIK_OPERATOR_CONTACT:-support@limidus.com}"

# The AGPL section 13 offer. Unlike the three above this one has a default and
# no draft state: the canonical repository is the true answer for any build
# that has not changed the code, and a deployment that has changed it owes its
# players that source instead — which is what overriding this is for.
SOURCE_URL="${ZOLIK_SOURCE_URL:-https://github.com/dbeasty/zolik}"

# Everything the host holds, and all of it written by this script.
REMOTE_DIR="/home/${DEPLOY_USER}"
ENV_REMOTE="${REMOTE_DIR}/.env"
COMPOSE_REMOTE="${REMOTE_DIR}/compose.yml"

# Where the source-built deployment used to live. Read only to shut it down
# and to rescue its .env; never written to again.
LEGACY_SRC="${REMOTE_DIR}/src/zolik"

INIT_ENV=false
SNAPSHOT=false
SKIP_NGINX=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --init-env)   INIT_ENV=true; shift ;;
    --snapshot)   SNAPSHOT=true; shift ;;
    --skip-nginx) SKIP_NGINX=true; shift ;;
    -h|--help)
      sed -n '2,36p' "$0"
      exit 0
      ;;
    *) printf 'Unknown option: %s\n' "$1" >&2; exit 1 ;;
  esac
done

eval "$(sh "${ROOT}/scripts/version.sh" --export)"
RELEASE="${ZOLIK_VERSION}+${ZOLIK_COMMIT}"
# The same release, spelled for Docker. A tag may not contain "+", and a
# rejected tag this late reads as a build failure rather than a naming one.
TAG="${ZOLIK_VERSION}-${ZOLIK_COMMIT}"
IMAGE="zolik:${TAG}"

say()  { printf '\033[1m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[33mwarn:\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[31mError:\033[0m %s\n' "$*" >&2; exit 1; }

ssh_admin() { ssh -o BatchMode=yes "$DEPLOY_SSH" "$@"; }
# Everything that touches Docker runs as the runtime owner, so the image, the
# containers and the volume all belong to one user and one daemon path.
ssh_zolik() { ssh_admin "sudo -u ${DEPLOY_USER} $*"; }

# ---------------------------------------------------------------- preflight
say "preflight (${RELEASE} → ${PUBLIC_URL})"

command -v ssh    >/dev/null || die "ssh is required"
command -v docker >/dev/null || die "docker is required — the image is built here now, not on the server"
docker buildx version >/dev/null 2>&1 || die "docker buildx is required (the build cross-compiles to linux/amd64)"

ssh_admin 'echo ok' >/dev/null 2>&1 || die "cannot SSH to ${DEPLOY_SSH} (BatchMode=yes)"

if [[ ! -f "${ROOT}/../kdb/go/go.mod" ]]; then
  die "kdb repo not found at ${ROOT}/../kdb — clone it as a sibling of zolik"
fi

ssh_admin "command -v docker >/dev/null" || die "docker is not installed on ${DEPLOY_HOST}"
ssh_admin "command -v nginx >/dev/null"  || die "nginx is not installed on ${DEPLOY_HOST}"

# The image is the artifact now, so what went into it matters more than it did
# when the host rebuilt from a named commit. A dirty tree still deploys — that
# is often deliberate — but the tag says so and so does this.
if [[ "$ZOLIK_COMMIT" == *-dirty ]]; then
  warn "the working tree has uncommitted changes; deploying as ${TAG}"
fi

# -------------------------------------------------------------- bootstrap
say "bootstrap ${DEPLOY_USER} on ${DEPLOY_HOST}"

ssh_admin "sudo bash -s" <<EOF
set -euo pipefail
if ! id -u ${DEPLOY_USER} >/dev/null 2>&1; then
  useradd -m -s /bin/bash ${DEPLOY_USER}
  echo "created user ${DEPLOY_USER}"
fi
usermod -aG docker ${DEPLOY_USER} 2>/dev/null || true
mkdir -p ${REMOTE_DIR}
chown ${DEPLOY_USER}:${DEPLOY_USER} ${REMOTE_DIR}
EOF

# -------------------------------------------------------------- server env
say "server .env"

ENV_TEMPLATE="${ROOT}/deploy/env.production.example"

# The env file moved with the rest of the deployment. Move it rather than
# asking someone to: regenerating it instead would mint new JWT secrets and
# sign every player out on release day, which is a strange thing to have
# happen because a file changed directory.
ssh_admin "sudo bash -s" <<EOF
set -euo pipefail
if [[ ! -f ${ENV_REMOTE} && -f ${LEGACY_SRC}/server/.env ]]; then
  cp ${LEGACY_SRC}/server/.env ${ENV_REMOTE}
  chown ${DEPLOY_USER}:${DEPLOY_USER} ${ENV_REMOTE}
  chmod 600 ${ENV_REMOTE}
  echo "moved .env from ${LEGACY_SRC}/server/.env"
fi
EOF

if [[ "$INIT_ENV" == true ]] || ! ssh_admin "sudo test -f ${ENV_REMOTE}" 2>/dev/null; then
  access_secret="$(openssl rand -hex 32)"
  refresh_secret="$(openssl rand -hex 32)"
  tmp_env="$(mktemp)"
  sed \
    -e "s|JWT_ACCESS_SECRET=REPLACE_ON_FIRST_DEPLOY|JWT_ACCESS_SECRET=${access_secret}|" \
    -e "s|JWT_REFRESH_SECRET=REPLACE_ON_FIRST_DEPLOY|JWT_REFRESH_SECRET=${refresh_secret}|" \
    "${ENV_TEMPLATE}" > "$tmp_env"
  scp -q "$tmp_env" "${DEPLOY_SSH}:/tmp/zolik-server.env"
  rm -f "$tmp_env"
  ssh_admin "sudo mv /tmp/zolik-server.env ${ENV_REMOTE} && sudo chown ${DEPLOY_USER}:${DEPLOY_USER} ${ENV_REMOTE} && sudo chmod 600 ${ENV_REMOTE}"
  say "wrote ${ENV_REMOTE}"
  warn "fresh JWT secrets — any existing session is now invalid"
else
  say "keeping existing ${ENV_REMOTE} (pass --init-env to replace)"
fi

# Patch PUBLIC_BASE_URL if the deploy target changed.
ssh_admin "sudo bash -s" <<EOF
set -euo pipefail
if grep -q '^PUBLIC_BASE_URL=' ${ENV_REMOTE}; then
  sed -i "s|^PUBLIC_BASE_URL=.*|PUBLIC_BASE_URL=${PUBLIC_URL}|" ${ENV_REMOTE}
else
  echo "PUBLIC_BASE_URL=${PUBLIC_URL}" >> ${ENV_REMOTE}
fi
chown ${DEPLOY_USER}:${DEPLOY_USER} ${ENV_REMOTE}
chmod 600 ${ENV_REMOTE}
EOF

# ------------------------------------------------------- will it even boot?
#
# APP_ENV decides four defaults on the server, and three of them get safer
# when it is not "local": the SSH terminal client, its admit-any-key mode, and
# the two development hatches all switch off. The fourth is a hard
# requirement — a real environment refuses to start without SMTP rather than
# silently swallowing sign-in codes (auth.NewMailer) — and getting it wrong
# does not look like a missing variable. It looks like the container fatally
# exiting and Docker restarting it forever.
#
# So it is checked here: after .env is final, before anything is built.
say "checking the server env will boot"

read_env() { ssh_admin "sudo sed -n 's/^$1=//p' ${ENV_REMOTE} | tail -1" 2>/dev/null || true; }
env_app="$(read_env APP_ENV)"
env_smtp="$(read_env SMTP_HOST)"

case "$env_app" in
  ""|local)
    warn "APP_ENV=${env_app:-<unset>} — guest sign-in only, and the SSH client and both"
    warn "  development hatches default ON (the .env and compose file hold them shut)"
    ;;
  *)
    if [[ -z "$env_smtp" ]]; then
      die "APP_ENV=${env_app} requires SMTP_HOST in ${ENV_REMOTE}.
  The server refuses to start without it and the container will restart forever.
  Either set SMTP_HOST/SMTP_FROM there, or set APP_ENV=local for guest-only play."
    fi
    say "APP_ENV=${env_app}, SMTP_HOST set — hatches and SSH off by default"
    ;;
esac

# ------------------------------------------------------------- build image
say "building ${IMAGE} (server + web client, one image)"

# The notices are prerendered into the bundle (app.json sets web output to
# "static"), so who they name is decided at build time and cannot be changed
# without a rebuild. Said out loud because the failure is otherwise silent — a
# perfectly working deploy whose Terms name "[OPERATOR NAME]".
if [[ -n "$OPERATOR_COUNTRY" && -n "$OPERATOR_CONTACT" ]]; then
  say "legal notices name ${OPERATOR} (${OPERATOR_COUNTRY}, ${OPERATOR_CONTACT})"
else
  warn "legal notices will deploy as a DRAFT — both screens carry a banner saying so."
  warn "  operator: ${OPERATOR}"
  # Full `if`s, not `[[ … ]] && warn`: under `set -e` a false test is a
  # failing command, and this script would exit here instead of warning.
  if [[ -z "$OPERATOR_COUNTRY" ]]; then
    warn "  missing ZOLIK_OPERATOR_COUNTRY (governing law)"
  fi
  if [[ -z "$OPERATOR_CONTACT" ]]; then
    warn "  missing ZOLIK_OPERATOR_CONTACT (where deletion requests arrive)"
  fi
fi

say "source offered at ${SOURCE_URL}"

# --platform is explicit because the deploy host is amd64 and this machine may
# not be. The Dockerfile pins both build stages to the *build* platform and
# cross-compiles, so this costs nothing beyond naming the target.
docker buildx build \
  --platform linux/amd64 \
  --build-context "kdbsrc=${ROOT}/../kdb" \
  -f "${ROOT}/server/Dockerfile" \
  --build-arg "ZOLIK_VERSION=${ZOLIK_VERSION}" \
  --build-arg "ZOLIK_COMMIT=${ZOLIK_COMMIT}" \
  --build-arg "ZOLIK_PUBLIC_URL=${PUBLIC_URL}" \
  --build-arg "ZOLIK_OPERATOR=${OPERATOR}" \
  --build-arg "ZOLIK_OPERATOR_COUNTRY=${OPERATOR_COUNTRY}" \
  --build-arg "ZOLIK_OPERATOR_CONTACT=${OPERATOR_CONTACT}" \
  --build-arg "ZOLIK_SOURCE_URL=${SOURCE_URL}" \
  -t "${IMAGE}" \
  --load \
  "${ROOT}"

# -------------------------------------------------------------- ship image
if [[ "$TAG" != *-dirty ]] && ssh_zolik "docker image inspect ${IMAGE} >/dev/null 2>&1"; then
  say "${IMAGE} already on ${DEPLOY_HOST} — skipping transfer"
else
  say "shipping ${IMAGE} to ${DEPLOY_HOST}"
  # `docker load` reads gzip directly, so there is nothing to decompress on
  # the far side. Piped rather than staged through a file: the host does not
  # need a copy of the tarball, and a partial transfer leaves nothing to
  # clean up.
  docker save "${IMAGE}" | gzip -1 | ssh_admin "sudo -u ${DEPLOY_USER} docker load" \
    || die "shipping the image failed"
fi

# ----------------------------------------------------------- compose + run
say "installing ${COMPOSE_REMOTE}"

scp -q "${ROOT}/deploy/compose/zolik.yml" "${DEPLOY_SSH}:/tmp/zolik-compose.yml"
ssh_admin "sudo mv /tmp/zolik-compose.yml ${COMPOSE_REMOTE} && sudo chown ${DEPLOY_USER}:${DEPLOY_USER} ${COMPOSE_REMOTE}"

# The source-built stack, if this host is still running one. It holds port
# 8090, so it has to go before the new one can bind.
#
# `down`, never `down -v`. The -v would delete its named volume, and on the
# first run of this script that volume is still the live database. Bringing it
# down without -v leaves it on disk, which is what makes the rollback at the
# bottom of docs/docker-deploy-plan.md possible.
ssh_admin "sudo -u ${DEPLOY_USER} bash -s" <<EOF
set -euo pipefail
if [[ -f ${LEGACY_SRC}/server/docker-compose.kdb.yml ]]; then
  echo "stopping the source-built stack (its volume is left in place)"
  cd ${LEGACY_SRC}/server
  docker compose -f docker-compose.kdb.yml down || true
fi
EOF

if [[ "$SNAPSHOT" == true ]]; then
  say "snapshotting the database volume"
  ssh_admin "sudo -u ${DEPLOY_USER} bash -s" <<EOF
set -euo pipefail
mkdir -p ${REMOTE_DIR}/backups
if docker volume inspect zolik_kdb_data >/dev/null 2>&1; then
  docker run --rm -v zolik_kdb_data:/from -v ${REMOTE_DIR}/backups:/backup alpine \
    tar czf /backup/kdb-\$(date +%F-%H%M).tgz -C /from .
  ls -1t ${REMOTE_DIR}/backups/kdb-*.tgz | tail -n +6 | xargs -r rm -f
  echo "snapshot written to ${REMOTE_DIR}/backups"
else
  echo "no zolik_kdb_data volume yet — nothing to snapshot"
fi
EOF
fi

say "starting ${IMAGE}"
ssh_admin "sudo -u ${DEPLOY_USER} bash -s" <<EOF
set -euo pipefail
export ZOLIK_RELEASE='${TAG}'
cd ${REMOTE_DIR}
docker compose up -d
EOF

say "waiting for API on ${DEPLOY_HOST}:8090"
for _ in $(seq 1 120); do
  if ssh_admin "curl -fsS -m 2 http://127.0.0.1:8090/healthz >/dev/null 2>&1"; then
    break
  fi
  sleep 1
done
ssh_admin "curl -fsS http://127.0.0.1:8090/healthz" >/dev/null \
  || die "server did not come up at http://127.0.0.1:8090/healthz"

# --------------------------------------------------------------- old tags
#
# Kept, not pruned to nothing: a rollback is only as good as the image it
# rolls back to. `docker image rm` by tag, never `docker system prune`, which
# reaches volumes.
say "keeping the last ${KEEP_IMAGES} release images"
ssh_admin "sudo -u ${DEPLOY_USER} bash -s" <<EOF
set -euo pipefail
docker image ls zolik --format '{{.Repository}}:{{.Tag}}' \
  | tail -n +\$((${KEEP_IMAGES} + 1)) \
  | xargs -r -n1 docker image rm 2>/dev/null || true
docker image ls zolik --format '  {{.Tag}}  {{.Size}}'
EOF

# -------------------------------------------------------------- nginx
if [[ "$SKIP_NGINX" == false ]]; then
  say "installing nginx vhost for play.limidus.com"

  # Substitute service IP if overridden (template ships with 192.168.13.13).
  tmp_nginx="$(mktemp)"
  sed "s/192\\.168\\.13\\.13/${SERVICE_IP}/g" \
    "${ROOT}/deploy/nginx/play-limidus.conf" > "$tmp_nginx"
  scp -q "$tmp_nginx" "${DEPLOY_SSH}:/tmp/play-limidus.conf"
  rm -f "$tmp_nginx"

  ssh_admin "sudo bash -s" <<'EOF'
set -euo pipefail
install -m 0644 /tmp/play-limidus.conf /etc/nginx/sites-available/play-limidus
ln -sfn /etc/nginx/sites-available/play-limidus /etc/nginx/sites-enabled/play-limidus
rm -f /tmp/play-limidus.conf
nginx -t
systemctl reload nginx
EOF
else
  warn "skipping nginx (--skip-nginx)"
fi

# ---------------------------------------------------------------- verify
say "verification"

health="FAIL"
for _ in $(seq 1 15); do
  if health="$(curl -fsS -m 10 "${PUBLIC_URL}/healthz" 2>/dev/null)"; then
    break
  fi
  health="FAIL"
  sleep 2
done

version="$(curl -fsS -m 10 "${PUBLIC_URL}/version" 2>/dev/null || echo FAIL)"
html="$(curl -fsS -m 10 "${PUBLIC_URL}/" 2>/dev/null || echo "")"

# The web client is served by the container now, so these two check something
# the old deploy could not: that the image's own bundle is reachable, and that
# the API/client split survives the proxy. /auth/login is the one that used to
# answer 405 — the server registers it as POST, the client has a screen there,
# and it is the URL every "sign in" link points at.
signin_code="$(curl -fsS -o /dev/null -w '%{http_code}' -m 10 \
  -H 'Accept: text/html' "${PUBLIC_URL}/auth/login" 2>/dev/null || echo FAIL)"
api_code="$(curl -s -o /dev/null -w '%{http_code}' -m 10 \
  -H 'Accept: application/json' "${PUBLIC_URL}/users/me" 2>/dev/null || echo FAIL)"

printf '  healthz         %s\n' "$health"
printf '  version         %s\n' "$version"
printf '  /auth/login     %s (want 200 — the sign-in page, not the API'"'"'s 405)\n' "$signin_code"
printf '  /users/me       %s (want 401 — the API, not a web page)\n' "$api_code"

if [[ "$health" != "ok" ]]; then
  die "health check failed at ${PUBLIC_URL}/healthz"
fi

if [[ "$html" != *"_expo/"* ]]; then
  die "${PUBLIC_URL}/ is not the Expo export — the image's web bundle is not being served"
fi
if [[ "$signin_code" != "200" ]]; then
  warn "GET ${PUBLIC_URL}/auth/login returned ${signin_code}; the sign-in link is broken"
fi
if [[ "$api_code" != "401" ]]; then
  warn "GET ${PUBLIC_URL}/users/me returned ${api_code}; expected the API's 401"
fi

echo
say "deployed ${RELEASE} to ${PUBLIC_URL}"
printf '  image    %s\n' "$IMAGE"
# Spelled through the admin account on purpose: the runtime user owns the
# containers but has no SSH key of its own, so every one of these is reached
# the same way this script reaches it.
printf '  logs     ssh %s "sudo -u %s bash -c \x27cd %s && docker compose logs -f app\x27"\n' \
  "$DEPLOY_SSH" "$DEPLOY_USER" "$REMOTE_DIR"
printf '  rollback ssh %s "sudo -u %s bash -c \x27cd %s && ZOLIK_RELEASE=<older-tag> docker compose up -d\x27"\n' \
  "$DEPLOY_SSH" "$DEPLOY_USER" "$REMOTE_DIR"
