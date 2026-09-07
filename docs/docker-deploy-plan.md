# Deploying zolik as an image, not as source

Status: **implemented and deployed**. Written and built 2026-09-06 against
`main` @ 2e01fae; `1.1.1.43+7ba5d8c` has been live on play.limidus.com since
2026-09-07 00:38 UTC.

Built as planned, with two notes. Item F (the pre-deploy snapshot) was folded
in as `deploy.sh --snapshot` rather than left for later — it was a dozen lines
once the volume had a pinned name. And the Dockerfile now takes its target
architecture from `TARGETOS`/`TARGETARCH` instead of hardcoding amd64, which
was needed to stop the build being emulated on an ARM machine and has the side
effect that a local `dev-stack.sh` build is now native arm64 rather than
cross-built.

The cutover has been run — see the bottom of this file for what actually
happened and the one step still outstanding.

## What happens today

`scripts/deploy.sh` treats the production host as a build machine:

1. rsyncs the whole zolik working tree to `/home/zolik/src/zolik`,
2. rsyncs the sibling `../kdb` checkout to `/home/zolik/src/kdb`,
3. runs `docker compose -f docker-compose.kdb.yml up -d --build` **on the server**,
4. separately builds the Expo web bundle locally and rsyncs it to
   `/home/zolik/web/releases/<release>`, symlinked as `current`,
5. installs the nginx vhost, which serves that directory as static files and
   proxies a list of path prefixes to `127.0.0.1:8090`.

So the host carries the full source of two repositories, downloads the Go
module graph, and compiles on every deploy. The unit of deployment is a
directory tree whose contents depend on what the developer's working tree
happened to look like — `--exclude '.claude/'` exists in that rsync list
because 945 MB of unrelated worktrees once went out with a release.

## Target shape

The unit of deployment becomes **one image**, built on the development
machine, carrying both the Go server and the web bundle. The host holds:

    /home/zolik/
      compose.yml     the production compose file (no `build:` key)
      .env            secrets and per-host settings, unchanged in spirit

…and nothing else. No zolik source, no kdb source, no Go toolchain layer,
no npm. `docker compose up -d` on that directory is the whole deploy.

The image is shipped over SSH with `docker save | ssh docker load`. That was
chosen over a registry because there is no registry in the picture today and
the host is on the LAN; the plan keeps the push/pull seam isolated to one
function in the deploy script so swapping in GHCR later is a small change,
not a rewrite.

nginx stays on the host and stays the public entry point. It stops serving
files and becomes a pure reverse proxy — see "The nginx vhost gets simpler".

## Decisions

| Question | Decision |
| --- | --- |
| Where is the image built | Development machine, `docker buildx build` |
| How does it reach the host | `docker save` piped over SSH to `docker load` |
| Where does the web bundle live | Baked into the same image, served by the Go server |
| Reverse proxy | Host nginx, kept |
| Orchestration | One compose file for zolik alone, in its own directory |
| Existing production data | Not migrated — this cutover starts clean, by decision. Every deploy after it preserves the database. |

## Hazards found while reading the current setup

These are the three things that will break quietly if the move is done
naively. Each has a work item below.

### 1. The kdb data volume is not named what the compose file says

Compose derives its project name from the directory holding the compose file.
Today that directory is `server/`, so the volume declared as `kdb_data` exists
on the host as **`server_kdb_data`**. Moving the compose file to
`/home/zolik/` renames the project to `zolik`, and `docker compose up` creates
a *new, empty* `zolik_kdb_data` without any error — with a healthy `/healthz`
and a working sign-up page to say everything is fine.

**For this cutover that is accepted deliberately: the new deployment starts
with an empty database.** Existing accounts, matches and lifetime statistics
are not carried over. The old volume is not deleted by any of this, so the
decision stays reversible for as long as nobody prunes it — see "Cutover".

What must still be fixed is the *cause*, because the requirement from here on
is that data survives every deploy. The production compose file pins the
volume name outright rather than letting Compose derive it:

    volumes:
      kdb_data:
        name: zolik_kdb_data

