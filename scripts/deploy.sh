#!/usr/bin/env bash
#
# Deploy zolik to play.limidus.com on limi-mini (192.168.13.13).
#
#   ./scripts/deploy.sh
#
# Builds the Expo web client locally, rsyncs zolik + kdb to the server,
# runs the KDB single-container stack as the zolik user, and installs the
# nginx vhost that replaces the static placeholder.
#
# Options:
#   --init-env     overwrite server .env from deploy/env.production.example
#                  (generates fresh JWT secrets)
#   --skip-web     skip local web build + upload (server/API only)
#   --skip-nginx   skip nginx vhost install (docker only)
#
# Environment:
#   ZOLIK_DEPLOY_HOST   default 192.168.13.13
#   ZOLIK_DEPLOY_SSH    default davja@192.168.13.13  (sudo/nginx steps)
#   ZOLIK_DEPLOY_USER   default zolik                 (runtime owner)
#   ZOLIK_PUBLIC_URL    default https://play.limidus.com
#   ZOLIK_SERVICE_IP    default 192.168.13.13         (nginx listen address)
#
#   Who the Terms and the Privacy Notice name. Baked into the web bundle at
#   build time; a deployment names its own operator. All three must be set or
#   both notices deploy carrying a "not yet in force" draft banner.
#   ZOLIK_OPERATOR          default Limidus Corp
#   ZOLIK_OPERATOR_COUNTRY  no default  (jurisdiction governing the terms)
#   ZOLIK_OPERATOR_CONTACT  no default  (address deletion requests arrive at)

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

DEPLOY_HOST="${ZOLIK_DEPLOY_HOST:-192.168.13.13}"
DEPLOY_SSH="${ZOLIK_DEPLOY_SSH:-davja@${DEPLOY_HOST}}"
DEPLOY_USER="${ZOLIK_DEPLOY_USER:-zolik}"
PUBLIC_URL="${ZOLIK_PUBLIC_URL:-https://play.limidus.com}"
SERVICE_IP="${ZOLIK_SERVICE_IP:-192.168.13.13}"

# Who the legal notices name. Only the name has a default: a wrong jurisdiction
# or an address nobody reads is worse than a visibly unfinished document, so
# those two stay empty until someone states them, and the client shows a draft
# banner while any of the three is missing.
OPERATOR="${ZOLIK_OPERATOR:-Limidus Corp}"
OPERATOR_COUNTRY="${ZOLIK_OPERATOR_COUNTRY:-}"
OPERATOR_CONTACT="${ZOLIK_OPERATOR_CONTACT:-}"

REMOTE_SRC="/home/${DEPLOY_USER}/src"
REMOTE_ZOLIK="${REMOTE_SRC}/zolik"
REMOTE_KDB="${REMOTE_SRC}/kdb"
REMOTE_WEB="/home/${DEPLOY_USER}/web"

INIT_ENV=false
SKIP_WEB=false
SKIP_NGINX=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --init-env)   INIT_ENV=true; shift ;;
    --skip-web)   SKIP_WEB=true; shift ;;
    --skip-nginx) SKIP_NGINX=true; shift ;;
    -h|--help)
      sed -n '2,29p' "$0"
      exit 0
      ;;
    *) printf 'Unknown option: %s\n' "$1" >&2; exit 1 ;;
  esac
done

eval "$(sh "${ROOT}/scripts/version.sh" --export)"
RELEASE="${ZOLIK_VERSION}+${ZOLIK_COMMIT}"

say()  { printf '\033[1m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[33mwarn:\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[31mError:\033[0m %s\n' "$*" >&2; exit 1; }

ssh_admin() { ssh -o BatchMode=yes "$DEPLOY_SSH" "$@"; }

rsync_common=(
  -az --delete
  --exclude '.git/'
  --exclude 'node_modules/'
  --exclude '.dev-stack/'
  --exclude 'server/.ssh/'
  --exclude 'server/.env'
  --exclude 'client-react-native/dist/'
  --exclude 'client-react-native/.expo/'
  --exclude 'e2e/test-results/'
  --exclude 'e2e/playwright-report/'
)

# ---------------------------------------------------------------- preflight
say "preflight (${RELEASE} → ${PUBLIC_URL})"

command -v rsync >/dev/null || die "rsync is required"
command -v ssh   >/dev/null || die "ssh is required"

ssh_admin 'echo ok' >/dev/null 2>&1 || die "cannot SSH to ${DEPLOY_SSH} (BatchMode=yes)"

if [[ ! -f "${ROOT}/../kdb/go/go.mod" ]]; then
  die "kdb repo not found at ${ROOT}/../kdb — clone it as a sibling of zolik"
fi

ssh_admin "command -v docker >/dev/null" || die "docker is not installed on ${DEPLOY_HOST}"
ssh_admin "command -v nginx >/dev/null"  || die "nginx is not installed on ${DEPLOY_HOST}"

# -------------------------------------------------------------- bootstrap
say "bootstrap ${DEPLOY_USER} on ${DEPLOY_HOST}"

ssh_admin "sudo bash -s" <<EOF
set -euo pipefail
if ! id -u ${DEPLOY_USER} >/dev/null 2>&1; then
  useradd -m -s /bin/bash ${DEPLOY_USER}
  echo "created user ${DEPLOY_USER}"
