# Stage 1: Build
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Download dependencies before copying source so layers are cached.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source and build.
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o /app/server ./cmd/server

# Stage 2: Runtime
FROM alpine:3.20

WORKDIR /app

COPY --from=builder /app/server /app/server

# Persistent data directory for the SQLite database.
RUN mkdir -p /app/data

EXPOSE 8000

CMD ["/app/server"]
