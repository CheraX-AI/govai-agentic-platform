# Build from backend/ so shared Go modules are available.

FROM golang:1.26-alpine AS dependencies
WORKDIR /src

# Download dependencies before copying service source for stable build caching.

COPY pkg/authn/ pkg/authn/
COPY pkg/chatcontract/ pkg/chatcontract/
COPY pkg/notificationcontract/ pkg/notificationcontract/
COPY services/auth-service/go.mod services/auth-service/go.sum services/auth-service/
WORKDIR /src/services/auth-service
RUN go mod download
COPY services/auth-service/ .

# Database integration tests remain opt-in and use disposable fixtures only.

FROM dependencies AS checks
RUN go test ./...
RUN go vet ./...

FROM dependencies AS build
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/auth-api ./cmd/api

# Package only the binary and run without root privileges.

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/auth-api /auth-api
USER nonroot:nonroot
EXPOSE 5001
ENTRYPOINT ["/auth-api"]
