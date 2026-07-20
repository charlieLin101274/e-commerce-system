FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/notification-worker ./cmd/notification-worker

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /bin/api /api
COPY --from=builder /bin/notification-worker /notification-worker

EXPOSE 8080

ENTRYPOINT ["/api"]
