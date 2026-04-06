# Build stage
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o proxy ./cmd/proxy

# Runtime stage
FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/proxy .
COPY config.json.example .
EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["./proxy"]
