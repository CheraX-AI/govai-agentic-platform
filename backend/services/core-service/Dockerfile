# Build from the backend/ folder (it needs pkg/authn):
#   docker build -f services/core-service/Dockerfile -t govai-core .

# ---- Build: compile a static Linux binary ----
FROM golang:1.26-alpine AS build
WORKDIR /src

# Download dependencies first so Docker can cache them between builds.
COPY pkg/authn/ pkg/authn/
COPY services/core-service/go.mod services/core-service/go.sum services/core-service/
WORKDIR /src/services/core-service
RUN go mod download

COPY services/core-service/ .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/core-api ./cmd/api

# ---- Run: only the binary, no shell, non-root user ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/core-api /core-api
USER nonroot:nonroot
EXPOSE 5002
ENTRYPOINT ["/core-api"]
