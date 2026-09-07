# The whole deployable: the Expo web client and the Go server, from one commit.
#
# Both halves used to ship separately — the bundle rsynced to the host for
# nginx to serve, the server built from source on the host — with nothing
# tying them to the same revision. Building them together here is what makes
# "which version is deployed" a question with one answer.
#
#   docker buildx build --platform linux/amd64 --build-context kdbsrc=../kdb \
#     -f server/Dockerfile -t zolik:$(scripts/version.sh | tr ' ' '-') ..
#
# scripts/deploy.sh does that and ships the result; docker-compose.kdb.yml
# does it for local development.

# ---------------------------------------------------------------- web client
#
# --platform=$BUILDPLATFORM pins this to the architecture of the machine
# running the build, not the one being built for. Without it, building an
# amd64 image on an Apple Silicon Mac runs npm and Metro under QEMU: correct,
# and slow enough that the build reads as hung. The output is static files,
# which have no architecture, so there is nothing to lose by building them
# natively.
FROM --platform=$BUILDPLATFORM node:22-bookworm AS web

WORKDIR /web

# Dependencies first, so a source-only change does not reinstall them. This
# layer rebuilds only when package-lock.json does.
COPY client-react-native/package.json client-react-native/package-lock.json ./
RUN npm ci --no-audit --no-fund

COPY client-react-native/ ./

# What the bundle says about itself. These were `EXPO_PUBLIC_*` environment
# variables on a developer's laptop before the build moved in here; they are
# build args now because the export is prerendered — app.json sets web output
# to "static", so every one of these is baked into HTML at this point and
# cannot be changed by anything the container is started with later.
#
# The operator values decide whether the Terms and the Privacy Notice name a
# real company or carry a "not yet in force" draft banner. scripts/deploy.sh
# warns when they are unset; the defaults here are deliberately blank rather
# than plausible, so an unconfigured build looks unconfigured.
ARG ZOLIK_PUBLIC_URL=http://localhost:8090
ARG ZOLIK_VERSION=0.0.0-dev
ARG ZOLIK_COMMIT=unknown
ARG ZOLIK_OPERATOR=
ARG ZOLIK_OPERATOR_COUNTRY=
ARG ZOLIK_OPERATOR_CONTACT=
ARG ZOLIK_SOURCE_URL=https://github.com/dbeasty/zolik

RUN EXPO_PUBLIC_ZOLIK_BASE_URL="${ZOLIK_PUBLIC_URL}" \
    EXPO_PUBLIC_ZOLIK_VERSION="${ZOLIK_VERSION}" \
    EXPO_PUBLIC_ZOLIK_COMMIT="${ZOLIK_COMMIT}" \
    EXPO_PUBLIC_ZOLIK_OPERATOR="${ZOLIK_OPERATOR}" \
    EXPO_PUBLIC_ZOLIK_OPERATOR_COUNTRY="${ZOLIK_OPERATOR_COUNTRY}" \
    EXPO_PUBLIC_ZOLIK_OPERATOR_CONTACT="${ZOLIK_OPERATOR_CONTACT}" \
    EXPO_PUBLIC_ZOLIK_SOURCE_URL="${ZOLIK_SOURCE_URL}" \
    npx expo export --platform web

# Fail here rather than three layers down. An export that produced no
# index.html still exits 0 often enough to be worth checking, and the symptom
# otherwise is a running server that 404s its own home page.
RUN test -f dist/index.html || (echo "expo export produced no dist/index.html" >&2; exit 1)

# ------------------------------------------------------------------- server
#
# $BUILDPLATFORM for the same reason as above; the Go toolchain then
# cross-compiles to $TARGETPLATFORM, which it does natively and for free
# because CGO is off.
FROM --platform=$BUILDPLATFORM golang:1.26 AS build

WORKDIR /src

# Monorepo: client-tui is required by server/go.mod replace directive.
COPY client-tui/ ./client-tui/
# KDB is the embedded storage engine behind FEATURE_FLAG_DB_ENGINE=kdb. Its
# source lives in a sibling repository, handed in as a BuildKit named
# context; the copy target matches server/go.mod's `replace ../../kdb/go`
# as resolved from /src/server. Compose wires the context automatically
# (additional_contexts); a manual build needs:
#   docker build --build-context kdbsrc=../kdb -f server/Dockerfile ..
COPY --from=kdbsrc go/ /kdb/go/
COPY server/go.mod server/go.sum ./server/
WORKDIR /src/server
RUN go mod download

COPY server/ .

# The client, compiled into the binary — see internal/webui. It lands after
# `go mod download` so a client-only change does not re-resolve the module
# graph, and before `go build` because //go:embed reads the directory at
# compile time.
#
# The placeholder goes: it exists only so `go:embed` has something to match on
# a checkout where Expo has never run, and an image has no use for a file
# explaining that.
COPY --from=web /web/dist/ ./internal/webui/dist/
RUN rm -f ./internal/webui/dist/.gitkeep

# Declared here, not near the top, so a version bump does not invalidate the
# `go mod download` layer above. .git is not in this image, so neither value
# can be derived inside the build — scripts/version.sh computes them outside
# and passes them in as build args. The defaults are deliberately implausible:
# a plain `docker build` should produce an honest "I don't know" rather than a
# believable lie.
ARG ZOLIK_VERSION=0.0.0-dev
ARG ZOLIK_COMMIT=unknown

# TARGETOS/TARGETARCH are filled in by BuildKit from --platform. Hardcoding
# amd64 was free while the build ran on the amd64 host; now that it runs on
# whatever machine deploys, the target has to be stated by the caller rather
# than assumed here — otherwise a local `docker compose up --build` on an ARM
# Mac produces an amd64 binary inside an ARM image, which exits with nothing
# more helpful than "exec format error".
ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
      -ldflags "-X zolik/server/internal/buildinfo.Version=${ZOLIK_VERSION} \
                -X zolik/server/internal/buildinfo.Commit=${ZOLIK_COMMIT}" \
      -o /out/server ./cmd/server

# An empty, nonroot-owned directory for the KDB data volume to inherit its
# ownership from — distroless has no shell to chown with, and a named volume
# copies the image directory's owner on first use.
RUN mkdir -p /out/data && chown 65532:65532 /out/data

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /
COPY --from=build /out/server /server
COPY --from=build --chown=nonroot:nonroot /out/data /data
EXPOSE 8090 8092 2222
USER nonroot:nonroot
ENTRYPOINT ["/server"]
