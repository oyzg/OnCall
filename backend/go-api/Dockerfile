FROM golang:1.24.1-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/go-api ./cmd/server

FROM alpine:3.21

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata wget

COPY --from=builder /out/go-api /app/go-api

ENV APP_NAME=go-api \
    APP_ENV=demo \
    LOG_LEVEL=INFO \
    HTTP_HOST=0.0.0.0 \
    HTTP_PORT=8080

EXPOSE 8080

ENTRYPOINT ["/app/go-api"]