Pinned that way, the name no longer depends on which directory the file sits
in, so moving or renaming things later can never repeat this. Combined with
`name: zolik` at the top level of the file, the project is stated rather than
inferred.

The second half of the guarantee is a rule for `scripts/deploy.sh`: **nothing
in the deploy path may ever run `docker compose down -v` or
`docker volume prune`.** `down -v` deletes named volumes, which is precisely
the "clean it up between deploys" reflex that would destroy the database on an
otherwise ordinary release. A plain `docker compose up -d` against a new image
tag replaces the container and reuses the volume, which is the behaviour we
want and want by default.

### 2. Three SPA routes collide with API routes — already, today

The client has `app/auth/login.tsx`, `app/auth/register.tsx` and
`app/auth/guest.tsx`. The server registers `POST /auth/login`,
`POST /auth/register` and `POST /auth/guest`. The nginx vhost sends everything
matching `^/(auth|…)` to the API, so a browser hard-refresh or a shared link to
`https://play.limidus.com/auth/login` does not load the app — it reaches the Go
server, which answers 405 because only POST is registered there.

This is a pre-existing bug, not one the containerization introduces, but the
containerization is where it gets fixed: once the Go server serves the bundle
itself, a `MethodNotAllowed` handler that falls through to the SPA for GET
requests asking for HTML makes those URLs work. Doing nothing would carry the
bug across unchanged.

