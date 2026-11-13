# Build from the backend/ folder (it needs pkg/authn):
#   docker build -f services/auth-service/Dockerfile -t govai-auth .

# ---- Build: compile a static Linux binary ----
FROM golang:1.26-alpine AS build
WORKDIR /src

# Download dependencies first so Docker can cache them between builds.
COPY pkg/authn/ pkg/authn/
COPY services/auth-service/go.mod services/auth-service/go.sum services/auth-service/
WORKDIR /src/services/auth-service
RUN go mod download

COPY services/auth-service/ .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/auth-api ./cmd/api

# ---- Run: only the binary, no shell, non-root user ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/auth-api /auth-api
USER nonroot:nonroot
EXPOSE 5001
ENTRYPOINT ["/auth-api"]