fi
usermod -aG docker ${DEPLOY_USER} 2>/dev/null || true
mkdir -p ${REMOTE_SRC} ${REMOTE_WEB}/releases ${REMOTE_WEB}
chown -R ${DEPLOY_USER}:${DEPLOY_USER} /home/${DEPLOY_USER}
# nginx (www-data) must traverse into the static web root.
chmod o+x /home/${DEPLOY_USER}
chmod -R a+rX ${REMOTE_WEB}
EOF

# ---------------------------------------------------------------- sync src
say "syncing source to ${REMOTE_ZOLIK} and ${REMOTE_KDB}"

ssh_admin "sudo -u ${DEPLOY_USER} mkdir -p ${REMOTE_SRC}"

rsync "${rsync_common[@]}" \
  "${ROOT}/" "${DEPLOY_SSH}:${REMOTE_ZOLIK}/" \
  --rsync-path="sudo rsync"

rsync "${rsync_common[@]}" \
  "${ROOT}/../kdb/" "${DEPLOY_SSH}:${REMOTE_KDB}/" \
  --rsync-path="sudo rsync"

ssh_admin "sudo chown -R ${DEPLOY_USER}:${DEPLOY_USER} ${REMOTE_SRC} && sudo rm -rf ${REMOTE_ZOLIK}/server/.ssh"

# -------------------------------------------------------------- server env
say "server .env"

ENV_REMOTE="${REMOTE_ZOLIK}/server/.env"
ENV_TEMPLATE="${ROOT}/deploy/env.production.example"

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
  warn "APP_ENV=local — guest sign-in only until you set APP_ENV=production and SMTP_*"
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
# exiting and Docker restarting it forever, which is exactly how the SSH host
# key failure presented before it was fixed.
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

# --------------------------------------------------------------- build web
if [[ "$SKIP_WEB" == false ]]; then
  say "building web client for ${PUBLIC_URL}"

  # The notices are prerendered into the bundle (app.json sets web output to
  # "static"), so who they name is decided here and cannot be changed without
  # a rebuild. Said out loud for the same reason the APP_ENV check above is:
  # the failure is silent otherwise — a perfectly working deploy whose Terms
  # name "[OPERATOR NAME]".
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

  (cd "${ROOT}/client-react-native" && npm ci --silent)
  (cd "${ROOT}/client-react-native" && \
    EXPO_PUBLIC_ZOLIK_BASE_URL="$PUBLIC_URL" \
    EXPO_PUBLIC_ZOLIK_VERSION="$ZOLIK_VERSION" \
    EXPO_PUBLIC_ZOLIK_COMMIT="$ZOLIK_COMMIT" \
    EXPO_PUBLIC_ZOLIK_OPERATOR="$OPERATOR" \
    EXPO_PUBLIC_ZOLIK_OPERATOR_COUNTRY="$OPERATOR_COUNTRY" \
    EXPO_PUBLIC_ZOLIK_OPERATOR_CONTACT="$OPERATOR_CONTACT" \
    npx expo export --platform web)

  say "uploading web release ${RELEASE}"
  ssh_admin "sudo -u ${DEPLOY_USER} mkdir -p ${REMOTE_WEB}/releases/${RELEASE}"
  rsync -az --delete \
    "${ROOT}/client-react-native/dist/" \
    "${DEPLOY_SSH}:${REMOTE_WEB}/releases/${RELEASE}/" \
    --rsync-path="sudo rsync"
  ssh_admin "sudo -u ${DEPLOY_USER} ln -sfn releases/${RELEASE} ${REMOTE_WEB}/current"
  ssh_admin "sudo chmod -R a+rX ${REMOTE_WEB}"
else
  warn "skipping web build (--skip-web)"
fi

# ----------------------------------------------------------- docker server
say "building and starting server (docker-compose.kdb.yml)"

ssh_admin "sudo -u ${DEPLOY_USER} bash -s" <<EOF
set -euo pipefail
export ZOLIK_VERSION='${ZOLIK_VERSION}'
export ZOLIK_COMMIT='${ZOLIK_COMMIT}'
export ZOLIK_BIND='127.0.0.1'
cd ${REMOTE_ZOLIK}/server
docker compose -f docker-compose.kdb.yml up -d --build
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
title="$(printf '%s' "$html" | sed -n 's:.*<title>\([^<]*\)</title>.*:\1:p' | head -1)"

printf '  healthz     %s\n' "$health"
printf '  version     %s\n' "$version"
printf '  page title  %s\n' "${title:-(empty — Expo SPA)}"

if [[ "$health" != "ok" ]]; then
  die "health check failed at ${PUBLIC_URL}/healthz"
fi

if [[ "$html" == *"Coming soon"* ]]; then
  warn "page still shows the placeholder — nginx vhost may not be installed yet"
elif [[ "$html" != *"_expo/"* ]]; then
  warn "root page does not look like the Expo export — check ${REMOTE_WEB}/current"
fi

echo
say "deployed ${RELEASE} to ${PUBLIC_URL}"
printf '  server   ssh %s@%s\n' "$DEPLOY_USER" "$DEPLOY_HOST"
printf '  logs     ssh %s@%s "cd %s/server && docker compose -f docker-compose.kdb.yml logs -f app"\n' \
  "$DEPLOY_USER" "$DEPLOY_HOST" "$REMOTE_ZOLIK"
printf '\n  Next hardening: set APP_ENV=production and SMTP_* in %s/server/.env\n' "$REMOTE_ZOLIK"
