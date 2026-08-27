FROM golang:1.24 AS build

WORKDIR /src

# Monorepo: client-tui is required by server/go.mod replace directive.
COPY client-tui/ ./client-tui/
COPY server/go.mod server/go.sum ./server/
WORKDIR /src/server
RUN go mod download

COPY server/ .

# Declared here, not near the top, so a version bump does not invalidate the
# `go mod download` layer above. .git is not in this image, so neither value
# can be derived inside the build — scripts/version.sh computes them outside
# and passes them in as build args. The defaults are deliberately implausible:
# a plain `docker build` should produce an honest "I don't know" rather than a
# believable lie.
ARG ZOLIK_VERSION=0.0.0-dev
ARG ZOLIK_COMMIT=unknown

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
      -ldflags "-X zolik/server/internal/buildinfo.Version=${ZOLIK_VERSION} \
                -X zolik/server/internal/buildinfo.Commit=${ZOLIK_COMMIT}" \
      -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /
COPY --from=build /out/server /server
EXPOSE 8090 8092 2222
USER nonroot:nonroot
ENTRYPOINT ["/server"]
