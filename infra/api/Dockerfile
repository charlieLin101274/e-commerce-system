FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /bin/api /api

EXPOSE 8080

ENTRYPOINT ["/api"]
