# Build from backend/, following the other Go services.

FROM golang:1.26-alpine AS dependencies
COPY pkg/authn/go.mod pkg/authn/go.sum /src/pkg/authn/
COPY pkg/notificationcontract/go.mod /src/pkg/notificationcontract/go.mod
WORKDIR /src/services/notification-service
COPY services/notification-service/go.mod services/notification-service/go.sum ./
RUN go mod download
COPY pkg/authn/ /src/pkg/authn/
COPY pkg/notificationcontract/ /src/pkg/notificationcontract/
COPY services/notification-service/ ./

# The checks stage can be run independently before packaging the application.

FROM dependencies AS checks
RUN test -z "$(gofmt -l cmd internal templates ../../pkg/notificationcontract)"
RUN go -C ../../pkg/notificationcontract test ./...
RUN go test ./...
RUN go -C ../../pkg/notificationcontract vet ./...
RUN go vet ./...

FROM dependencies AS build
RUN CGO_ENABLED=0 go build -trimpath -o /out/notification-api ./cmd/api

# Run as a non-root user with only the application binary.

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/notification-api /notification-api
USER nonroot:nonroot
EXPOSE 5003
ENTRYPOINT ["/notification-api"]