(The other client routes are safe. `/stats` does not collide — the server only
registers `/stats/ai`. `/lobby/games`, `/lobby/table` and `/lobby/join` reach
chi's NotFound because only `/lobby/waiting` is registered. `/match/[matchId]`
uses a different word from the API's `/matches/`.)

### 3. The build stops being native

`server/Dockerfile` already sets `CGO_ENABLED=0 GOOS=linux GOARCH=amd64`, and
that has been free so far because the build ran on the amd64 host. Building on
an Apple Silicon Mac without saying anything runs the golang and node stages
under QEMU emulation — correct, and slow enough to feel broken.

Both build stages must be pinned to `--platform=$BUILDPLATFORM` so they run
natively on the developer's machine and cross-compile; only the final stage is
`linux/amd64`. The Go stage needs no change beyond the pin, since it already
cross-compiles. The node stage emits platform-independent static files.

## Work items

### A. Serve the web bundle from the Go server

`server/internal/app` gains a static handler. It is not a `r.Get("/*")` mount —
that would shadow nothing today but is a trap the moment a new API group is
added. Instead it is wired as chi's `NotFound` **and** `MethodNotAllowed`
handler on the root router in `cmd/server/main.go`, so the API always wins and
the SPA gets everything left over.

Behaviour must match what nginx does today (`try_files $uri $uri/ /index.html`)
because `app.json` sets web output to `static` and expo-router prerenders one
HTML file per route:

- exact file hit → serve it,
- `<path>.html` → serve it,
- `<path>/index.html` → serve it,
- otherwise → `index.html`, so client-side routing takes over
  (`/match/<id>` depends on this).

Two guards on the fallthrough, so a broken API call still fails like an API
call rather than returning a page:

- only for `GET`/`HEAD`,
- only when `Accept` mentions `text/html`; anything else keeps the 404/405.

The bundle is embedded with `go:embed` against a directory the build populates,
so the binary stays a single file and the distroless image needs no extra
copy step. A build with no bundle present (every `go test`, every
`dev-stack.sh` run) must still compile — a build tag or an empty placeholder
directory checked into the repo, with the static handler answering 404 when
the embedded FS is empty.

Tests, in the style the package already uses for routing:

- each of `/auth/login`, `/auth/register`, `/auth/guest` returns HTML to a
  browser-shaped GET and still returns the API's answer to a POST,
- `/match/{anything}` returns `index.html`,
- `/healthz` and `/users/me` are untouched,
- a JSON-shaped GET to an unregistered path still gets 404, not a page.

### B. Build the web bundle inside the image

A node stage ahead of the Go stage:

    FROM --platform=$BUILDPLATFORM node:22 AS web
    …npm ci, then npx expo export --platform web

with the `EXPO_PUBLIC_*` variables that `deploy.sh` sets today (base URL,
version, commit, operator name/country/contact, source URL) passed in as build
args. They are already computed in the script; they move from the environment
of a local `npx` call to `--build-arg`.

This puts the bundle and the binary in the same image, built from the same
commit, which is the point: today a `--skip-web` deploy can leave a bundle
from one release being served by a server from another, and nothing says so.

`.dockerignore` currently excludes `client-react-native/node_modules`,
`client-react-native/dist` and `client-react-native/.expo`. That is correct and
stays — the node stage installs its own.

Cost: `npm ci` becomes part of the image build. It caches on
`package-lock.json`, so it re-runs only when dependencies actually change.

### C. Split the compose files

- `server/docker-compose.kdb.yml` keeps its `build:` section. It is the
  development stack, driven by `scripts/dev-stack.sh`, and building from
  source is the right thing there.
- `deploy/compose/zolik.yml` is new and production-only: `image:` with no
  `build:`, an explicit `name: zolik` at the top level so the project name
  never depends on a directory name again, a pinned
  `volumes.kdb_data.name: zolik_kdb_data` so the database's identity is stated
  rather than inferred from a path (hazard 1), and the security-critical
  environment block stated outright (`SSH_ENABLED=false`,
  `SSH_ALLOW_ALL_KEYS=false`, `ENABLE_TEST_ENDPOINTS=false` — no
  `${…:-false}` indirection on a production host).

The two files now state the same container shape in two places, which is
exactly the kind of drift this repo already writes tests against (see
`TestPublicVhostReachesEveryRoute`). Add a test that reads both files and
fails if the production one lets any of the three switches above be anything
but a literal `false`, and if the volume name is not pinned.

### D. Rewrite `scripts/deploy.sh`

The preflight, `.env` handling, the APP_ENV/SMTP boot check, the operator
warnings and the verification block are all good and stay. What changes is the
middle:

Removed:
- both source rsyncs (zolik and kdb) and the whole `rsync_common` exclude list,
- `/home/zolik/src` and `/home/zolik/web` and the release symlink dance,
- the requirement that the host have the kdb checkout,
- `--skip-web` (the bundle is no longer separable from the server).

Added:
- `docker buildx build --platform linux/amd64` on the dev machine, tagging
  `zolik:<release>` and `zolik:current`,
- `docker save zolik:<release> | gzip | ssh … 'gunzip | docker load'`,
- upload of `deploy/compose/zolik.yml` to `/home/zolik/compose.yml`,
- `docker compose up -d` with `ZOLIK_RELEASE` set, so the tag is explicit and
  a rollback is `ZOLIK_RELEASE=<older> docker compose up -d`,
- pruning to the last 3 release **tags** on the host (`docker image rm`, never
  `docker system prune`, which reaches volumes).

Forbidden, and worth stating in the script itself where the next person
editing it will read it: `docker compose down -v` and `docker volume prune`.
Both destroy the database, both look like ordinary housekeeping, and neither
is needed — `up -d` against a new image tag replaces the container and keeps
the volume. This is the whole of the "data survives a deploy" guarantee, and
it is one line away from being lost at any time.

`.env` moves from `/home/zolik/src/zolik/server/.env` to `/home/zolik/.env`.
The script should move it if the old one exists and the new one does not,
rather than making the operator do it.

### E. The nginx vhost gets simpler

Once the Go server serves the bundle, nginx has no static root and no prefix
list. The whole vhost becomes: the ACME block, the redirect, and two proxy
locations — `/ws/` with the upgrade headers and long read timeout, and `/`
with everything else.

That deletes the class of bug `TestPublicVhostReachesEveryRoute` was written to
catch: a route can no longer be missing from a list, because there is no list.
That test should be deleted along with the regex it checks, and its comment's
warning moved to wherever it stays true. Deleting a test that catches a real
bug is normally wrong; here the bug it catches becomes unrepresentable, and
leaving it would leave a test asserting against a config block that no longer
exists.

The `root /home/zolik/web/current` line goes. `/home/zolik/web` can be removed
from the host once a release has been served from the image.

### F. Optional: a pre-deploy snapshot

Not required for the cutover, and cheap insurance once the database matters
again. A `--snapshot` step in the deploy script that tars `zolik_kdb_data` into
`/home/zolik/backups/` before bringing the new image up, keeping the last few.

Worth doing at the point where losing the database would mean losing something
that cannot be recreated — which, from step 5 of the cutover onward, it does.

## Cutover — done 2026-09-07

Ran from the dev machine (192.168.1.7) against limi-mini (192.168.13.13) as
`davja`, with `sudo -u zolik` for everything touching Docker. The runtime user
has no SSH key of its own, which is why every remote command in this repo is
spelled through the admin account.

The host survey confirmed the plan's assumptions rather than merely agreeing
with them: `x86_64`, so the amd64 target was right; and the database volume
was called **`server_kdb_data`**, exactly as hazard 1 predicted. It was derived
from the compose file's directory, and it was one `docker compose up` in a new
directory away from being silently replaced.

1. **Backed up first.** `server_kdb_data` was 30 MB; a 29 MB tarball now sits
   at `/home/zolik/backups/kdb-preimage-2026-09-07.tgz`. The original volume
   was left in place as well, so the pre-image data exists in two forms.
2. **Deployed.** The image built in ~1 min and shipped as 33.5 MB. The script
   brought the source-built stack down (`down`, no `-v`), created
   `zolik_kdb_data`, and started `zolik:1.1.1.43-7ba5d8c`. All four
   verifications passed, including the two that only exist because of this
   change: `GET /auth/login` → 200 and `GET /users/me` → 401.
3. **Verified with real data, not `/healthz`.** Signed up a guest named
   `DeployProbe43` through the browser; the sign-up wrote WAL entries under
   `identities/` and `matches/`.
4. **Verified the guarantee.** Forced the container replacement a release
   performs (`up -d --force-recreate`). The container id changed
   (98666590dafe → 386964c64756); the volume's `CreatedAt` did not, and the
   data was byte-identical afterwards. Reloading the site still showed
   "Playing as DeployProbe43" — so the database outlives the container, proven
   from both sides.

Two things about what "an unchanged redeploy" tests: running `deploy.sh` a
second time with nothing changed does *not* replace the container (Compose
correctly leaves it alone), so it proves nothing about data survival on its
own. Step 4 forced the replacement deliberately. And the second run did
exercise the transfer-skip path — "already on 192.168.13.13 — skipping
transfer".

### Still outstanding

- **`DeployProbe43` is a real guest account on production.** Harmless, and
  worth deleting when there is a way to.
- **The legal notices are still a DRAFT *in production*.** `ZOLIK_OPERATOR_COUNTRY`
  (USA) and `ZOLIK_OPERATOR_CONTACT` (support@limidus.com) now have defaults in
  `scripts/deploy.sh`, which clears the banner — but the notices are prerendered
  into the bundle at build time, so the live site keeps its draft banner until
  the next deploy rebuilds the image. Nothing else is needed; the next
  `./scripts/deploy.sh` does it.
- **After a week, delete the old deployment.** `/home/zolik/src` (both source
  trees), `/home/zolik/web` (the static bundle nginx no longer reads), and the
  `server_kdb_data` volume. Until then they are the rollback:

      ssh davja@192.168.13.13 'sudo -u zolik bash -c "cd /home/zolik/src/zolik/server && docker compose -f docker-compose.kdb.yml up -d --build"'

  That path still has its own volume with the pre-cutover accounts in it.

## What does not change

- The KDB single-container shape, its 1 GB memory cap, and the `/data` volume
  layout inside the container.
- `deploy/env.production.example` and everything it documents about `APP_ENV`.
- `scripts/version.sh` as the one definition of a release string.
- The distroless nonroot final image.
- `scripts/dev-stack.sh` and how local development works.
