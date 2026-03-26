# Stage 1: Build
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Install gcc for CGO (required by SQLite via mattn/go-sqlite3).
RUN apk add --no-cache gcc musl-dev

# Download dependencies before copying source so layers are cached.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source and build.
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags "-X main.version=${VERSION}" -o /app/server ./cmd/server

# Stage 2: Runtime
FROM alpine:3.20

WORKDIR /app

COPY --from=builder /app/server /app/server

# Persistent data directory for the SQLite database.
RUN mkdir -p /app/data

EXPOSE 8000

CMD ["/app/server"]
