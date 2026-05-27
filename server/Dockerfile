FROM golang:1.24 AS build

WORKDIR /src

# Monorepo: client-tui is required by server/go.mod replace directive.
COPY client-tui/ ./client-tui/
COPY server/go.mod server/go.sum ./server/
WORKDIR /src/server
RUN go mod download

COPY server/ .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /
COPY --from=build /out/server /server
EXPOSE 8090 8092 2222
USER nonroot:nonroot
ENTRYPOINT ["/server"]
